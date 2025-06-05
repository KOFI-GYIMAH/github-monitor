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
	t.Run("creates client with token", func(t *testing.T) {
		token := "test-token"
		client := NewClient(token)
		assert.Equal(t, token, client.token)
		assert.NotNil(t, client.httpClient)
		assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
		assert.Equal(t, baseURL, baseURL)
	})

	t.Run("creates client without token", func(t *testing.T) {
		client := NewClient("")
		assert.Empty(t, client.token)
		assert.Equal(t, baseURL, baseURL)
	})
}

func TestClient_GetRepository(t *testing.T) {
	testTime := time.Date(2024, time.June, 5, 0, 0, 0, 0, time.UTC)
	expectedRepo := &Repository{
		FullName:        "owner/repo",
		Description:     "Test repository",
		HTMLURL:         "https://github.com/owner/repo",
		Language:        "Go",
		ForksCount:      10,
		StargazersCount: 20,
		OpenIssuesCount: 3,
		WatchersCount:   15,
		CreatedAt:       testTime,
		UpdatedAt:       testTime,
	}

	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/repos/owner/repo", r.URL.Path)
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "application/vnd.github.v3+json", r.Header.Get("Accept"))

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(expectedRepo)
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.httpClient = server.Client()
		baseURL = server.URL

		repo, err := client.GetRepository(context.Background(), "owner", "repo")
		require.NoError(t, err)
		assert.Equal(t, expectedRepo, repo)
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.httpClient = server.Client()
		baseURL = server.URL

		repo, err := client.GetRepository(context.Background(), "owner", "repo")
		assert.Nil(t, repo)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code: 404")
	})

	t.Run("invalid response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.httpClient = server.Client()
		baseURL = server.URL

		repo, err := client.GetRepository(context.Background(), "owner", "repo")
		assert.Nil(t, repo)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal repository")
	})
}

func TestClient_ListCommits(t *testing.T) {
	testTime := time.Date(2024, time.June, 5, 0, 0, 0, 0, time.UTC)
	expectedCommit := &Commit{
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
				Date:  testTime,
			},
		},
		Author: struct {
			Login string `json:"login"`
		}{
			Login: "johndoe",
		},
	}

	t.Run("fetch single page", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/repos/owner/repo/commits", r.URL.Path)
			assert.Equal(t, "page=2&per_page=50", r.URL.RawQuery)
			assert.Equal(t, "GET", r.Method)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]*Commit{expectedCommit})
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.httpClient = server.Client()
		baseURL = server.URL

		commits, err := client.ListCommits(context.Background(), "owner", "repo", CommitListOptions{
			Page:    2,
			PerPage: 50,
		})
		require.NoError(t, err)
		assert.Equal(t, []*Commit{expectedCommit}, commits)
	})

	t.Run("fetch all pages", func(t *testing.T) {
		var callCount int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			var commits []*Commit
			if callCount == 1 {
				commits = []*Commit{expectedCommit}
				w.Header().Set("Link", `<`+r.URL.String()+`?page=2>; rel="next"`)
			} else {
				commits = []*Commit{
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
								Name:  "John Doe",
								Email: "john@example.com",
								Date:  testTime,
							},
						},
						Author: struct {
							Login string `json:"login"`
						}{
							Login: "johndoe",
						},
					},
				}
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(commits)
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.httpClient = server.Client()
		baseURL = server.URL

		commits, err := client.ListCommits(context.Background(), "owner", "repo", CommitListOptions{})
		require.NoError(t, err)
		assert.Len(t, commits, 2)
		assert.Equal(t, "abc123", commits[0].SHA)
		assert.Equal(t, "def456", commits[1].SHA)
		assert.Equal(t, 2, callCount)
	})

	t.Run("with since parameter", func(t *testing.T) {
		since := time.Now().Add(-7 * 24 * time.Hour)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, since.Format(time.RFC3339), r.URL.Query().Get("since"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]*Commit{expectedCommit})
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.httpClient = server.Client()
		baseURL = server.URL

		commits, err := client.ListCommits(context.Background(), "owner", "repo", CommitListOptions{
			Since: since,
		})
		require.NoError(t, err)
		assert.Equal(t, []*Commit{expectedCommit}, commits)
	})

	t.Run("empty response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[]"))
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.httpClient = server.Client()
		baseURL = server.URL

		commits, err := client.ListCommits(context.Background(), "owner", "repo", CommitListOptions{})
		require.NoError(t, err)
		assert.Empty(t, commits)
	})

	t.Run("error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.httpClient = server.Client()
		baseURL = server.URL

		commits, err := client.ListCommits(context.Background(), "owner", "repo", CommitListOptions{})
		assert.Nil(t, commits)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code for page 1: 500")
	})
}

func TestClient_makeRequest(t *testing.T) {
	t.Run("with token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.httpClient = server.Client()
		baseURL = server.URL

		resp, err := client.makeRequest(context.Background(), "GET", "/test")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("without token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClient("")
		client.httpClient = server.Client()
		baseURL = server.URL

		resp, err := client.makeRequest(context.Background(), "GET", "/test")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("invalid URL", func(t *testing.T) {
		originalBaseURL := baseURL
		defer func() { baseURL = originalBaseURL }()

		baseURL = "://invalid"

		client := NewClient("")
		resp, err := client.makeRequest(context.Background(), "GET", "/test")
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse")
	})

	t.Run("with context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.httpClient = server.Client()
		baseURL = server.URL

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		resp, err := client.makeRequest(ctx, "GET", "/test")
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}
