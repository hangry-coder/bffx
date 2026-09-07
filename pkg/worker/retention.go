package worker

import (
	"github.com/hangry-coder/bffx/pkg/logger"
	"context"
	"time"
)

// StartRetentionWorker starts a background goroutine that prunes old jobs periodically.
func StartRetentionWorker(ctx context.Context, store JobStore, days int) {
	if days <= 0 {
		logger.Warn("Job retention disabled (days=%d)", days)
		return
	}

	logger.Info("Starting job retention worker (retention_days=%d)", days)
	
	// Initial prune on startup
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Job retention worker panicked: %v\n%s", r, logger.Stack())
			}
		}()
		prune(ctx, store, days)
		
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("Job retention worker stopping")
				return
			case <-ticker.C:
				prune(ctx, store, days)
			}
		}
	}()
}

func prune(ctx context.Context, store JobStore, days int) {
	count, err := store.Prune(ctx, days)
	if err != nil {
		logger.Error("Job retention prune failed: %v", err)
	} else if count > 0 {
		logger.Info("Job retention: pruned %d jobs older than %d days", count, days)
	}
}
