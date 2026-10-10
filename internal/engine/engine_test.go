package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/drona-gyawali/logStore/internal/log"
	"github.com/drona-gyawali/logStore/internal/manifest"
)

type fakeStorage struct {
	mu        sync.Mutex
	objects   map[string][]byte
	seq       uint64
	lastKey   string
	appendErr error
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{
		objects: make(map[string][]byte),
	}
}

func (f *fakeStorage) Get(
	ctx context.Context,
	key string,
) ([]byte, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, "", err
	}

	data, ok := f.objects[key]
	if !ok {
		return nil, "", fmt.Errorf("get %s: %w", key, errors.New("missing object"))
	}

	return append([]byte(nil), data...), fmt.Sprintf("%s-etag", key), nil
}

func (f *fakeStorage) Put(
	ctx context.Context,
	key string,
	data []byte,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.objects[key] = append([]byte(nil), data...)
	return key + "-etag", nil
}

func (f *fakeStorage) PutIfMatch(
	ctx context.Context,
	key string,
	data []byte,
	etag string,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.objects[key]; !ok {
		return "", errors.New("object does not exist")
	}

	f.objects[key] = append([]byte(nil), data...)
	return key + "-etag", nil
}

func (f *fakeStorage) PutIfNoneMatch(
	ctx context.Context,
	key string,
	data []byte,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.objects[key]; ok {
		return "", errors.New("object already exists")
	}

	f.objects[key] = append([]byte(nil), data...)
	return key + "-etag", nil
}

func (f *fakeStorage) Delete(
	ctx context.Context,
	key string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.objects, key)
	return nil
}

func (f *fakeStorage) List(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (f *fakeStorage) Append(
	ctx context.Context,
	operations []log.LogEntry,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.appendErr != nil {
		return f.appendErr
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	data, ok := f.objects["index.json"]
	if !ok {
		return errors.New("manifest does not exist")
	}

	var current manifest.IndexManifest
	if err := json.Unmarshal(data, &current); err != nil {
		return err
	}

	nextSeq := current.CurrentSeq + 1
	logKey := fmt.Sprintf("logs/%d.json", nextSeq)

	batch := log.LogBatch{
		SequenceID: nextSeq,
		PrevLogKey: current.CurrentLogKey,
		Operations: append([]log.LogEntry(nil), operations...),
	}

	logData, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	f.objects[logKey] = logData

	next := manifest.IndexManifest{
		CurrentSeq:         nextSeq,
		CurrentLogKey:      logKey,
		CurrentSnapshotSeq: current.CurrentSnapshotSeq,
	}

	manifestData, err := json.Marshal(next)
	if err != nil {
		return err
	}

	f.objects["index.json"] = manifestData
	f.seq = nextSeq
	f.lastKey = logKey

	return nil
}

func TestEngineInitialize(t *testing.T) {
	store := newFakeStorage()

	if _, err := store.Put(context.Background(), "index.json", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	engine := New(store)

	if err := engine.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize returned error: %v", err)
	}
}

func TestEngineSetGetDelete(t *testing.T) {
	ctx := context.Background()
	store := newFakeStorage()

	initialManifest, err := json.Marshal(manifest.IndexManifest{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Put(ctx, "index.json", initialManifest); err != nil {
		t.Fatal(err)
	}

	engine := New(store)

	if err := engine.Initialize(ctx); err != nil {
		t.Fatalf("Initialize returned error: %v", err)
	}

	if err := engine.Set(ctx, "user:1", "Alice"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	value, exists, err := engine.Get(ctx, "user:1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !exists || value != "Alice" {
		t.Fatalf("expected Alice, got %q, exists=%v", value, exists)
	}

	if err := engine.Delete(ctx, "user:1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, exists, err = engine.Get(ctx, "user:1")
	if err != nil {
		t.Fatalf("Get after Delete returned error: %v", err)
	}
	if exists {
		t.Fatal("expected user:1 to be deleted")
	}
}
func TestEngineSetDoesNotUpdateRAMWhenAppendFails(t *testing.T) {
	ctx := context.Background()
	store := newFakeStorage()

	initialManifest, err := json.Marshal(manifest.IndexManifest{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Put(ctx, "index.json", initialManifest); err != nil {
		t.Fatal(err)
	}

	engine := New(store)
	if err := engine.Initialize(ctx); err != nil {
		t.Fatalf("Initialize returned error: %v", err)
	}

	appendFailure := errors.New("simulated durable storage failure")

	store.mu.Lock()
	store.appendErr = appendFailure
	store.mu.Unlock()

	err = engine.Set(ctx, "user:failure", "should-not-appear")
	if !errors.Is(err, appendFailure) {
		t.Fatalf("Set error = %v, want %v", err, appendFailure)
	}

	value, exists, err := engine.Get(ctx, "user:failure")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if exists {
		t.Fatalf("failed Set became visible in RAM: value = %q", value)
	}
}

func TestEngineDeleteDoesNotUpdateRAMWhenAppendFails(t *testing.T) {
	ctx := context.Background()
	store := newFakeStorage()

	initialManifest, err := json.Marshal(manifest.IndexManifest{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Put(ctx, "index.json", initialManifest); err != nil {
		t.Fatal(err)
	}

	engine := New(store)
	if err := engine.Initialize(ctx); err != nil {
		t.Fatalf("Initialize returned error: %v", err)
	}

	if err := engine.Set(ctx, "user:1", "Alice"); err != nil {
		t.Fatalf("initial Set returned error: %v", err)
	}

	appendFailure := errors.New("simulated durable storage failure")

	store.mu.Lock()
	store.appendErr = appendFailure
	store.mu.Unlock()

	err = engine.Delete(ctx, "user:1")
	if !errors.Is(err, appendFailure) {
		t.Fatalf("Delete error = %v, want %v", err, appendFailure)
	}

	value, exists, err := engine.Get(ctx, "user:1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !exists || value != "Alice" {
		t.Fatalf("failed Delete changed RAM state: value = %q, exists = %v; want Alice, true", value, exists)
	}
}
