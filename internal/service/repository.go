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

type GitHubClientInterface interface {
	GetRepository(ctx context.Context, owner, name string) (*github.Repository, error)
	ListCommits(ctx context.Context, owner, name string, opts github.CommitListOptions) ([]*github.Commit, error)
}

type RepositoryService struct {
	githubClient GitHubClientInterface
	db           models.Database
}

func NewRepositoryService(githubClient GitHubClientInterface, db models.Database) *RepositoryService {
	return &RepositoryService{
		githubClient: githubClient,
		db:           db,
	}
}

func (s *RepositoryService) GetRepository(ctx context.Context, name string) (*models.Repository, error) {
	return s.db.GetRepository(ctx, name)
}

func (s *RepositoryService) SyncRepository(ctx context.Context, owner, name string, since time.Time) error {
	logger.Info("Syncing repository... %s", name)

	return s.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		repo, err := s.githubClient.GetRepository(ctx, owner, name)
		if err != nil {
			return err
		}

		logger.Info("Successfully fetched repository %s", repo.FullName)

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
			return err
		}

		logger.Info("Successfully saved repository %s", repo.FullName)

		// * Get commits since last sync
		commitOpts := github.CommitListOptions{Since: since}
		commits, err := s.githubClient.ListCommits(ctx, owner, name, commitOpts)
		if err != nil {
			return fmt.Errorf("failed to list commits for %s: %w", repo.FullName, err)
		}

		logger.Info("Successfully fetched %d commits for %s", len(commits), repo.FullName)

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
				return fmt.Errorf("failed to insert commit for %s: %w", repo.FullName, err)
			}

			logger.Info("Successfully saved commit %s for %s", commit.SHA, repo.FullName)
		}

		logger.Info("Successfully synced repository %s with %d commits", repo.FullName, len(commits))

		// * Update last sync time
		now := time.Now()
		dbRepo.LastCommitFetchedAt = &now
		return s.db.UpdateRepositoryTx(ctx, tx, &dbRepo)
	})
}

func (s *RepositoryService) ListAllRepositories(ctx context.Context) ([]*models.Repository, error) {
	return s.db.GetAllRepositories(ctx)
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
