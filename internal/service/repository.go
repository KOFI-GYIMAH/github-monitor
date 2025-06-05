package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/KOFI-GYIMAH/github-monitor/internal/github"
	"github.com/KOFI-GYIMAH/github-monitor/internal/models"
	"github.com/KOFI-GYIMAH/github-monitor/pkg/logger"
)

type RepositoryService struct {
	githubClient *github.Client
	db           models.Database
}

func NewRepositoryService(githubClient *github.Client, db models.Database) *RepositoryService {
	return &RepositoryService{
		githubClient: githubClient,
		db:           db,
	}
}

func (s *RepositoryService) GetRepository(ctx context.Context, name string) (*models.Repository, error) {
	return s.db.GetRepository(ctx, name)
}

func (s *RepositoryService) SyncRepository(ctx context.Context, owner, name string, since time.Time) error {
	logger.Info("Syncing repository...")
	return s.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		repo, err := s.githubClient.GetRepository(ctx, owner, name)
		if err != nil {
			return fmt.Errorf("failed to get repository: %w", err)
		}

		logger.Info("Successfully synced repository %s", repo.FullName)

		// * Save repository metadata
		dbRepo := models.Repository{
			Name:            repo.FullName,
			Description:     repo.Description,
			URL:             repo.HTMLURL,
			Language:        repo.Language,
			ForksCount:      repo.ForksCount,
			StarsCount:      repo.StargazersCount,
			OpenIssuesCount: repo.OpenIssuesCount,
			WatchersCount:   repo.WatchersCount,
			CreatedAt:       repo.CreatedAt,
			UpdatedAt:       repo.UpdatedAt,
		}

		err = s.db.UpsertRepositoryTx(ctx, tx, &dbRepo)
		if err != nil {
			return fmt.Errorf("failed to save repository: %w", err)
		}

		logger.Info("Successfully saved repository %s", repo.FullName)

		// * Get commits since last sync
		commitOpts := github.CommitListOptions{Since: since}
		commits, err := s.githubClient.ListCommits(ctx, owner, name, commitOpts)
		if err != nil {
			return fmt.Errorf("failed to list commits: %w", err)
		}

		logger.Info("Successfully fetched %d commits", len(commits))

		// * Save commits
		for _, commit := range commits {
			author := commit.Author

			dbCommit := models.Commit{
				SHA:          commit.SHA,
				RepositoryID: dbRepo.ID,
				Message:      commit.Commit.Message,
				AuthorName:   author.Login,
				AuthorEmail:  commit.Commit.Author.Email,
				AuthorDate:   commit.Commit.Author.Date,
				CommitURL:    commit.HTMLURL,
			}

			err = s.db.InsertCommitTx(ctx, tx, &dbCommit)
			if err != nil {
				return fmt.Errorf("failed to insert commit: %w", err)
			}

			logger.Info("Successfully saved commit %s", commit.SHA)
		}

		logger.Info("Successfully synced repository %s", repo.FullName)
		// * Update last sync time
		now := time.Now()
		dbRepo.LastCommitFetchedAt = &now
		return s.db.UpdateRepositoryTx(ctx, tx, &dbRepo)
	})
}

func (s *RepositoryService) GetTopAuthors(ctx context.Context, repoName string, limit int) ([]models.AuthorCommitCount, error) {
	return s.db.GetTopAuthors(ctx, repoName, limit)
}

func (s *RepositoryService) GetCommits(ctx context.Context, repoName string, since, until *time.Time) ([]models.Commit, error) {
	return s.db.GetCommits(ctx, repoName, since, until)
}

func (s *RepositoryService) ResetRepository(ctx context.Context, repoName string, since time.Time) error {
	return s.db.ResetRepository(ctx, repoName, since)
}
