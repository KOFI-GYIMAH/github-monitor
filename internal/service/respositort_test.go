package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/KOFI-GYIMAH/github-monitor/internal/github"
	"github.com/KOFI-GYIMAH/github-monitor/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockGitHubClient struct {
	mock.Mock
}

func (m *MockGitHubClient) GetRepository(ctx context.Context, owner, name string) (*github.Repository, error) {
	args := m.Called(ctx, owner, name)
	return args.Get(0).(*github.Repository), args.Error(1)
}

func (m *MockGitHubClient) ListCommits(ctx context.Context, owner, name string, opts github.CommitListOptions) ([]*github.Commit, error) {
	args := m.Called(ctx, owner, name, opts)
	return args.Get(0).([]*github.Commit), args.Error(1)
}

type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) UpsertRepository(ctx context.Context, repo *models.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockDatabase) GetRepository(ctx context.Context, name string) (*models.Repository, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Repository), args.Error(1)
}

func (m *MockDatabase) GetAllRepositories(ctx context.Context) ([]*models.Repository, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*models.Repository), args.Error(1)
}

func (m *MockDatabase) UpdateRepository(ctx context.Context, repo *models.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockDatabase) ResetRepository(ctx context.Context, repoName string, since time.Time) error {
	args := m.Called(ctx, repoName, since)
	return args.Error(0)
}

func (m *MockDatabase) InsertCommit(ctx context.Context, commit *models.Commit) error {
	args := m.Called(ctx, commit)
	return args.Error(0)
}

func (m *MockDatabase) GetCommits(ctx context.Context, repoName string, since, until *time.Time) ([]models.Commit, error) {
	args := m.Called(ctx, repoName, since, until)
	return args.Get(0).([]models.Commit), args.Error(1)
}

func (m *MockDatabase) GetTopAuthors(ctx context.Context, repoName string, limit int) ([]models.AuthorCommitCount, error) {
	args := m.Called(ctx, repoName, limit)
	return args.Get(0).([]models.AuthorCommitCount), args.Error(1)
}

func (m *MockDatabase) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	args := m.Called(ctx, fn)
	if err := fn(&sql.Tx{}); err != nil {
		return err
	}
	return args.Error(0)
}

func (m *MockDatabase) UpsertRepositoryTx(ctx context.Context, tx *sql.Tx, repo *models.Repository) error {
	args := m.Called(ctx, tx, repo)
	return args.Error(0)
}

func (m *MockDatabase) InsertCommitTx(ctx context.Context, tx *sql.Tx, commit *models.Commit) error {
	args := m.Called(ctx, tx, commit)
	return args.Error(0)
}

func (m *MockDatabase) UpdateRepositoryTx(ctx context.Context, tx *sql.Tx, repo *models.Repository) error {
	args := m.Called(ctx, tx, repo)
	return args.Error(0)
}

func TestNewRepositoryService(t *testing.T) {
	mockGitHubClient := new(MockGitHubClient)
	mockDB := new(MockDatabase)

	service := NewRepositoryService(mockGitHubClient, mockDB)

	assert.NotNil(t, service)
	// assert.Equal(t, mockGitHubClient, service.GitHubClient)
	// assert.Equal(t, mockDB, service.DB)
}

func TestGetRepository(t *testing.T) {
	tests := []struct {
		name        string
		repoName    string
		mockRepo    *models.Repository
		mockError   error
		expectError bool
	}{
		{
			name:     "success",
			repoName: "owner/repo",
			mockRepo: &models.Repository{
				Name: "owner/repo",
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "not found",
			repoName:    "owner/repo",
			mockRepo:    nil,
			mockError:   sql.ErrNoRows,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGitHubClient := new(MockGitHubClient)
			mockDB := new(MockDatabase)
			service := NewRepositoryService(mockGitHubClient, mockDB)

			mockDB.On("GetRepository", mock.Anything, tt.repoName).Return(tt.mockRepo, tt.mockError)

			repo, err := service.GetRepository(context.Background(), tt.repoName)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, repo)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockRepo, repo)
			}

			mockDB.AssertExpectations(t)
		})
	}
}

