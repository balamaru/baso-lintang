package jobs

import (
	"context"
	"log/slog"
	"time"
)

type Purger interface{ Purge(context.Context) error }

func Run(ctx context.Context, p Purger, log *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		taskCtx, cancel := context.WithTimeout(ctx, time.Minute)
		if err := p.Purge(taskCtx); err != nil && ctx.Err() == nil {
			log.Error("privacy cleanup failed")
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
