package state

import (
	"context"
	"fmt"
	"testing"

	"github.com/drona-gyawali/logStore/internal/storage"
)

type fakeObjectStorage struct {
	objects map[string][]byte
}

func newFakeObjectStorage() *fakeObjectStorage {
	return &fakeObjectStorage{
		objects: make(map[string][]byte),
	}
}

func (f *fakeObjectStorage) Get(
	ctx context.Context,
	key string,
) ([]byte, string, error) {
	data, ok := f.objects[key]
	if !ok {
		return nil, "", fmt.Errorf("object %q not found", key)
	}

	return data, "fake-etag", nil
}

func (f *fakeObjectStorage) Put(
	ctx context.Context,
	key string,
	data []byte,
) (string, error) {
	f.objects[key] = data
	return "fake-etag", nil
}

func (f *fakeObjectStorage) PutIfMatch(
	ctx context.Context,
	key string,
	data []byte,
	etag string,
) (string, error) {
	f.objects[key] = data
	return "fake-etag", nil
}

func (f *fakeObjectStorage) PutIfNoneMatch(
	ctx context.Context,
	key string,
	data []byte,
) (string, error) {
	if _, exists := f.objects[key]; exists {
		return "", fmt.Errorf("object %q already exists", key)
	}

	f.objects[key] = data
	return "fake-etag", nil
}

func (f *fakeObjectStorage) Delete(
	ctx context.Context,
	key string,
) error {
	delete(f.objects, key)
	return nil
}

var _ storage.ObjectStorage = (*fakeObjectStorage)(nil)

func TestMaterializer(t *testing.T) {
	ctx := context.Background()
	store := newFakeObjectStorage()

	// Snapshot at sequence 2.
	snapshotData := []byte(`{
		"sequence_id": 2,
		"data": {
			"user:1": "Alice",
			"user:2": "Bob"
		}
	}`)

	store.objects["snapshots/snapshot-000002.json"] = snapshotData

	// Log 2 — snapshot boundary.
	log2 := []byte(`{
	  "sequence_id": 2,
	  "prev_log_key": "",
	  "operations": []
       }`)

	store.objects["logs/2.json"] = log2

	// Log 3.
	log3 := []byte(`{
		"sequence_id": 3,
		"prev_log_key": "logs/2.json",
		"operations": [
			{
				"operation": "SET",
				"key": "user:1",
				"value": "Charlie"
			}
		]
	}`)

	store.objects["logs/3.json"] = log3

	// Log 4.
	log4 := []byte(`{
		"sequence_id": 4,
		"prev_log_key": "logs/3.json",
		"operations": [
			{
				"operation": "DELETE",
				"key": "user:2"
			},
			{
				"operation": "SET",
				"key": "user:3",
				"value": "David"
			}
		]
	}`)

	store.objects["logs/4.json"] = log4

	// Current manifest points to log 4
	// and says the snapshot is at sequence 2.
	manifestData := []byte(`{
		"current_seq": 4,
		"current_log_key": "logs/4.json",
		"current_snapshot_seq": 2
	}`)

	store.objects["index.json"] = manifestData

	materializer := NewMaterializer(store)

	state, err := materializer.Materialize(ctx)
	if err != nil {
		t.Fatalf("Materialize() failed: %v", err)
	}

	// user:1 should have been updated by log 3.
	value, ok := state.Get("user:1")
	if !ok {
		t.Fatal("expected user:1 to exist")
	}

	if value != "Charlie" {
		t.Fatalf("expected user:1=Charlie, got %q", value)
	}

	// user:2 existed in the snapshot but was deleted by log 4.
	_, ok = state.Get("user:2")
	if ok {
		t.Fatal("expected user:2 to be deleted")
	}

	// user:3 was created by log 4.
	value, ok = state.Get("user:3")
	if !ok {
		t.Fatal("expected user:3 to exist")
	}

	if value != "David" {
		t.Fatalf("expected user:3=David, got %q", value)
	}
}

func TestMaterializerWithoutSnapshot(t *testing.T) {
	ctx := context.Background()
	store := newFakeObjectStorage()

	// Log 1.
	log1 := []byte(`{
		"sequence_id": 1,
		"prev_log_key": "",
		"operations": [
			{
				"operation": "SET",
				"key": "user:1",
				"value": "Alice"
			}
		]
	}`)

	store.objects["logs/1.json"] = log1

	// Log 2.
	log2 := []byte(`{
		"sequence_id": 2,
		"prev_log_key": "logs/1.json",
		"operations": [
			{
				"operation": "SET",
				"key": "user:2",
				"value": "Bob"
			}
		]
	}`)

	store.objects["logs/2.json"] = log2

	// Log 3.
	log3 := []byte(`{
		"sequence_id": 3,
		"prev_log_key": "logs/2.json",
		"operations": [
			{
				"operation": "SET",
				"key": "user:1",
				"value": "Charlie"
			}
		]
	}`)

	store.objects["logs/3.json"] = log3

	// No snapshot exists.
	manifestData := []byte(`{
		"current_seq": 3,
		"current_log_key": "logs/3.json",
		"current_snapshot_seq": 0
	}`)

	store.objects["index.json"] = manifestData

	materializer := NewMaterializer(store)

	state, err := materializer.Materialize(ctx)
	if err != nil {
		t.Fatalf("Materialize() failed: %v", err)
	}

	// user:1 should have been updated by log 3.
	value, ok := state.Get("user:1")
	if !ok {
		t.Fatal("expected user:1 to exist")
	}

	if value != "Charlie" {
		t.Fatalf("expected user:1=Charlie, got %q", value)
	}

	// user:2 should come from log 2.
	value, ok = state.Get("user:2")
	if !ok {
		t.Fatal("expected user:2 to exist")
	}

	if value != "Bob" {
		t.Fatalf("expected user:2=Bob, got %q", value)
	}
}

func TestMaterializerMissingManifest(t *testing.T) {
	ctx := context.Background()
	store := newFakeObjectStorage()

	materializer := NewMaterializer(store)

	_, err := materializer.Materialize(ctx)
	if err == nil {
		t.Fatal("expected error when index.json is missing")
	}
}

func TestMaterializerInvalidManifest(t *testing.T) {
	ctx := context.Background()
	store := newFakeObjectStorage()

	store.objects["index.json"] = []byte(`{
		"current_seq": 4,
		"current_log_key":
	}`)

	materializer := NewMaterializer(store)

	_, err := materializer.Materialize(ctx)
	if err == nil {
		t.Fatal("expected error when index.json is invalid")
	}
}

func TestMaterializerMissingLog(t *testing.T) {
	ctx := context.Background()
	store := newFakeObjectStorage()

	// Manifest points to a log that does not exist.
	store.objects["index.json"] = []byte(`{
		"current_seq": 1,
		"current_log_key": "logs/1.json",
		"current_snapshot_seq": 0
	}`)

	materializer := NewMaterializer(store)

	_, err := materializer.Materialize(ctx)
	if err == nil {
		t.Fatal("expected error when log is missing")
	}
}
