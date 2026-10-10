package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"

	"github.com/drona-gyawali/logStore/internal/engine"
	"github.com/drona-gyawali/logStore/internal/log"
	"github.com/drona-gyawali/logStore/internal/manifest"
	"github.com/drona-gyawali/logStore/internal/state"
	"github.com/drona-gyawali/logStore/internal/storage"
)

type testEnvironment struct {
	client *s3.Client
	store  *storage.S3Storage
	bucket string
}

func newTestEnvironment(t *testing.T) *testEnvironment {
	t.Helper()

	endpoint := os.Getenv("S3_ENDPOINT")
	region := os.Getenv("S3_REGION")
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

	if endpoint == "" || region == "" || accessKey == "" || secretKey == "" {
		t.Skip("set S3_ENDPOINT, S3_REGION, AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)
	if err != nil {
		t.Fatalf("load AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	bucket := "annales-it-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("create isolated test bucket: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(), 30*time.Second,
		)
		defer cleanupCancel()

		paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
		})
		for paginator.HasMorePages() {
			page, listErr := paginator.NextPage(cleanupCtx)
			if listErr != nil {
				t.Errorf("list objects during cleanup: %v", listErr)
				return
			}

			objects := make([]types.ObjectIdentifier, 0, len(page.Contents))
			for _, object := range page.Contents {
				objects = append(objects, types.ObjectIdentifier{Key: object.Key})
			}
			if len(objects) == 0 {
				continue
			}

			_, deleteErr := client.DeleteObjects(cleanupCtx, &s3.DeleteObjectsInput{
				Bucket: aws.String(bucket),
				Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)},
			})
			if deleteErr != nil {
				t.Errorf("delete test objects: %v", deleteErr)
				return
			}
		}

		_, err := client.DeleteBucket(cleanupCtx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})
		if err != nil {
			t.Errorf("delete isolated test bucket: %v", err)
		}
	})

	return &testEnvironment{
		client: client,
		store:  storage.NewS3Storage(client, bucket),
		bucket: bucket,
	}
}

func TestMinIOConditionalWrites(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	etag, err := env.store.PutIfNoneMatch(ctx, "conditional.json", []byte(`{"version":1}`))
	if err != nil {
		t.Fatalf("initial conditional write: %v", err)
	}
	if etag == "" {
		t.Fatal("initial write returned an empty ETag")
	}

	if _, err := env.store.PutIfNoneMatch(
		ctx, "conditional.json", []byte(`{"version":2}`),
	); err == nil {
		t.Fatal("PutIfNoneMatch unexpectedly overwrote an existing object")
	}

	newETag, err := env.store.PutIfMatch(
		ctx, "conditional.json", []byte(`{"version":2}`), etag,
	)
	if err != nil {
		t.Fatalf("PutIfMatch with current ETag: %v", err)
	}
	if newETag == "" {
		t.Fatal("conditional update returned an empty ETag")
	}

	if _, err := env.store.PutIfMatch(
		ctx, "conditional.json", []byte(`{"version":3}`), etag,
	); err == nil {
		t.Fatal("PutIfMatch unexpectedly accepted a stale ETag")
	}

	data, _, err := env.store.Get(ctx, "conditional.json")
	if err != nil {
		t.Fatalf("read updated object: %v", err)
	}
	if string(data) != `{"version":2}` {
		t.Fatalf("unexpected stored data: %s", data)
	}
}

func TestMinIOConcurrentAppendsPreserveCommittedOperations(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	e := engine.New(env.store)
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("initialize engine: %v", err)
	}

	const writers = 8
	var wg sync.WaitGroup
	errs := make(chan error, writers)

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			entry := log.LogEntry{
				Operation: "SET",
				Key:       fmt.Sprintf("writer-%02d", i),
				Value:     fmt.Sprintf("value-%02d", i),
			}
			errs <- env.store.Append(ctx, []log.LogEntry{entry})
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent append failed: %v", err)
		}
	}
	if t.Failed() {
		return
	}

	data, _, err := env.store.Get(ctx, "index.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var index manifest.IndexManifest
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if index.CurrentSeq != writers {
		t.Fatalf("manifest sequence = %d; want %d", index.CurrentSeq, writers)
	}

	seen := make(map[string]bool, writers)
	key := index.CurrentLogKey
	for expected := uint64(writers); expected > 0; expected-- {
		if key == "" {
			t.Fatalf("log chain ended before sequence %d", expected)
		}

		data, _, err := env.store.Get(ctx, key)
		if err != nil {
			t.Fatalf("read log %q: %v", key, err)
		}
		var batch log.LogBatch
		if err := json.Unmarshal(data, &batch); err != nil {
			t.Fatalf("decode log %q: %v", key, err)
		}
		if batch.SequenceID != expected {
			t.Fatalf("log sequence = %d; want %d", batch.SequenceID, expected)
		}
		for _, operation := range batch.Operations {
			seen[operation.Key] = true
		}
		key = batch.PrevLogKey
	}

	if key != "" {
		t.Fatalf("log chain has an unexpected predecessor: %q", key)
	}
	if len(seen) != writers {
		t.Fatalf("committed distinct keys = %d; want %d", len(seen), writers)
	}
}