func TestSyncRepository(t *testing.T) {
	now := time.Now()
	testRepo := &github.Repository{
		FullName:        "owner/repo",
		Description:     "test repo",
		HTMLURL:         "http://github.com/owner/repo",
		Language:        "Go",
		ForksCount:      10,
		StargazersCount: 20,
		OpenIssuesCount: 5,
		WatchersCount:   15,
		CreatedAt:       now.Add(-24 * time.Hour),
		UpdatedAt:       now,
	}

	testCommits := []*github.Commit{
		{
			SHA: "abc123",
			Commit: struct {
				Message string `json:"message"`
				Author  struct {
					Name  string    `json:"name"`
					Email string    `json:"email"`
					Date  time.Time `json:"date"`
				} `json:"author"`
			}{
				Message: "test commit",
				Author: struct {
					Name  string    `json:"name"`
					Email string    `json:"email"`
					Date  time.Time `json:"date"`
				}{
					Email: "test@example.com",
					Date:  now,
				},
			},
			Author: struct {
				Login string `json:"login"`
			}{
				Login: "testuser",
			},
			HTMLURL: "http://github.com/owner/repo/commit/abc123",
		},
	}

	tests := []struct {
		name         string
		owner        string
		repoName     string
		since        time.Time
		mockRepo     *github.Repository
		repoError    error
		mockCommits  []*github.Commit
		commitsError error
		expectError  bool
	}{
		{
			name:         "successful sync",
			owner:        "owner",
			repoName:     "repo",
			since:        now.Add(-1 * time.Hour),
			mockRepo:     testRepo,
			repoError:    nil,
			mockCommits:  testCommits,
			commitsError: nil,
			expectError:  false,
		},
		{
			name:         "github repo error",
			owner:        "owner",
			repoName:     "repo",
			since:        now.Add(-1 * time.Hour),
			mockRepo:     nil,
			repoError:    errors.New("github error"),
			mockCommits:  nil,
			commitsError: nil,
			expectError:  true,
		},
		{
			name:         "github commits error",
			owner:        "owner",
			repoName:     "repo",
			since:        now.Add(-1 * time.Hour),
			mockRepo:     testRepo,
			repoError:    nil,
			mockCommits:  nil,
			commitsError: errors.New("github error"),
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGitHubClient := new(MockGitHubClient)
			mockDB := new(MockDatabase)
			service := NewRepositoryService(mockGitHubClient, mockDB)

			mockGitHubClient.On("GetRepository", mock.Anything, tt.owner, tt.repoName).Return(tt.mockRepo, tt.repoError)

			mockDB.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(*sql.Tx) error)
				fn(&sql.Tx{})
			})

			if tt.repoError == nil {
				mockGitHubClient.On("ListCommits", mock.Anything, tt.owner, tt.repoName, github.CommitListOptions{Since: tt.since}).Return(tt.mockCommits, tt.commitsError)

				mockDB.On("UpsertRepositoryTx", mock.Anything, mock.Anything, mock.AnythingOfType("*models.Repository")).Return(nil)

				if tt.commitsError == nil && tt.mockCommits != nil {
					mockDB.On("InsertCommitTx", mock.Anything, mock.Anything, mock.AnythingOfType("*models.Commit")).Return(nil)
					mockDB.On("UpdateRepositoryTx", mock.Anything, mock.Anything, mock.AnythingOfType("*models.Repository")).Return(nil)
				}
			}

			err := service.SyncRepository(context.Background(), tt.owner, tt.repoName, tt.since)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockGitHubClient.AssertExpectations(t)
			mockDB.AssertExpectations(t)
		})
	}
}

