package state

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/drona-gyawali/logStore/internal/log"
	"github.com/drona-gyawali/logStore/internal/manifest"
	"github.com/drona-gyawali/logStore/internal/snapshot"
	"github.com/drona-gyawali/logStore/internal/storage"
)

type Materializer struct {
	storage storage.ObjectStorage
}

func NewMaterializer(storage storage.ObjectStorage) *Materializer {
	return &Materializer{
		storage: storage,
	}
}

func (m *Materializer) loadManifest(
	ctx context.Context,
) (manifest.IndexManifest, error) {
	data, _, err := m.storage.Get(ctx, "index.json")
	if err != nil {
		return manifest.IndexManifest{}, fmt.Errorf(
			"get manifest: %w",
			err,
		)
	}

	var current manifest.IndexManifest

	if err := json.Unmarshal(data, &current); err != nil {
		return manifest.IndexManifest{}, fmt.Errorf(
			"decode manifest: %w",
			err,
		)
	}

	return current, nil
}

func (m *Materializer) loadLog(
	ctx context.Context,
	key string,
) (log.LogBatch, error) {
	data, _, err := m.storage.Get(ctx, key)
	if err != nil {
		return log.LogBatch{}, fmt.Errorf(
			"get log %q: %w",
			key,
			err,
		)
	}

	var batch log.LogBatch

	if err := json.Unmarshal(data, &batch); err != nil {
		return log.LogBatch{}, fmt.Errorf(
			"decode log %q: %w",
			key,
			err,
		)
	}

	return batch, nil
}

func (m *Materializer) loadLogChain(
	ctx context.Context,
	startKey string,
	snapshotSeq uint64,
) ([]log.LogBatch, error) {
	var batches []log.LogBatch

	currentKey := startKey

	for currentKey != "" {
		batch, err := m.loadLog(ctx, currentKey)
		if err != nil {
			return nil, err
		}

		if batch.SequenceID <= snapshotSeq {
			break
		}

		batches = append(batches, batch)

		currentKey = batch.PrevLogKey
	}

	return batches, nil
}

func reverseBatches(batches []log.LogBatch) {
	for left, right := 0, len(batches)-1; left < right; left, right = left+1, right-1 {
		batches[left], batches[right] = batches[right], batches[left]
	}
}

func (m *Materializer) loadSnapshot(
	ctx context.Context,
	sequenceID uint64,
) (snapshot.Snapshot, error) {
	key := fmt.Sprintf(
		"snapshots/snapshot-%06d.json",
		sequenceID,
	)

	data, _, err := m.storage.Get(ctx, key)
	if err != nil {
		return snapshot.Snapshot{}, fmt.Errorf(
			"get snapshot %q: %w",
			key,
			err,
		)
	}

	var snap snapshot.Snapshot

	if err := json.Unmarshal(data, &snap); err != nil {
		return snapshot.Snapshot{}, fmt.Errorf(
			"decode snapshot %q: %w",
			key,
			err,
		)
	}

	return snap, nil
}

func (m *Materializer) Materialize(
	ctx context.Context,
) (*State, error) {
	currentManifest, err := m.loadManifest(ctx)
	if err != nil {
		return nil, err
	}

	state := New()

	// Restore the latest snapshot if one exists.
	if currentManifest.CurrentSnapshotSeq > 0 {
		snap, err := m.loadSnapshot(
			ctx,
			currentManifest.CurrentSnapshotSeq,
		)
		if err != nil {
			return nil, err
		}

		state.Load(snap.Data)
	}

	// Replay only logs newer than the snapshot.
	if currentManifest.CurrentLogKey == "" {
		return state, nil
	}

	batches, err := m.loadLogChain(
		ctx,
		currentManifest.CurrentLogKey,
		currentManifest.CurrentSnapshotSeq,
	)
	if err != nil {
		return nil, err
	}

	reverseBatches(batches)

	for _, batch := range batches {
		state.ApplyBatch(batch)
	}

	return state, nil
}
