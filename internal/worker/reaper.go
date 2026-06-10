package worker

import (
	"context"
	"log/slog"
	"os"
	"time"

	"sovereign-pdf/internal/engine"
)

func StartTTLReaper(ctx context.Context, store *JobStore, retentionPeriod time.Duration) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	slog.Info("TTL reaper started", "retention", retentionPeriod)
	for {
		select {
		case <-ctx.Done():
			slog.Info("TTL reaper shutting down")
			return
		case <-ticker.C:
			for _, job := range store.List() {
				if job.Status != engine.StatusCompleted && job.Status != engine.StatusFailed {
					continue
				}
				if time.Since(job.CompletedAt) < retentionPeriod {
					continue
				}
				if job.OutputFile == "" {
					continue
				}
				if _, err := os.Stat(job.OutputFile); os.IsNotExist(err) {
					continue
				}
				if err := os.RemoveAll(job.OutputFile); err != nil {
					slog.Error("reaper failed to purge file", "job_id", job.ID, "file", job.OutputFile, "error", err)
				} else {
					slog.Info("reaper purged stale file", "job_id", job.ID, "file", job.OutputFile)
				}
			}
		}
	}
}
