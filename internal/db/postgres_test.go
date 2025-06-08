package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/KOFI-GYIMAH/github-monitor/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestUpsertRepository(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	repo := &models.Repository{
		Name:            "test/repo",
		Description:     "Test Repo",
		URL:             "https://github.com/test/repo",
		Language:        "Go",
		ForksCount:      10,
		StarsCount:      50,
		OpenIssuesCount: 3,
		WatchersCount:   20,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	rows := sqlmock.NewRows([]string{"id", "last_commit_fetched_at"}).
		AddRow(1, nil)

	mock.ExpectQuery("INSERT INTO repositories").
		WithArgs(
			repo.Name, repo.Description, repo.URL, repo.Language,
			repo.ForksCount, repo.StarsCount, repo.OpenIssuesCount,
			repo.WatchersCount, repo.CreatedAt, repo.UpdatedAt,
		).WillReturnRows(rows)

	pg := &PostgresDB{db: mockDB}
	err = pg.UpsertRepository(context.Background(), repo)
	assert.NoError(t, err)
	assert.Equal(t, 1, repo.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetRepository(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "name", "description", "url", "language", "forks_count", "stars_count",
		"open_issues_count", "watchers_count", "created_at", "updated_at", "last_commit_fetched_at",
	}).AddRow(1, "test/repo", "desc", "url", "Go", 1, 2, 3, 4, now, now, nil)

	mock.ExpectQuery("SELECT id, name, description, url, language").
		WithArgs("test/repo").
		WillReturnRows(rows)

	pg := &PostgresDB{db: mockDB}
	repo, err := pg.GetRepository(context.Background(), "test/repo")
	assert.NoError(t, err)
	assert.Equal(t, "test/repo", repo.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertCommit(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	commit := &models.Commit{
		SHA:          "abc123",
		RepositoryID: 1,
		Message:      "initial commit",
		AuthorName:   "test",
		AuthorEmail:  "test@example.com",
		AuthorDate:   time.Now(),
		CommitURL:    "https://github.com/commit/abc123",
	}

	mock.ExpectExec("INSERT INTO commits").
		WithArgs(commit.SHA, commit.RepositoryID, commit.Message, commit.AuthorName,
			commit.AuthorEmail, commit.AuthorDate, commit.CommitURL).
		WillReturnResult(sqlmock.NewResult(1, 1))

	pg := &PostgresDB{db: mockDB}
	err = pg.InsertCommit(context.Background(), commit)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTransaction_Commit(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	mock.ExpectBegin()
	mock.ExpectCommit()

	pg := &PostgresDB{db: mockDB}
	err = pg.WithTransaction(context.Background(), func(tx *sql.Tx) error {
		return nil
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTransaction_Rollback(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()

	pg := &PostgresDB{db: mockDB}
	err = pg.WithTransaction(context.Background(), func(tx *sql.Tx) error {
		return assert.AnError
	})
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
