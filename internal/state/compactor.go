package state

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/smithy-go"

	"github.com/drona-gyawali/logStore/internal/manifest"
	"github.com/drona-gyawali/logStore/internal/snapshot"
	"github.com/drona-gyawali/logStore/internal/storage"
)

type Compactor struct {
	storage      storage.ObjectStorage
	materializer *Materializer
}

func NewCompactor(objectStorage storage.ObjectStorage) *Compactor {
	return &Compactor{
		storage:      objectStorage,
		materializer: NewMaterializer(objectStorage),
	}
}

func (c *Compactor) Compact(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("compaction cancelled: %w", err)
		}

		// Read the current manifest and its ETag.
		data, etag, err := c.storage.Get(ctx, "index.json")
		if err != nil {
			return fmt.Errorf("get manifest: %w", err)
		}

		var current manifest.IndexManifest
		if err := json.Unmarshal(data, &current); err != nil {
			return fmt.Errorf("decode manifest: %w", err)
		}

		if current.CurrentSnapshotSeq > current.CurrentSeq {
			return fmt.Errorf(
				"invalid manifest: snapshot sequence %d exceeds current sequence %d",
				current.CurrentSnapshotSeq,
				current.CurrentSeq,
			)
		}

		// If the snapshot is already up to date, nothing to do.
		if current.CurrentSeq == current.CurrentSnapshotSeq {
			return nil
		}

		//  Reconstruct the latest state from the snapshot and logs.
		state, err := c.materializer.Materialize(ctx)
		if err != nil {
			return fmt.Errorf("materialize state: %w", err)
		}

		// Ensure the manifest did not change during materialization.
		_, latestETag, err := c.storage.Get(ctx, "index.json")
		if err != nil {
			return fmt.Errorf("recheck manifest: %w", err)
		}

		if latestETag != etag {
			// An append or another compactor changed the manifest.
			// Restart using the latest version.
			continue
		}

		if current.CurrentSeq > 0 && current.CurrentLogKey == "" {
			return fmt.Errorf(
				"invalid manifest: sequence %d has no current log key",
				current.CurrentSeq,
			)
		}

		//  Build a snapshot of the reconstructed state.
		snap := snapshot.Snapshot{
			SequenceID: current.CurrentSeq,
			LastLogKey: current.CurrentLogKey,
			Data:       state.data,
		}

		snapshotKey := fmt.Sprintf(
			"snapshots/snapshot-%06d.json",
			snap.SequenceID,
		)

		snapshotData, err := json.Marshal(snap)
		if err != nil {
			return fmt.Errorf("encode snapshot: %w", err)
		}

		// Create the snapshot without overwriting an existing object.
		_, err = c.storage.PutIfNoneMatch(
			ctx,
			snapshotKey,
			snapshotData,
		)
		if err != nil {
			// A concurrent compactor may already have created it.
			existingData, _, getErr := c.storage.Get(ctx, snapshotKey)
			if getErr != nil {
				return fmt.Errorf("write snapshot %q: %w", snapshotKey, err)
			}

			if !bytes.Equal(existingData, snapshotData) {
				return fmt.Errorf(
					"snapshot %q already exists with different contents",
					snapshotKey,
				)
			}
		}

		// Advance only the snapshot pointer.
		// Appends are responsible for advancing CurrentSeq and CurrentLogKey.
		nextManifest := current
		nextManifest.CurrentSnapshotSeq = current.CurrentSeq

		manifestData, err := json.Marshal(nextManifest)
		if err != nil {
			return fmt.Errorf("encode updated manifest: %w", err)
		}

		// Commit only if the manifest still has the version we read.
		_, err = c.storage.PutIfMatch(
			ctx,
			"index.json",
			manifestData,
			etag,
		)
		if err != nil {
			if isCompactionConflict(err) {
				// The manifest changed; retry against the latest version.
				continue
			}

			return fmt.Errorf("commit compacted manifest: %w", err)
		}

		// Keep old logs for now. Deleting them is a separate
		// TODO: garbage-collection step that we can implement later.
		return nil
	}
}

func isCompactionConflict(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) &&
		apiErr.ErrorCode() == "PreconditionFailed"
}

// DeleteOldLogs removes log objects whose sequence is at or below
// the snapshot sequence. Call after Compact succeeds.
func (c *Compactor) DeleteOldLogs(ctx context.Context) error {
	data, _, err := c.storage.Get(ctx, "index.json")
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	var current manifest.IndexManifest
	if err := json.Unmarshal(data, &current); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}

	if current.CurrentSnapshotSeq == 0 {
		return nil
	}

	keys, err := c.storage.List(ctx, "logs/")
	if err != nil {
		return fmt.Errorf("list logs: %w", err)
	}

	for _, key := range keys {
		var seq uint64
		if _, err := fmt.Sscanf(key, "logs/%d-", &seq); err != nil {
			continue
		}
		if seq > current.CurrentSnapshotSeq {
			continue
		}
		if err := c.storage.Delete(ctx, key); err != nil {
			return fmt.Errorf("delete log %q: %w", key, err)
		}
	}

	return nil
}
