package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/aws/smithy-go"

	"github.com/drona-gyawali/logStore/internal/storage"
)

type fakeObjectStorage struct {
	objects map[string][]byte
}

type fakePreconditionFailed struct{}

func (fakePreconditionFailed) Error() string {
	return "precondition failed"
}

func (fakePreconditionFailed) ErrorCode() string {
	return "PreconditionFailed"
}

func (fakePreconditionFailed) ErrorMessage() string {
	return "precondition failed"
}

func (fakePreconditionFailed) ErrorFault() smithy.ErrorFault {
	return smithy.FaultClient
}

func fakeETag(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
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

	return data, fakeETag(data), nil
}

func (f *fakeObjectStorage) Put(
	ctx context.Context,
	key string,
	data []byte,
) (string, error) {
	f.objects[key] = data
	return fakeETag(data), nil
}

func (f *fakeObjectStorage) PutIfMatch(
	ctx context.Context,
	key string,
	data []byte,
	etag string,
) (string, error) {
	current, exists := f.objects[key]
	if !exists || fakeETag(current) != etag {
		return "", fakePreconditionFailed{}
	}

	f.objects[key] = data
	return fakeETag(data), nil
}

func (f *fakeObjectStorage) PutIfNoneMatch(
	ctx context.Context,
	key string,
	data []byte,
) (string, error) {
	if _, exists := f.objects[key]; exists {
		return "", fakePreconditionFailed{}
	}

	f.objects[key] = data
	return fakeETag(data), nil
}

func (f *fakeObjectStorage) Delete(
	ctx context.Context,
	key string,
) error {
	delete(f.objects, key)
	return nil
}

