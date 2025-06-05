package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/KOFI-GYIMAH/github-monitor/internal/models"
	"github.com/KOFI-GYIMAH/github-monitor/pkg/logger"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

type PostgresDB struct {
	db *sql.DB
}

func NewPostgresDB(url string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// * Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// * Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("connected to database successfully 🎉")
	return &PostgresDB{db: db}, nil
}

func (p *PostgresDB) Migrate() error {
	driver, err := postgres.WithInstance(p.db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migrate driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func (p *PostgresDB) Close() error {
	return p.db.Close()
}

func (p *PostgresDB) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %v, rollback error: %w", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}

func (p *PostgresDB) UpsertRepository(ctx context.Context, repo *models.Repository) error {
	query := `
		INSERT INTO repositories (
			name, description, url, language, forks_count, stars_count, 
			open_issues_count, watchers_count, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT(name) DO UPDATE SET
			description = EXCLUDED.description,
			url = EXCLUDED.url,
			language = EXCLUDED.language,
			forks_count = EXCLUDED.forks_count,
			stars_count = EXCLUDED.stars_count,
			open_issues_count = EXCLUDED.open_issues_count,
			watchers_count = EXCLUDED.watchers_count,
			updated_at = EXCLUDED.updated_at
		RETURNING id, last_commit_fetched_at
	`

	row := p.db.QueryRowContext(ctx, query,
		repo.Name, repo.Description, repo.URL, repo.Language, repo.ForksCount,
		repo.StarsCount, repo.OpenIssuesCount, repo.WatchersCount,
		repo.CreatedAt, repo.UpdatedAt,
	)

	var lastFetched sql.NullTime
	err := row.Scan(&repo.ID, &lastFetched)
	if err != nil {
		return fmt.Errorf("failed to upsert repository: %w", err)
	}

	if lastFetched.Valid {
		repo.LastCommitFetchedAt = &lastFetched.Time
	}

	return nil
}

func (p *PostgresDB) GetRepository(ctx context.Context, name string) (*models.Repository, error) {
	query := `
		SELECT id, name, description, url, language, forks_count, stars_count,
		open_issues_count, watchers_count, created_at, updated_at, last_commit_fetched_at
		FROM repositories
		WHERE name = $1
	`

	row := p.db.QueryRowContext(ctx, query, name)

	var repo models.Repository
	var lastFetched sql.NullTime

	err := row.Scan(
		&repo.ID, &repo.Name, &repo.Description, &repo.URL, &repo.Language,
		&repo.ForksCount, &repo.StarsCount, &repo.OpenIssuesCount, &repo.WatchersCount,
		&repo.CreatedAt, &repo.UpdatedAt, &lastFetched,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	if lastFetched.Valid {
		repo.LastCommitFetchedAt = &lastFetched.Time
	}

	return &repo, nil
}

func (p *PostgresDB) UpdateRepository(ctx context.Context, repo *models.Repository) error {
	query := `
		UPDATE repositories
		SET last_commit_fetched_at = $1
		WHERE id = $2
	`

	_, err := p.db.ExecContext(ctx, query, repo.LastCommitFetchedAt, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}

	return nil
}

func (p *PostgresDB) InsertCommit(ctx context.Context, commit *models.Commit) error {
	query := `
		INSERT INTO commits (
			sha, repository_id, message, author_name, author_email, author_date, commit_url
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT(sha, repository_id) DO NOTHING
	`

	_, err := p.db.ExecContext(ctx, query,
		commit.SHA, commit.RepositoryID, commit.Message, commit.AuthorName,
		commit.AuthorEmail, commit.AuthorDate, commit.CommitURL,
	)

	if err != nil {
		return fmt.Errorf("failed to insert commit: %w", err)
	}

	return nil
}

func (p *PostgresDB) GetCommits(ctx context.Context, repoName string, since, until *time.Time) ([]models.Commit, error) {
	query := `
		SELECT c.sha, c.repository_id, c.message, c.author_name, c.author_email, 
					c.author_date, c.commit_url
		FROM commits c
		JOIN repositories r ON c.repository_id = r.id
		WHERE r.name = $1
	`

	args := []interface{}{repoName}
	paramCount := 1

	if since != nil {
		paramCount++
		query += fmt.Sprintf(" AND c.author_date >= $%d", paramCount)
		args = append(args, *since)
	}

	if until != nil {
		paramCount++
		query += fmt.Sprintf(" AND c.author_date <= $%d", paramCount)
		args = append(args, *until)
	}

	query += " ORDER BY c.author_date DESC"

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query commits: %w", err)
	}
	defer rows.Close()

	var commits []models.Commit
	for rows.Next() {
		var c models.Commit
		err := rows.Scan(
			&c.SHA, &c.RepositoryID, &c.Message, &c.AuthorName,
			&c.AuthorEmail, &c.AuthorDate, &c.CommitURL,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan commit: %w", err)
		}
		commits = append(commits, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return commits, nil
}

func (p *PostgresDB) GetTopAuthors(ctx context.Context, repoName string, limit int) ([]models.AuthorCommitCount, error) {
	query := `
		SELECT c.author_name, COUNT(*) as commit_count 
		FROM commits c
		JOIN repositories r ON c.repository_id = r.id 
		WHERE r.name = $1 
		GROUP BY c.author_name 
		ORDER BY commit_count DESC 
		LIMIT $2
	`

	rows, err := p.db.QueryContext(ctx, query, repoName, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top authors: %w", err)
	}
	defer rows.Close()

	var results []models.AuthorCommitCount
	for rows.Next() {
		var acc models.AuthorCommitCount
		err := rows.Scan(&acc.AuthorName, &acc.CommitCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan author commit count: %w", err)
		}
		results = append(results, acc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return results, nil
}

func (p *PostgresDB) ResetRepository(ctx context.Context, repoName string, since time.Time) error {
	// * Delete all commits for the repository
	_, err := p.db.ExecContext(ctx, `
		DELETE FROM commits 
		WHERE repository_id = (
			SELECT id FROM repositories WHERE name = $1
		)
	`, repoName)
	if err != nil {
		return fmt.Errorf("failed to delete commits: %w", err)
	}

	// * update the last fetched time
	_, err = p.db.ExecContext(ctx, `
		UPDATE repositories 
		SET last_commit_fetched_at = $1 
		WHERE name = $2
	`, since, repoName)
	if err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}

	return nil
}

// Transaction versions of methods for use with WithTransaction
func (p *PostgresDB) UpsertRepositoryTx(ctx context.Context, tx *sql.Tx, repo *models.Repository) error {
	query := `
		INSERT INTO repositories (
			name, description, url, language, forks_count, stars_count, 
			open_issues_count, watchers_count, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT(name) DO UPDATE SET
			description = EXCLUDED.description,
			url = EXCLUDED.url,
			language = EXCLUDED.language,
			forks_count = EXCLUDED.forks_count,
			stars_count = EXCLUDED.stars_count,
			open_issues_count = EXCLUDED.open_issues_count,
			watchers_count = EXCLUDED.watchers_count,
			updated_at = EXCLUDED.updated_at
		RETURNING id, last_commit_fetched_at
	`

	row := tx.QueryRowContext(ctx, query,
		repo.Name, repo.Description, repo.URL, repo.Language, repo.ForksCount,
		repo.StarsCount, repo.OpenIssuesCount, repo.WatchersCount,
		repo.CreatedAt, repo.UpdatedAt,
	)

	var lastFetched sql.NullTime
	err := row.Scan(&repo.ID, &lastFetched)
	if err != nil {
		return fmt.Errorf("failed to upsert repository: %w", err)
	}

	if lastFetched.Valid {
		repo.LastCommitFetchedAt = &lastFetched.Time
	}

	return nil
}

func (p *PostgresDB) InsertCommitTx(ctx context.Context, tx *sql.Tx, commit *models.Commit) error {
	query := `
		INSERT INTO commits (
			sha, repository_id, message, author_name, author_email, author_date, commit_url
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT(sha, repository_id) DO NOTHING
	`

	_, err := tx.ExecContext(ctx, query,
		commit.SHA, commit.RepositoryID, commit.Message, commit.AuthorName,
		commit.AuthorEmail, commit.AuthorDate, commit.CommitURL,
	)

	if err != nil {
		return fmt.Errorf("failed to insert commit: %w", err)
	}

	return nil
}

func (p *PostgresDB) UpdateRepositoryTx(ctx context.Context, tx *sql.Tx, repo *models.Repository) error {
	query := `
		UPDATE repositories
		SET last_commit_fetched_at = $1
		WHERE id = $2
	`

	_, err := tx.ExecContext(ctx, query, repo.LastCommitFetchedAt, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}

	return nil
}
