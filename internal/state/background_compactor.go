package state

import (
	"context"
	"log"
	"time"

	"github.com/drona-gyawali/logStore/internal/storage"
)

// BackgroundCompactor runs Compact and DeleteOldLogs on a fixed interval
// until its context is cancelled.
type BackgroundCompactor struct {
	compactor *Compactor
	interval  time.Duration
}

func NewBackgroundCompactor(
	store storage.ObjectStorage,
	interval time.Duration,
) *BackgroundCompactor {
	return &BackgroundCompactor{
		compactor: NewCompactor(store),
		interval:  interval,
	}
}

// Run blocks until ctx is cancelled. Call it in a goroutine.
func (b *BackgroundCompactor) Run(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := b.compactor.Compact(ctx); err != nil {
				log.Printf("background compaction: %v", err)
				continue
			}
			if err := b.compactor.DeleteOldLogs(ctx); err != nil {
				log.Printf("background log cleanup: %v", err)
			}
		}
	}
}
