package models

import (
	"context"
	"database/sql"
	"time"
)

// * This interface defines all db operations needed by the application
type Database interface {
	// * Repository operations
	UpsertRepository(ctx context.Context, repo *Repository) error
	GetRepository(ctx context.Context, name string) (*Repository, error)
	UpdateRepository(ctx context.Context, repo *Repository) error
	ResetRepository(ctx context.Context, repoName string, since time.Time) error

	// * Commit operations
	InsertCommit(ctx context.Context, commit *Commit) error
	GetCommits(ctx context.Context, repoName string, since, until *time.Time) ([]Commit, error)
	GetTopAuthors(ctx context.Context, repoName string, limit int) ([]AuthorCommitCount, error)

	// * Transaction support
	WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error
	UpsertRepositoryTx(ctx context.Context, tx *sql.Tx, repo *Repository) error
	InsertCommitTx(ctx context.Context, tx *sql.Tx, commit *Commit) error
	UpdateRepositoryTx(ctx context.Context, tx *sql.Tx, repo *Repository) error
}
