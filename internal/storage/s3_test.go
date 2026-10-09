package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/drona-gyawali/logStore/internal/log"
	"github.com/drona-gyawali/logStore/internal/manifest"
)

type storedObject struct {
	data []byte
	etag string
}

type fakeS3 struct {
	mu sync.Mutex

	objects map[string]storedObject
	version int

	indexReads int
	bothRead   chan struct{}
	release    chan struct{}
}

func newFakeS3() *fakeS3 {
	initialManifest, err := json.Marshal(manifest.IndexManifest{})
	if err != nil {
		panic(err)
	}

	return &fakeS3{
		objects: map[string]storedObject{
			"index.json": {
				data: initialManifest,
				etag: `"etag-0"`,
			},
		},
		bothRead: make(chan struct{}),
		release:  make(chan struct{}),
	}
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/bucket/")
	if key == r.URL.Path {
		http.Error(w, "unexpected bucket path", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		f.handleGet(w, r, key)
	case http.MethodPut:
		f.handlePut(w, r, key)
	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
	}
}

func (f *fakeS3) handleGet(
	w http.ResponseWriter,
	r *http.Request,
	key string,
) {
	f.mu.Lock()
	obj, exists := f.objects[key]

	// Capture the object before releasing either writer.
	// The first two manifest reads must both see sequence zero.
	if key == "index.json" {
		f.indexReads++
		if f.indexReads == 2 {
			close(f.bothRead)
		}
	}
	f.mu.Unlock()

	if key == "index.json" {
		select {
		case <-f.bothRead:
		case <-r.Context().Done():
			return
		}
	}

	if !exists {
		writeS3Error(w, http.StatusNotFound, "NoSuchKey")
		return
	}

	w.Header().Set("ETag", obj.etag)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(obj.data)
}

func (f *fakeS3) handlePut(
	w http.ResponseWriter,
	r *http.Request,
	key string,
) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read request body", http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	current, exists := f.objects[key]

	if match := r.Header.Get("If-Match"); match != "" {
		if !exists || current.etag != match {
			writeS3Error(w, http.StatusPreconditionFailed, "PreconditionFailed")
			return
		}
	}

	if r.Header.Get("If-None-Match") == "*" && exists {
		writeS3Error(w, http.StatusPreconditionFailed, "PreconditionFailed")
		return
	}

	f.version++
	etag := fmt.Sprintf(`"etag-%d"`, f.version)
	f.objects[key] = storedObject{
		data: append([]byte(nil), data...),
		etag: etag,
	}

	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
}

func writeS3Error(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(
		w,
		"<Error><Code>%s</Code><Message>simulated error</Message></Error>",
		code,
	)
}

func TestS3StorageAppendConcurrentWriters(t *testing.T) {
	fake := newFakeS3()
	server := httptest.NewServer(fake)
	defer server.Close()

	client := s3.NewFromConfig(
		aws.Config{
			Region: "us-east-1",
			Credentials: credentials.NewStaticCredentialsProvider(
				"test-access-key",
				"test-secret-key",
				"",
			),
		},
		func(options *s3.Options) {
			options.BaseEndpoint = aws.String(server.URL)
			options.UsePathStyle = true
		},
	)

	store := NewS3Storage(client, "bucket")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	operations := [][]log.LogEntry{
		{{Operation: "SET", Key: "user:1", Value: "Alice"}},
		{{Operation: "SET", Key: "user:2", Value: "Bob"}},
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(operations))

	for _, ops := range operations {
		wg.Add(1)
		go func(ops []log.LogEntry) {
			defer wg.Done()
			errs <- store.Append(ctx, ops)
		}(ops)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("concurrent appends did not finish before the deadline")
	}
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Append returned an error: %v", err)
		}
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()

	var finalManifest manifest.IndexManifest
	if err := json.Unmarshal(fake.objects["index.json"].data, &finalManifest); err != nil {
		t.Fatalf("decode final manifest: %v", err)
	}

	if finalManifest.CurrentSeq != 2 {
		t.Fatalf(
			"final sequence = %d, want 2",
			finalManifest.CurrentSeq,
		)
	}

	// Follow only the committed chain from the manifest.
	// A failed conditional write may leave an orphan log behind.
	currentKey := finalManifest.CurrentLogKey
	seen := make(map[string]bool)
	committedOperations := make(map[string]string)

	for currentKey != "" {
		if seen[currentKey] {
			t.Fatalf("cycle detected in log chain at %q", currentKey)
		}
		seen[currentKey] = true

		obj, exists := fake.objects[currentKey]
		if !exists {
			t.Fatalf("committed log %q is missing", currentKey)
		}

		var batch log.LogBatch
		if err := json.Unmarshal(obj.data, &batch); err != nil {
			t.Fatalf("decode log %q: %v", currentKey, err)
		}

		for _, operation := range batch.Operations {
			committedOperations[operation.Key] = operation.Value
		}

		currentKey = batch.PrevLogKey
	}

	if len(seen) != 2 {
		t.Fatalf("committed log count = %d, want 2", len(seen))
	}

	if committedOperations["user:1"] != "Alice" {
		t.Errorf("user:1 = %q, want Alice", committedOperations["user:1"])
	}
	if committedOperations["user:2"] != "Bob" {
		t.Errorf("user:2 = %q, want Bob", committedOperations["user:2"])
	}
}

func TestS3StorageAppendCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := &S3Storage{}

	err := store.Append(ctx, []log.LogEntry{
		{
			Operation: "SET",
			Key:       "user:1",
			Value:     "Alice",
		},
	})

	if err == nil {
		t.Fatal("expected an error for cancelled context")
	}

	if !strings.Contains(err.Error(), "append cancelled") {
		t.Fatalf("expected cancellation error, got: %v", err)
	}
}