func TestMinIOEngineRestartAndSnapshotRecovery(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	first := engine.New(env.store)
	if err := first.Initialize(ctx); err != nil {
		t.Fatalf("initialize first engine: %v", err)
	}

	if err := first.Set(ctx, "persist-me", "before-snapshot"); err != nil {
		t.Fatalf("set persistent value: %v", err)
	}
	if err := first.Set(ctx, "delete-me", "temporary"); err != nil {
		t.Fatalf("set deletable value: %v", err)
	}
	if err := first.Delete(ctx, "delete-me"); err != nil {
		t.Fatalf("delete value: %v", err)
	}
	if err := first.Set(ctx, "persist-me", "after-snapshot"); err != nil {
		t.Fatalf("update persistent value: %v", err)
	}

	// A new engine has an empty RAM state and must rebuild from MinIO.
	second := engine.New(env.store)
	if err := second.Initialize(ctx); err != nil {
		t.Fatalf("recover engine from logs: %v", err)
	}
	assertValue(t, second, "persist-me", "after-snapshot")
	assertMissing(t, second, "delete-me")

	compactor := state.NewCompactor(env.store)
	if err := compactor.Compact(ctx); err != nil {
		t.Fatalf("compact state: %v", err)
	}

	// Rebuild again, now using the snapshot plus any logs after it.
	third := engine.New(env.store)
	if err := third.Initialize(ctx); err != nil {
		t.Fatalf("recover engine from snapshot: %v", err)
	}
	assertValue(t, third, "persist-me", "after-snapshot")
	assertMissing(t, third, "delete-me")
}

func assertValue(t *testing.T, e *engine.Engine, key, want string) {
	t.Helper()
	got, exists, err := e.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("get %q: %v", key, err)
	}
	if !exists || got != want {
		t.Fatalf("get %q = (%q, %v); want (%q, true)", key, got, exists, want)
	}
}

func assertMissing(t *testing.T, e *engine.Engine, key string) {
	t.Helper()
	_, exists, err := e.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("get %q: %v", key, err)
	}
	if exists {
		t.Fatalf("key %q should have been deleted", key)
	}
}

func TestMinIORestartIgnoresUncommittedOrphanLog(t *testing.T) {
	ctx := context.Background()
	env := newTestEnvironment(t)

	// Start a fresh engine and commit one legitimate write.
	first := engine.New(env.store)
	if err := first.Initialize(ctx); err != nil {
		t.Fatalf("initialize first engine: %v", err)
	}

	if err := first.Set(ctx, "committed:key", "survives"); err != nil {
		t.Fatalf("commit legitimate write: %v", err)
	}

	// Read the manifest to identify the committed log-chain head.
	manifestData, _, err := env.store.Get(ctx, "index.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var current manifest.IndexManifest
	if err := json.Unmarshal(manifestData, &current); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	// Simulate a writer that uploaded a log but crashed before
	// advancing index.json. This object is deliberately orphaned.
	orphan := log.LogBatch{
		SequenceID: current.CurrentSeq + 1,
		PrevLogKey: current.CurrentLogKey,
		Operations: []log.LogEntry{
			{
				Operation: "SET",
				Key:       "orphan:key",
				Value:     "must-not-appear",
			},
		},
	}

	orphanData, err := json.Marshal(orphan)
	if err != nil {
		t.Fatalf("encode orphan log: %v", err)
	}

	if _, err := env.store.Put(
		ctx,
		"logs/orphan-uncommitted.json",
		orphanData,
	); err != nil {
		t.Fatalf("upload orphan log: %v", err)
	}

	// Simulate a process restart by creating a new engine instance.
	restarted := engine.New(env.store)
	if err := restarted.Initialize(ctx); err != nil {
		t.Fatalf("initialize restarted engine: %v", err)
	}

	// The committed write must survive the restart.
	assertValue(t, restarted, "committed:key", "survives")

	// The orphan must not become visible because index.json never
	// committed it as part of the log chain.
	assertMissing(t, restarted, "orphan:key")
}

func TestMinIOFailedManifestCommitLeavesUncommittedLog(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	e := engine.New(env.store)
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("initialize engine: %v", err)
	}

	if err := e.Set(ctx, "before-crash", "value"); err != nil {
		t.Fatalf("set before crash: %v", err)
	}

	faultErr := errors.New("simulated crash before manifest commit")
	fired := false
	env.store.TestHookBeforeManifestCommit = func() error {
		if !fired {
			fired = true
			return faultErr
		}
		return nil
	}

	err := e.Set(ctx, "crash-key", "must-not-appear")
	if !errors.Is(err, faultErr) {
		t.Fatalf("expected fault error, got: %v", err)
	}

	env.store.TestHookBeforeManifestCommit = nil

	restarted := engine.New(env.store)
	if err := restarted.Initialize(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}

	assertValue(t, restarted, "before-crash", "value")
	assertMissing(t, restarted, "crash-key")
}

