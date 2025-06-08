package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	token := "test-token"
	client := NewClient(token)

	assert.NotNil(t, client)
	assert.Equal(t, token, client.token)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
}

func TestClient_makeRequest(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectedError  bool
		validateReq    func(t *testing.T, r *http.Request)
	}{
		{
			name:  "successful request with token",
			token: "test-token",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"message": "success"}`))
			},
			validateReq: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
				assert.Equal(t, "application/vnd.github.v3+json", r.Header.Get("Accept"))
				assert.Equal(t, "GET", r.Method)
			},
		},
		{
			name:  "successful request without token",
			token: "",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			validateReq: func(t *testing.T, r *http.Request) {
				assert.Empty(t, r.Header.Get("Authorization"))
				assert.Equal(t, "application/vnd.github.v3+json", r.Header.Get("Accept"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.validateReq != nil {
					tt.validateReq(t, r)
				}
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client := NewClient(tt.token)
			originalBaseURL := baseURL
			baseURL = server.URL
			defer func() { baseURL = originalBaseURL }()

			resp, err := client.makeRequest(context.Background(), "GET", "/test")

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			resp.Body.Close()
		})
	}
}

func TestClient_GetRepository(t *testing.T) {
	tests := []struct {
		name           string
		owner          string
		repo           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectedRepo   *Repository
		expectedError  string
		errorCode      string
	}{
		{
			name:  "successful repository fetch",
			owner: "testowner",
			repo:  "testrepo",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/repos/testowner/testrepo", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				repo := Repository{
					FullName:        "testowner/testrepo",
					Description:     "Test repository",
					HTMLURL:         "https://github.com/testowner/testrepo",
					Language:        "Go",
					ForksCount:      5,
					StargazersCount: 10,
					OpenIssuesCount: 2,
					WatchersCount:   8,
					CreatedAt:       time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					UpdatedAt:       time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC),
				}
				json.NewEncoder(w).Encode(repo)
			},
			expectedRepo: &Repository{
				FullName:        "testowner/testrepo",
				Description:     "Test repository",
				HTMLURL:         "https://github.com/testowner/testrepo",
				Language:        "Go",
				ForksCount:      5,
				StargazersCount: 10,
				OpenIssuesCount: 2,
				WatchersCount:   8,
				CreatedAt:       time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedAt:       time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:  "repository not found",
			owner: "nonexistent",
			repo:  "repo",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectedError: "Repository not found on GitHub",
			errorCode:     "REPOSITORY_NOT_FOUND",
		},
		{
			name:  "server error",
			owner: "testowner",
			repo:  "testrepo",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedError: "Unexpected response from GitHub API",
			errorCode:     "GITHUB_API_ERROR",
		},
		{
			name:  "invalid json response",
			owner: "testowner",
			repo:  "testrepo",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json`))
			},
			expectedError: "Failed to parse GitHub API response",
			errorCode:     "GITHUB_API_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			client := NewClient("test-token")
			originalBaseURL := baseURL
			baseURL = server.URL
			defer func() { baseURL = originalBaseURL }()

			repo, err := client.GetRepository(context.Background(), tt.owner, tt.repo)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, repo)
			} else {
				require.NoError(t, err)
				require.NotNil(t, repo)
				assert.Equal(t, tt.expectedRepo, repo)
			}
		})
	}
}

