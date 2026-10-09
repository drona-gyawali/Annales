package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/aws/smithy-go"

	"github.com/drona-gyawali/logStore/internal/log"
	"github.com/drona-gyawali/logStore/internal/manifest"
	"github.com/drona-gyawali/logStore/internal/state"
	"github.com/drona-gyawali/logStore/internal/storage"
)

// LogStorage combines ordinary object storage with the append operation.
type LogStorage interface {
	storage.ObjectStorage
	Append(ctx context.Context, operations []log.LogEntry) error
}

// Engine coordinates durable writes and in-memory reads.
type Engine struct {
	storage      LogStorage
	materializer *state.Materializer

	mu           sync.RWMutex
	currentState *state.State
}

// New creates an engine using the supplied storage implementation.
func New(store LogStorage) *Engine {
	return &Engine{
		storage:      store,
		materializer: state.NewMaterializer(store),
	}
}

// Initialize ensures the manifest exists and reconstructs the state once.
// The engine is ready to serve requests only after reconstruction succeeds.
func (e *Engine) Initialize(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Initialization is idempotent for an already initialized engine.
	if e.currentState != nil {
		return nil
	}

	_, _, err := e.storage.Get(ctx, "index.json")
	if err != nil {
		var apiErr smithy.APIError
		if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "NoSuchKey" {
			return fmt.Errorf("check manifest: %w", err)
		}

		manifestData, marshalErr := json.Marshal(manifest.IndexManifest{})
		if marshalErr != nil {
			return fmt.Errorf("encode initial manifest: %w", marshalErr)
		}

		if _, putErr := e.storage.PutIfNoneMatch(
			ctx,
			"index.json",
			manifestData,
		); putErr != nil {
			// Another initializer may have created the manifest first.
			if _, _, getErr := e.storage.Get(ctx, "index.json"); getErr != nil {
				return fmt.Errorf("initialize manifest: %w", putErr)
			}
		}
	}

	// Build the complete state from the snapshot and committed log chain.
	loadedState, err := e.materializer.Materialize(ctx)
	if err != nil {
		return fmt.Errorf("materialize initial state: %w", err)
	}

	e.currentState = loadedState
	return nil
}

// Set durably records a key-value pair before updating RAM.
func (e *Engine) Set(ctx context.Context, key, value string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.currentState == nil {
		return fmt.Errorf("engine is not initialized")
	}

	entry := log.LogEntry{
		Operation: "SET",
		Key:       key,
		Value:     value,
	}

	if err := e.storage.Append(ctx, []log.LogEntry{entry}); err != nil {
		return err
	}

	e.currentState.Apply(entry)
	return nil
}

// Get reads directly from the in-memory state.
func (e *Engine) Get(
	ctx context.Context,
	key string,
) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.currentState == nil {
		return "", false, fmt.Errorf("engine is not initialized")
	}

	value, exists := e.currentState.Get(key)
	return value, exists, nil
}

// Delete durably records a deletion before updating RAM.
func (e *Engine) Delete(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.currentState == nil {
		return fmt.Errorf("engine is not initialized")
	}

	entry := log.LogEntry{
		Operation: "DELETE",
		Key:       key,
	}

	if err := e.storage.Append(ctx, []log.LogEntry{entry}); err != nil {
		return err
	}

	e.currentState.Apply(entry)
	return nil
}