func TestMinIODeleteIsRecoveredAfterRestart(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	first := engine.New(env.store)
	if err := first.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := first.Set(ctx, "to-delete", "exists"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := first.Delete(ctx, "to-delete"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	second := engine.New(env.store)
	if err := second.Initialize(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertMissing(t, second, "to-delete")
}

func TestMinIOGetMissingKeyReturnsFalse(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	e := engine.New(env.store)
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	_, exists, err := e.Get(ctx, "no-such-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected exists=false for missing key")
	}
}

func TestMinIOEmptyKeyIsRejected(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	e := engine.New(env.store)
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	if err := e.Set(ctx, "", "value"); err == nil {
		t.Fatal("Set with empty key should return an error")
	}
	if err := e.Delete(ctx, ""); err == nil {
		t.Fatal("Delete with empty key should return an error")
	}
}

func TestMinIOCompactionAdvancesSnapshotSequence(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	e := engine.New(env.store)
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := e.Set(ctx, "k1", "v1"); err != nil {
		t.Fatalf("set k1: %v", err)
	}
	if err := e.Set(ctx, "k2", "v2"); err != nil {
		t.Fatalf("set k2: %v", err)
	}

	compactor := state.NewCompactor(env.store)
	if err := compactor.Compact(ctx); err != nil {
		t.Fatalf("compact: %v", err)
	}

	data, _, err := env.store.Get(ctx, "index.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var idx manifest.IndexManifest
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if idx.CurrentSnapshotSeq == 0 {
		t.Fatal("CurrentSnapshotSeq should be non-zero after compaction")
	}

	recovered := engine.New(env.store)
	if err := recovered.Initialize(ctx); err != nil {
		t.Fatalf("recover after compaction: %v", err)
	}
	assertValue(t, recovered, "k1", "v1")
	assertValue(t, recovered, "k2", "v2")
}
func TestMinIOOldLogsDeletedAfterCompaction(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	e := engine.New(env.store)
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := e.Set(ctx, "k1", "v1"); err != nil {
		t.Fatalf("set k1: %v", err)
	}
	if err := e.Set(ctx, "k2", "v2"); err != nil {
		t.Fatalf("set k2: %v", err)
	}

	compactor := state.NewCompactor(env.store)
	if err := compactor.Compact(ctx); err != nil {
		t.Fatalf("compact: %v", err)
	}
	if err := compactor.DeleteOldLogs(ctx); err != nil {
		t.Fatalf("delete old logs: %v", err)
	}

	// No logs should remain in the bucket.
	keys, err := env.store.List(ctx, "logs/")
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 logs after cleanup, got %d: %v", len(keys), keys)
	}

	// Recovery must still work using the snapshot.
	recovered := engine.New(env.store)
	if err := recovered.Initialize(ctx); err != nil {
		t.Fatalf("recover after log cleanup: %v", err)
	}
	assertValue(t, recovered, "k1", "v1")
	assertValue(t, recovered, "k2", "v2")
}

func TestMinIOMultiInstanceWritesPreserveAllData(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	// Two independent engine instances sharing the same bucket.
	a := engine.New(env.store)
	if err := a.Initialize(ctx); err != nil {
		t.Fatalf("initialize a: %v", err)
	}

	b := engine.New(env.store)
	if err := b.Initialize(ctx); err != nil {
		t.Fatalf("initialize b: %v", err)
	}

	const writesPerInstance = 10
	var wg sync.WaitGroup
	errs := make(chan error, writesPerInstance*2)

	for i := 0; i < writesPerInstance; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			errs <- a.Set(ctx, fmt.Sprintf("a-key-%02d", i), fmt.Sprintf("a-val-%02d", i))
		}(i)
		go func(i int) {
			defer wg.Done()
			errs <- b.Set(ctx, fmt.Sprintf("b-key-%02d", i), fmt.Sprintf("b-val-%02d", i))
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent write failed: %v", err)
		}
	}
	if t.Failed() {
		return
	}

	// A third engine rebuilds from storage and must see all writes.
	third := engine.New(env.store)
	if err := third.Initialize(ctx); err != nil {
		t.Fatalf("initialize third: %v", err)
	}

	for i := 0; i < writesPerInstance; i++ {
		assertValue(t, third, fmt.Sprintf("a-key-%02d", i), fmt.Sprintf("a-val-%02d", i))
		assertValue(t, third, fmt.Sprintf("b-key-%02d", i), fmt.Sprintf("b-val-%02d", i))
	}
}

