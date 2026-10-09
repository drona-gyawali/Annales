package state

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/drona-gyawali/logStore/internal/manifest"
	"github.com/drona-gyawali/logStore/internal/snapshot"
)

func TestCompactorCreatesSnapshot(t *testing.T) {
	ctx := context.Background()
	store := newFakeObjectStorage()

	store.objects["logs/1.json"] = []byte(`{
		"sequence_id": 1,
		"prev_log_key": "",
		"operations": [
			{"operation": "SET", "key": "user:1", "value": "Alice"}
		]
	}`)

	store.objects["logs/2.json"] = []byte(`{
		"sequence_id": 2,
		"prev_log_key": "logs/1.json",
		"operations": [
			{"operation": "SET", "key": "user:1", "value": "Bob"},
			{"operation": "SET", "key": "user:2", "value": "Charlie"}
		]
	}`)

	store.objects["index.json"] = []byte(`{
		"current_seq": 2,
		"current_log_key": "logs/2.json",
		"current_snapshot_seq": 0
	}`)

	compactor := NewCompactor(store)
	if err := compactor.Compact(ctx); err != nil {
		t.Fatalf("Compact() failed: %v", err)
	}

	snapshotData, ok := store.objects["snapshots/snapshot-000002.json"]
	if !ok {
		t.Fatal("expected snapshot to be created")
	}

	var snap snapshot.Snapshot
	if err := json.Unmarshal(snapshotData, &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	if snap.SequenceID != 2 {
		t.Fatalf("expected snapshot sequence 2, got %d", snap.SequenceID)
	}

	if snap.LastLogKey != "logs/2.json" {
		t.Fatalf("expected last log key logs/2.json, got %q", snap.LastLogKey)
	}

	if got := snap.Data["user:1"]; got != "Bob" {
		t.Fatalf("expected user:1=Bob, got %q", got)
	}

	if got := snap.Data["user:2"]; got != "Charlie" {
		t.Fatalf("expected user:2=Charlie, got %q", got)
	}

	var updated manifest.IndexManifest
	if err := json.Unmarshal(store.objects["index.json"], &updated); err != nil {
		t.Fatalf("decode updated manifest: %v", err)
	}

	if updated.CurrentSeq != 2 {
		t.Fatalf("expected current sequence 2, got %d", updated.CurrentSeq)
	}

	if updated.CurrentLogKey != "logs/2.json" {
		t.Fatalf("current log key changed unexpectedly: %q", updated.CurrentLogKey)
	}

	if updated.CurrentSnapshotSeq != 2 {
		t.Fatalf("expected snapshot sequence 2, got %d", updated.CurrentSnapshotSeq)
	}

	if _, ok := store.objects["logs/1.json"]; !ok {
		t.Fatal("compaction unexpectedly deleted log 1")
	}

	if _, ok := store.objects["logs/2.json"]; !ok {
		t.Fatal("compaction unexpectedly deleted log 2")
	}
}

func TestCompactorAlreadyUpToDate(t *testing.T) {
	ctx := context.Background()
	store := newFakeObjectStorage()

	store.objects["index.json"] = []byte(`{
		"current_seq": 2,
		"current_log_key": "logs/2.json",
		"current_snapshot_seq": 2
	}`)

	compactor := NewCompactor(store)
	if err := compactor.Compact(ctx); err != nil {
		t.Fatalf("Compact() failed: %v", err)
	}

	if _, exists := store.objects["snapshots/snapshot-000002.json"]; exists {
		t.Fatal("expected no new snapshot when already compacted")
	}
}

func TestCompactorCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	compactor := NewCompactor(nil)

	err := compactor.Compact(ctx)
	if err == nil {
		t.Fatal("expected an error for cancelled context")
	}

	if !strings.Contains(err.Error(), "compaction cancelled") {
		t.Fatalf("expected cancellation error, got: %v", err)
	}
}
