package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/KOFI-GYIMAH/github-monitor/pkg/logger"
)

var (
	baseURL = "https://api.github.com"
)

type Client struct {
	httpClient *http.Client
	token      string
}

func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      token,
	}
}

func (c *Client) makeRequest(ctx context.Context, method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	return c.httpClient.Do(req)
}

type Repository struct {
	FullName        string    `json:"full_name"`
	Description     string    `json:"description"`
	HTMLURL         string    `json:"html_url"`
	Language        string    `json:"language"`
	ForksCount      int       `json:"forks_count"`
	StargazersCount int       `json:"stargazers_count"`
	OpenIssuesCount int       `json:"open_issues_count"`
	WatchersCount   int       `json:"watchers_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Commit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message string `json:"message"`
		Author  struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
}

func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s", owner, repo))
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var repository Repository
	if err := json.Unmarshal(body, &repository); err != nil {
		return nil, fmt.Errorf("failed to unmarshal repository: %w", err)
	}

	return &repository, nil
}

type CommitListOptions struct {
	Since   time.Time
	Page    int
	PerPage int
}

func (c *Client) ListCommits(ctx context.Context, owner, repo string, opts CommitListOptions) ([]*Commit, error) {
	path := fmt.Sprintf("/repos/%s/%s/commits", owner, repo)

	queryParams := make(url.Values)

	if !opts.Since.IsZero() {
		sinceUTC := opts.Since.UTC()
		sinceParam := sinceUTC.Format(time.RFC3339)
		queryParams.Add("since", sinceParam)
	}

	if opts.Page > 0 {
		return c.fetchSinglePage(ctx, path, queryParams)
	}
	return c.fetchAllPages(ctx, path, queryParams)
}

func (c *Client) fetchSinglePage(ctx context.Context, path string, queryParams url.Values) ([]*Commit, error) {
	if len(queryParams) > 0 {
		path += "?" + queryParams.Encode()
	}

	resp, err := c.makeRequest(ctx, "GET", path)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var commits []*Commit
	if err := json.Unmarshal(body, &commits); err != nil {
		return nil, fmt.Errorf("failed to unmarshal commits: %w", err)
	}

	return commits, nil
}

func (c *Client) fetchAllPages(ctx context.Context, path string, queryParams url.Values) ([]*Commit, error) {
	var allCommits []*Commit
	page := 1

	for {
		currentParams := make(url.Values)
		maps.Copy(currentParams, queryParams)
		currentParams.Set("page", strconv.Itoa(page))
		currentParams.Set("per_page", "100")

		currentPath := path + "?" + currentParams.Encode()

		resp, err := c.makeRequest(ctx, "GET", currentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to make request for page %d: %w", page, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code for page %d: %d", page, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body for page %d: %w", page, err)
		}

		var commits []*Commit
		if err := json.Unmarshal(body, &commits); err != nil {
			return nil, fmt.Errorf("failed to unmarshal commits for page %d: %w", page, err)
		}

		if len(commits) == 0 {
			break
		}

		allCommits = append(allCommits, commits...)
		page++

		linkHeader := resp.Header.Get("Link")
		if !strings.Contains(linkHeader, `rel="next"`) {
			break
		}

		logger.Info("Fetched page %d", page)
	}

	logger.Info("Fetched %d commits", len(allCommits))

	return allCommits, nil
}
