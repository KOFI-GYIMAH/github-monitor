package worker

import (
	"context"
	"time"

	"github.com/KOFI-GYIMAH/github-monitor/internal/service"
	"github.com/KOFI-GYIMAH/github-monitor/pkg/logger"
)

type SyncWorker struct {
	service  *service.RepositoryService
	interval time.Duration
	owner    string
	repo     string
}

func NewSyncWorker(service *service.RepositoryService, interval time.Duration, owner, repo string) *SyncWorker {
	return &SyncWorker{
		service:  service,
		interval: interval,
		owner:    owner,
		repo:     repo,
	}
}

func (w *SyncWorker) Run(ctx context.Context) {
	err := w.service.SyncRepository(ctx, w.owner, w.repo, time.Time{})
	if err != nil {
		logger.Error("initial sync failed: %v", err)
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// * Get last sync time from DB
			fullRepoName := w.owner + "/" + w.repo
			repo, err := w.service.GetRepository(ctx, fullRepoName)
			if err != nil {
				logger.Error("failed to get repository: %v", err)
				continue
			}

			var since time.Time
			if repo != nil && repo.LastCommitFetchedAt != nil {
				since = *repo.LastCommitFetchedAt
			}

			err = w.service.SyncRepository(ctx, w.owner, w.repo, since)
			if err != nil {
				logger.Error("sync failed: %v", err)
			} else {
				logger.Info("successfully synced repository %s", fullRepoName)
			}

		case <-ctx.Done():
			logger.Info("stopping sync worker")
			return
		}
	}
}