func (f *fakeObjectStorage) List(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

var _ storage.ObjectStorage = (*fakeObjectStorage)(nil)
var _ smithy.APIError = fakePreconditionFailed{}

func TestMaterializer(t *testing.T) {
	ctx := context.Background()
	store := newFakeObjectStorage()

	store.objects["snapshots/snapshot-000002.json"] = []byte(`{
		"sequence_id": 2,
		"last_log_key": "logs/2.json",
		"data": {"user:1": "Alice", "user:2": "Bob"}
	}`)

	store.objects["logs/3.json"] = []byte(`{
		"sequence_id": 3,
		"prev_log_key": "logs/2.json",
		"operations": [
			{"operation": "SET", "key": "user:1", "value": "Charlie"}
		]
	}`)

	store.objects["logs/4.json"] = []byte(`{
		"sequence_id": 4,
		"prev_log_key": "logs/3.json",
		"operations": [
			{"operation": "DELETE", "key": "user:2", "value": ""},
			{"operation": "SET", "key": "user:3", "value": "David"}
		]
	}`)

	store.objects["index.json"] = []byte(`{
		"current_seq": 4,
		"current_log_key": "logs/4.json",
		"current_snapshot_seq": 2
	}`)

	state, err := NewMaterializer(store).Materialize(ctx)
	if err != nil {
		t.Fatalf("Materialize() failed: %v", err)
	}

	if value, ok := state.Get("user:1"); !ok || value != "Charlie" {
		t.Fatalf("expected user:1=Charlie, got %q, exists=%v", value, ok)
	}

	if _, ok := state.Get("user:2"); ok {
		t.Fatal("expected user:2 to be deleted")
	}

	if value, ok := state.Get("user:3"); !ok || value != "David" {
		t.Fatalf("expected user:3=David, got %q, exists=%v", value, ok)
	}
}

func TestMaterializerWithoutSnapshot(t *testing.T) {
	store := newFakeObjectStorage()

	store.objects["logs/1.json"] = []byte(`{
		"sequence_id": 1, "prev_log_key": "",
		"operations": [{"operation": "SET", "key": "user:1", "value": "Alice"}]
	}`)
	store.objects["logs/2.json"] = []byte(`{
		"sequence_id": 2, "prev_log_key": "logs/1.json",
		"operations": [{"operation": "SET", "key": "user:2", "value": "Bob"}]
	}`)
	store.objects["logs/3.json"] = []byte(`{
		"sequence_id": 3, "prev_log_key": "logs/2.json",
		"operations": [{"operation": "SET", "key": "user:1", "value": "Charlie"}]
	}`)
	store.objects["index.json"] = []byte(`{
		"current_seq": 3, "current_log_key": "logs/3.json",
		"current_snapshot_seq": 0
	}`)

	state, err := NewMaterializer(store).Materialize(context.Background())
	if err != nil {
		t.Fatalf("Materialize() failed: %v", err)
	}

	if value, ok := state.Get("user:1"); !ok || value != "Charlie" {
		t.Fatalf("expected user:1=Charlie, got %q, exists=%v", value, ok)
	}
	if value, ok := state.Get("user:2"); !ok || value != "Bob" {
		t.Fatalf("expected user:2=Bob, got %q, exists=%v", value, ok)
	}
}

func TestMaterializerMissingManifest(t *testing.T) {
	store := newFakeObjectStorage()
	if _, err := NewMaterializer(store).Materialize(context.Background()); err == nil {
		t.Fatal("expected error when index.json is missing")
	}
}

func TestMaterializerInvalidManifest(t *testing.T) {
	store := newFakeObjectStorage()
	store.objects["index.json"] = []byte(`{"current_seq":`)

	if _, err := NewMaterializer(store).Materialize(context.Background()); err == nil {
		t.Fatal("expected error when index.json is invalid")
	}
}

func TestMaterializerMissingLog(t *testing.T) {
	store := newFakeObjectStorage()
	store.objects["index.json"] = []byte(`{
		"current_seq": 1, "current_log_key": "logs/1.json",
		"current_snapshot_seq": 0
	}`)

	if _, err := NewMaterializer(store).Materialize(context.Background()); err == nil {
		t.Fatal("expected error when log is missing")
	}
}

func TestMaterializerBrokenLogChain(t *testing.T) {
	store := newFakeObjectStorage()
	store.objects["snapshots/snapshot-000002.json"] = []byte(`{
		"sequence_id": 2, "last_log_key": "logs/2.json",
		"data": {"user:1": "Alice"}
	}`)
	store.objects["logs/3.json"] = []byte(`{
		"sequence_id": 3, "prev_log_key": "logs/missing.json",
		"operations": [{"operation": "SET", "key": "user:1", "value": "Charlie"}]
	}`)
	store.objects["index.json"] = []byte(`{
		"current_seq": 3, "current_log_key": "logs/3.json",
		"current_snapshot_seq": 2
	}`)

	if _, err := NewMaterializer(store).Materialize(context.Background()); err == nil {
		t.Fatal("expected error when log chain does not reach snapshot boundary")
	}
}

func TestMaterializerRejectsSequenceMismatch(t *testing.T) {
	store := newFakeObjectStorage()
	store.objects["logs/3.json"] = []byte(`{
		"sequence_id": 7, "prev_log_key": "logs/2.json",
		"operations": []
	}`)
	store.objects["index.json"] = []byte(`{
		"current_seq": 3, "current_log_key": "logs/3.json",
		"current_snapshot_seq": 0
	}`)

	if _, err := NewMaterializer(store).Materialize(context.Background()); err == nil {
		t.Fatal("expected error for log sequence mismatch")
	}
}

func TestMaterializerIgnoresOrphanLog(t *testing.T) {
	store := newFakeObjectStorage()

	// This is the committed log chain.
	store.objects["logs/1-committed.json"] = []byte(`{
		"sequence_id": 1,
		"prev_log_key": "",
		"operations": [
			{"operation": "SET", "key": "user:1", "value": "Alice"}
		]
	}`)

	// This log exists in storage but is not referenced by the manifest
	// or by any log in the committed chain.
	store.objects["logs/1-orphan.json"] = []byte(`{
		"sequence_id": 1,
		"prev_log_key": "",
		"operations": [
			{"operation": "SET", "key": "user:1", "value": "Mallory"}
		]
	}`)

	store.objects["index.json"] = []byte(`{
		"current_seq": 1,
		"current_log_key": "logs/1-committed.json",
		"current_snapshot_seq": 0
	}`)

	state, err := NewMaterializer(store).Materialize(context.Background())
	if err != nil {
		t.Fatalf("Materialize() failed: %v", err)
	}

	if value, ok := state.Get("user:1"); !ok || value != "Alice" {
		t.Fatalf("expected committed value Alice, got %q, exists=%v", value, ok)
	}
}