func TestMinIOBackgroundCompactorCleansLogs(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	e := engine.New(env.store)
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// Write enough keys to have a non-trivial log chain.
	for i := 0; i < 10; i++ {
		if err := e.Set(ctx, fmt.Sprintf("key-%02d", i), fmt.Sprintf("val-%02d", i)); err != nil {
			t.Fatalf("set key-%02d: %v", i, err)
		}
	}

	// Start the background compactor with a short interval.
	bgCtx, bgCancel := context.WithCancel(ctx)
	defer bgCancel()

	bc := state.NewBackgroundCompactor(env.store, 100*time.Millisecond)
	go bc.Run(bgCtx)

	// Wait long enough for at least one compaction cycle.
	time.Sleep(500 * time.Millisecond)
	bgCancel()

	// All logs at or below the snapshot sequence must be gone.
	keys, err := env.store.List(ctx, "logs/")
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 logs after background compaction, got %d: %v", len(keys), keys)
	}

	// Recovery from snapshot must still return correct values.
	recovered := engine.New(env.store)
	if err := recovered.Initialize(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	for i := 0; i < 10; i++ {
		assertValue(t, recovered, fmt.Sprintf("key-%02d", i), fmt.Sprintf("val-%02d", i))
	}
}

func TestMinIOBackgroundCompactorAtScale(t *testing.T) {
	env := newTestEnvironment(t)
	ctx := context.Background()

	e := engine.New(env.store)
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// Write 100 keys across 4 concurrent writers.
	const total = 100
	const writers = 4
	var wg sync.WaitGroup
	errs := make(chan error, total)

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := w; i < total; i += writers {
				errs <- e.Set(
					ctx,
					fmt.Sprintf("scale-key-%03d", i),
					fmt.Sprintf("scale-val-%03d", i),
				)
			}
		}(w)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("write failed: %v", err)
		}
	}
	if t.Failed() {
		return
	}

	// Verify all 100 keys are readable before compaction.
	for i := 0; i < total; i++ {
		assertValue(t, e, fmt.Sprintf("scale-key-%03d", i), fmt.Sprintf("scale-val-%03d", i))
	}

	// Run background compactor with short interval.
	bgCtx, bgCancel := context.WithCancel(ctx)
	bc := state.NewBackgroundCompactor(env.store, 100*time.Millisecond)
	go bc.Run(bgCtx)

	time.Sleep(500 * time.Millisecond)
	bgCancel()

	// All logs must be cleaned up.
	keys, err := env.store.List(ctx, "logs/")
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 logs after compaction, got %d", len(keys))
	}

	// Snapshot must exist.
	snapKeys, err := env.store.List(ctx, "snapshots/")
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snapKeys) == 0 {
		t.Fatal("expected at least one snapshot after compaction")
	}

	// Fresh engine must recover all 100 keys from snapshot alone.
	recovered := engine.New(env.store)
	if err := recovered.Initialize(ctx); err != nil {
		t.Fatalf("recover: %v", err)
	}
	for i := 0; i < total; i++ {
		assertValue(t, recovered,
			fmt.Sprintf("scale-key-%03d", i),
			fmt.Sprintf("scale-val-%03d", i),
		)
	}

	// Write 20 more keys after compaction to verify log chain
	// picks up correctly after the snapshot.
	for i := total; i < total+20; i++ {
		if err := recovered.Set(
			ctx,
			fmt.Sprintf("scale-key-%03d", i),
			fmt.Sprintf("scale-val-%03d", i),
		); err != nil {
			t.Fatalf("post-compaction write %d: %v", i, err)
		}
	}

	// Final engine must see all 120 keys.
	final := engine.New(env.store)
	if err := final.Initialize(ctx); err != nil {
		t.Fatalf("final recover: %v", err)
	}
	for i := 0; i < total+20; i++ {
		assertValue(t, final,
			fmt.Sprintf("scale-key-%03d", i),
			fmt.Sprintf("scale-val-%03d", i),
		)
	}
}
