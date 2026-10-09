package state

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/drona-gyawali/logStore/internal/manifest"
	"github.com/drona-gyawali/logStore/internal/snapshot"
)

// appendDuringLogReadStorage simulates another writer appending sequence 3
// while the compactor is reading the sequence-2 log.
type appendDuringLogReadStorage struct {
	*fakeObjectStorage
	appended bool
}

func (s *appendDuringLogReadStorage) Get(
	ctx context.Context,
	key string,
) ([]byte, string, error) {
	data, etag, err := s.fakeObjectStorage.Get(ctx, key)
	if err != nil {
		return nil, "", err
	}

	if key == "logs/2.json" && !s.appended {
		s.appended = true

		// Simulate a writer creating the next immutable log.
		s.objects["logs/3.json"] = []byte(`{
			"sequence_id": 3,
			"prev_log_key": "logs/2.json",
			"operations": [
				{"operation": "SET", "key": "user:3", "value": "Dorna"}
			]
		}`)

		// Simulate the writer atomically advancing index.json.
		s.objects["index.json"] = []byte(`{
			"current_seq": 3,
			"current_log_key": "logs/3.json",
			"current_snapshot_seq": 0
		}`)
	}

	// Return the log data and ETag obtained before the simulated append.
	return data, etag, nil
}

func TestCompactorRetriesWhenAppendOccursDuringMaterialization(t *testing.T) {
	ctx := context.Background()

	base := newFakeObjectStorage()

	base.objects["logs/1.json"] = []byte(`{
		"sequence_id": 1,
		"prev_log_key": "",
		"operations": [
			{"operation": "SET", "key": "user:1", "value": "Alice"}
		]
	}`)

	base.objects["logs/2.json"] = []byte(`{
		"sequence_id": 2,
		"prev_log_key": "logs/1.json",
		"operations": [
			{"operation": "SET", "key": "user:2", "value": "Bob"}
		]
	}`)

	base.objects["index.json"] = []byte(`{
		"current_seq": 2,
		"current_log_key": "logs/2.json",
		"current_snapshot_seq": 0
	}`)

	store := &appendDuringLogReadStorage{
		fakeObjectStorage: base,
	}

	compactor := NewCompactor(store)

	if err := compactor.Compact(ctx); err != nil {
		t.Fatalf("Compact() failed: %v", err)
	}

	if !store.appended {
		t.Fatal("test did not simulate the concurrent append")
	}

	// The compactor must retry and produce a snapshot for sequence 3,
	// not commit the older sequence-2 snapshot as the latest snapshot.
	snapshotData, ok := base.objects["snapshots/snapshot-000003.json"]
	if !ok {
		t.Fatal("expected snapshot for sequence 3 after retry")
	}

	var snap snapshot.Snapshot
	if err := json.Unmarshal(snapshotData, &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	if snap.SequenceID != 3 {
		t.Fatalf("expected snapshot sequence 3, got %d", snap.SequenceID)
	}

	if snap.LastLogKey != "logs/3.json" {
		t.Fatalf("expected last log key logs/3.json, got %q", snap.LastLogKey)
	}

	if got := snap.Data["user:1"]; got != "Alice" {
		t.Fatalf("expected user:1=Alice, got %q", got)
	}

	if got := snap.Data["user:2"]; got != "Bob" {
		t.Fatalf("expected user:2=Bob, got %q", got)
	}

	if got := snap.Data["user:3"]; got != "Dorna" {
		t.Fatalf("expected user:3=Dorna, got %q", got)
	}

	var updated manifest.IndexManifest
	if err := json.Unmarshal(base.objects["index.json"], &updated); err != nil {
		t.Fatalf("decode updated manifest: %v", err)
	}

	if updated.CurrentSeq != 3 {
		t.Fatalf("expected current sequence 3, got %d", updated.CurrentSeq)
	}

	if updated.CurrentLogKey != "logs/3.json" {
		t.Fatalf("expected current log key logs/3.json, got %q", updated.CurrentLogKey)
	}

	if updated.CurrentSnapshotSeq != 3 {
		t.Fatalf(
			"expected snapshot sequence 3 after retry, got %d",
			updated.CurrentSnapshotSeq,
		)
	}
}