func TestClient_ListCommits(t *testing.T) {
	mockCommits := []*Commit{
		{
			SHA:     "abc123",
			HTMLURL: "https://github.com/owner/repo/commit/abc123",
			Commit: struct {
				Message string `json:"message"`
				Author  struct {
					Name  string    `json:"name"`
					Email string    `json:"email"`
					Date  time.Time `json:"date"`
				} `json:"author"`
			}{
				Message: "Initial commit",
				Author: struct {
					Name  string    `json:"name"`
					Email string    `json:"email"`
					Date  time.Time `json:"date"`
				}{
					Name:  "John Doe",
					Email: "john@example.com",
					Date:  time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				},
			},
			Author: struct {
				Login string `json:"login"`
			}{
				Login: "johndoe",
			},
		},
		{
			SHA:     "def456",
			HTMLURL: "https://github.com/owner/repo/commit/def456",
			Commit: struct {
				Message string `json:"message"`
				Author  struct {
					Name  string    `json:"name"`
					Email string    `json:"email"`
					Date  time.Time `json:"date"`
				} `json:"author"`
			}{
				Message: "Second commit",
				Author: struct {
					Name  string    `json:"name"`
					Email string    `json:"email"`
					Date  time.Time `json:"date"`
				}{
					Name:  "Jane Smith",
					Email: "jane@example.com",
					Date:  time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC),
				},
			},
			Author: struct {
				Login string `json:"login"`
			}{
				Login: "janesmith",
			},
		},
	}

	tests := []struct {
		name            string
		owner           string
		repo            string
		opts            CommitListOptions
		serverResponse  func(w http.ResponseWriter, r *http.Request)
		expectedCommits []*Commit
		expectedError   string
		validateRequest func(t *testing.T, r *http.Request)
	}{
		{
			name:  "fetch single page with no options",
			owner: "testowner",
			repo:  "testrepo",
			opts:  CommitListOptions{},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(mockCommits)
			},
			expectedCommits: mockCommits,
			validateRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/repos/testowner/testrepo/commits", r.URL.Path)
				assert.Contains(t, r.URL.RawQuery, "page=1")
				assert.Contains(t, r.URL.RawQuery, "per_page=100")
			},
		},
		{
			name:  "fetch single page with specific page number",
			owner: "testowner",
			repo:  "testrepo",
			opts:  CommitListOptions{Page: 2},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(mockCommits[:1])
			},
			expectedCommits: mockCommits[:1],
			validateRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/repos/testowner/testrepo/commits", r.URL.Path)
				assert.NotContains(t, r.URL.RawQuery, "page=")
			},
		},
		{
			name:  "fetch with since parameter",
			owner: "testowner",
			repo:  "testrepo",
			opts: CommitListOptions{
				Since: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(mockCommits[1:])
			},
			expectedCommits: mockCommits[1:],
			validateRequest: func(t *testing.T, r *http.Request) {
				assert.Contains(t, r.URL.RawQuery, "since=2023-01-01T00%3A00%3A00Z")
			},
		},
		{
			name:  "server error",
			owner: "testowner",
			repo:  "testrepo",
			opts:  CommitListOptions{},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedError: "Failed to fetch commits from GitHub",
		},
		{
			name:  "invalid json response",
			owner: "testowner",
			repo:  "testrepo",
			opts:  CommitListOptions{},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json`))
			},
			expectedError: "Failed to parse commits from GitHub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.validateRequest != nil {
					tt.validateRequest(t, r)
				}
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			client := NewClient("test-token")
			originalBaseURL := baseURL
			baseURL = server.URL
			defer func() { baseURL = originalBaseURL }()

			commits, err := client.ListCommits(context.Background(), tt.owner, tt.repo, tt.opts)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, commits)
			} else {
				require.NoError(t, err)
				require.NotNil(t, commits)
				assert.Equal(t, len(tt.expectedCommits), len(commits))
				for i, expectedCommit := range tt.expectedCommits {
					assert.Equal(t, expectedCommit.SHA, commits[i].SHA)
					assert.Equal(t, expectedCommit.HTMLURL, commits[i].HTMLURL)
					assert.Equal(t, expectedCommit.Commit.Message, commits[i].Commit.Message)
					assert.Equal(t, expectedCommit.Author.Login, commits[i].Author.Login)
				}
			}
		})
	}
}

func TestClient_fetchAllPages(t *testing.T) {
	mockCommitsPage1 := []*Commit{
		{SHA: "commit1", HTMLURL: "https://github.com/owner/repo/commit/commit1"},
		{SHA: "commit2", HTMLURL: "https://github.com/owner/repo/commit/commit2"},
	}
	mockCommitsPage2 := []*Commit{
		{SHA: "commit3", HTMLURL: "https://github.com/owner/repo/commit/commit3"},
	}

	pageCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++

		if pageCount == 1 {
			w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/commits?page=2>; rel="next"`)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(mockCommitsPage1)
		} else if pageCount == 2 {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(mockCommitsPage2)
		} else {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]*Commit{})
		}
	}))
	defer server.Close()

	client := NewClient("test-token")
	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	commits, err := client.ListCommits(context.Background(), "owner", "repo", CommitListOptions{})

	require.NoError(t, err)
	require.NotNil(t, commits)
	assert.Equal(t, 3, len(commits))
	assert.Equal(t, "commit1", commits[0].SHA)
	assert.Equal(t, "commit2", commits[1].SHA)
	assert.Equal(t, "commit3", commits[2].SHA)
	assert.Equal(t, 2, pageCount)
}

func TestClient_fetchAllPages_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]*Commit{})
	}))
	defer server.Close()

	client := NewClient("test-token")
	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	commits, err := client.ListCommits(context.Background(), "owner", "repo", CommitListOptions{})

	require.NoError(t, err)
	assert.Equal(t, 0, len(commits))
}

func TestClient_Context_Cancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Repository{})
	}))
	defer server.Close()

	client := NewClient("test-token")
	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetRepository(ctx, "owner", "repo")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func BenchmarkClient_GetRepository(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		repo := Repository{
			FullName:        "testowner/testrepo",
			Description:     "Test repository",
			HTMLURL:         "https://github.com/testowner/testrepo",
			Language:        "Go",
			ForksCount:      5,
			StargazersCount: 10,
		}
		json.NewEncoder(w).Encode(repo)
	}))
	defer server.Close()

	client := NewClient("test-token")
	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetRepository(context.Background(), "testowner", "testrepo")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClient_ListCommits(b *testing.B) {
	mockCommits := make([]*Commit, 10)
	for i := range 10 {
		mockCommits[i] = &Commit{
			SHA:     "commit" + string(rune(i)),
			HTMLURL: "https://github.com/owner/repo/commit/commit" + string(rune(i)),
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockCommits)
	}))
	defer server.Close()

	client := NewClient("test-token")
	originalBaseURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalBaseURL }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.ListCommits(context.Background(), "owner", "repo", CommitListOptions{Page: 1})
		if err != nil {
			b.Fatal(err)
		}
	}
}