func TestListAllRepositories(t *testing.T) {
	mockRepos := []*models.Repository{
		{Name: "repo1"},
		{Name: "repo2"},
	}

	tests := []struct {
		name        string
		mockRepos   []*models.Repository
		mockError   error
		expectError bool
	}{
		{
			name:        "success",
			mockRepos:   mockRepos,
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "database error",
			mockRepos:   nil,
			mockError:   errors.New("db error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGitHubClient := new(MockGitHubClient)
			mockDB := new(MockDatabase)
			service := NewRepositoryService(mockGitHubClient, mockDB)

			mockDB.On("GetAllRepositories", mock.Anything).Return(tt.mockRepos, tt.mockError)

			repos, err := service.ListAllRepositories(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, repos)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockRepos, repos)
			}

			mockDB.AssertExpectations(t)
		})
	}
}

func TestGetTopAuthors(t *testing.T) {
	mockAuthors := []models.AuthorCommitCount{
		{AuthorName: "user1", CommitCount: 10},
		{AuthorName: "user2", CommitCount: 5},
	}

	tests := []struct {
		name        string
		repoName    string
		limit       int
		mockAuthors []models.AuthorCommitCount
		mockError   error
		expectError bool
	}{
		{
			name:        "success",
			repoName:    "owner/repo",
			limit:       5,
			mockAuthors: mockAuthors,
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "database error",
			repoName:    "owner/repo",
			limit:       5,
			mockAuthors: nil,
			mockError:   errors.New("db error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGitHubClient := new(MockGitHubClient)
			mockDB := new(MockDatabase)
			service := NewRepositoryService(mockGitHubClient, mockDB)

			mockDB.On("GetTopAuthors", mock.Anything, tt.repoName, tt.limit).Return(tt.mockAuthors, tt.mockError)

			authors, err := service.GetTopAuthors(context.Background(), tt.repoName, tt.limit)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, authors)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockAuthors, authors)
			}

			mockDB.AssertExpectations(t)
		})
	}
}

func TestGetCommits(t *testing.T) {
	now := time.Now()
	mockCommits := []models.Commit{
		{SHA: "abc123", Message: "commit 1"},
		{SHA: "def456", Message: "commit 2"},
	}

	tests := []struct {
		name        string
		repoName    string
		since       *time.Time
		until       *time.Time
		mockCommits []models.Commit
		mockError   error
		expectError bool
	}{
		{
			name:        "success with time range",
			repoName:    "owner/repo",
			since:       &now,
			until:       &now,
			mockCommits: mockCommits,
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "success without time range",
			repoName:    "owner/repo",
			since:       nil,
			until:       nil,
			mockCommits: mockCommits,
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "database error",
			repoName:    "owner/repo",
			since:       nil,
			until:       nil,
			mockCommits: nil,
			mockError:   errors.New("db error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGitHubClient := new(MockGitHubClient)
			mockDB := new(MockDatabase)
			service := NewRepositoryService(mockGitHubClient, mockDB)

			mockDB.On("GetCommits", mock.Anything, tt.repoName, tt.since, tt.until).Return(tt.mockCommits, tt.mockError)

			commits, err := service.GetCommits(context.Background(), tt.repoName, tt.since, tt.until)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, commits)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockCommits, commits)
			}

			mockDB.AssertExpectations(t)
		})
	}
}

func TestResetRepository(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		repoName    string
		since       time.Time
		mockError   error
		expectError bool
	}{
		{
			name:        "success",
			repoName:    "owner/repo",
			since:       now,
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "database error",
			repoName:    "owner/repo",
			since:       now,
			mockError:   errors.New("db error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGitHubClient := new(MockGitHubClient)
			mockDB := new(MockDatabase)
			service := NewRepositoryService(mockGitHubClient, mockDB)

			mockDB.On("ResetRepository", mock.Anything, tt.repoName, tt.since).Return(tt.mockError)

			err := service.ResetRepository(context.Background(), tt.repoName, tt.since)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockDB.AssertExpectations(t)
		})
	}
}
