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
	snapshotLastLogKey string,
	currentSeq uint64,
	snapshotSeq uint64,
) ([]log.LogBatch, error) {
	var batches []log.LogBatch

	currentKey := startKey
	expectedSeq := currentSeq

	for currentKey != "" && currentKey != snapshotLastLogKey {
		if expectedSeq == 0 ||
			(snapshotLastLogKey != "" && expectedSeq <= snapshotSeq) {
			return nil, fmt.Errorf(
				"unexpected log %q at or before snapshot sequence %d",
				currentKey,
				snapshotSeq,
			)
		}

		batch, err := m.loadLog(ctx, currentKey)
		if err != nil {
			return nil, err
		}

		if batch.SequenceID != expectedSeq {
			return nil, fmt.Errorf(
				"log %q has sequence %d; expected %d",
				currentKey,
				batch.SequenceID,
				expectedSeq,
			)
		}

		batches = append(batches, batch)
		currentKey = batch.PrevLogKey
		expectedSeq--
	}

	if snapshotLastLogKey != "" && currentKey != snapshotLastLogKey {
		return nil, fmt.Errorf(
			"log chain did not reach snapshot boundary %q",
			snapshotLastLogKey,
		)
	}

	if expectedSeq != snapshotSeq {
		return nil, fmt.Errorf(
			"log chain ended at sequence %d; expected snapshot sequence %d",
			expectedSeq,
			snapshotSeq,
		)
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
	snapshotSeq := currentManifest.CurrentSnapshotSeq
	snapshotLastLogKey := ""

	if snapshotSeq > 0 {
		snap, err := m.loadSnapshot(ctx, snapshotSeq)
		if err != nil {
			return nil, err
		}

		if snap.SequenceID != snapshotSeq {
			return nil, fmt.Errorf(
				"snapshot sequence mismatch: manifest says %d, snapshot says %d",
				snapshotSeq,
				snap.SequenceID,
			)
		}

		if snap.LastLogKey == "" {
			return nil, fmt.Errorf(
				"snapshot at sequence %d has no last log key",
				snapshotSeq,
			)
		}

		state.Load(snap.Data)
		snapshotLastLogKey = snap.LastLogKey
	}

	if currentManifest.CurrentLogKey == "" {
		if currentManifest.CurrentSeq != snapshotSeq {
			return nil, fmt.Errorf(
				"manifest has current sequence %d but no current log key; snapshot sequence is %d",
				currentManifest.CurrentSeq,
				snapshotSeq,
			)
		}

		return state, nil
	}

	batches, err := m.loadLogChain(
		ctx,
		currentManifest.CurrentLogKey,
		snapshotLastLogKey,
		currentManifest.CurrentSeq,
		snapshotSeq,
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
