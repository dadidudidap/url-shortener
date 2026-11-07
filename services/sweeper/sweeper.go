package main

import (
	"context"
	"time"
)

type guestLinkStore interface {
	DeleteExpiredGuestURLs(ctx context.Context, limit int32) ([]string, error)
}

func sweepOnce(ctx context.Context, store guestLinkStore, batchSize int32) (int, error) {
	total := 0
	for {
		deleted, err := store.DeleteExpiredGuestURLs(ctx, batchSize)
		if err != nil {
			return total, err
		}
		total += len(deleted)
		if len(deleted) < int(batchSize) {
			return total, nil
		}
	}
}

func Run(ctx context.Context, store guestLinkStore, interval time.Duration, batchSize int32, logf func(string, ...any)) {
	sweep := func() {
		deleted, err := sweepOnce(ctx, store, batchSize)
		if err != nil {
			logf("Sweeper failed: %v", err)
			return
		}
		if deleted > 0 {
			logf("Deleted %d expired guest links", deleted)
		}
	}

	sweep()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweep()
		}
	}
}
