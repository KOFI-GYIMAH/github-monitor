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

	"github.com/KOFI-GYIMAH/github-monitor/pkg/errors"
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
	rl := NewRateLimiter()

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: rl.Middleware(http.DefaultTransport),
	}

	return &Client{
		httpClient: client,
		token:      token,
	}
}

func (c *Client) makeRequest(ctx context.Context, method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	return resp, nil
}

func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s", owner, repo))
	if err != nil {
		return nil, errors.New(
			"GITHUB_API_ERROR",
			"Failed to fetch repository from GitHub",
			fmt.Sprintf("Could not retrieve repository %s/%s from GitHub API", owner, repo),
			err,
			errors.LevelError,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New(
			"REPOSITORY_NOT_FOUND",
			"Repository not found on GitHub",
			fmt.Sprintf("The repository %s/%s does not exist or you don't have access to it", owner, repo),
			nil,
			errors.LevelInfo,
		)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(
			"GITHUB_API_ERROR",
			"Unexpected response from GitHub API",
			fmt.Sprintf("GitHub API returned status %d when fetching repository %s/%s", resp.StatusCode, owner, repo),
			nil,
			errors.LevelError,
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New(
			"GITHUB_API_ERROR",
			"Failed to read GitHub API response",
			"Could not read the response body from GitHub API",
			err,
			errors.LevelError,
		)
	}

	var repository Repository
	if err := json.Unmarshal(body, &repository); err != nil {
		return nil, errors.New(
			"GITHUB_API_ERROR",
			"Failed to parse GitHub API response",
			"Could not understand the response from GitHub API",
			err,
			errors.LevelError,
		)
	}

	return &repository, nil
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
		return nil, errors.New(
			"GITHUB_API_ERROR",
			"Failed to fetch commits from GitHub",
			"Could not connect to GitHub API to retrieve commits",
			err,
			errors.LevelError,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(
			"GITHUB_API_ERROR",
			"Failed to fetch commits from GitHub",
			fmt.Sprintf("GitHub API returned unexpected status code %d when fetching commits", resp.StatusCode),
			nil,
			errors.LevelError,
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New(
			"GITHUB_API_ERROR",
			"Failed to read commits from GitHub",
			"Could not read the response body containing commits from GitHub API",
			err,
			errors.LevelError,
		)
	}

	var commits []*Commit
	if err := json.Unmarshal(body, &commits); err != nil {
		return nil, errors.New(
			"GITHUB_API_ERROR",
			"Failed to parse commits from GitHub",
			"Could not understand the commits data returned by GitHub API",
			err,
			errors.LevelError,
		)
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
			return nil, errors.New(
				"GITHUB_API_ERROR",
				"Failed to fetch commits from GitHub",
				fmt.Sprintf("Could not connect to GitHub API to retrieve page %d of commits", page),
				err,
				errors.LevelError,
			)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, errors.New(
				"GITHUB_API_ERROR",
				"Failed to fetch commits from GitHub",
				fmt.Sprintf("GitHub API returned unexpected status code %d when fetching page %d of commits", resp.StatusCode, page),
				nil,
				errors.LevelError,
			)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.New(
				"GITHUB_API_ERROR",
				"Failed to read commits from GitHub",
				fmt.Sprintf("Could not read the response body for page %d of commits", page),
				err,
				errors.LevelError,
			)
		}

		var commits []*Commit
		if err := json.Unmarshal(body, &commits); err != nil {
			return nil, errors.New(
				"GITHUB_API_ERROR",
				"Failed to parse commits from GitHub",
				fmt.Sprintf("Could not understand the commits data for page %d returned by GitHub API", page),
				err,
				errors.LevelError,
			)
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
	}

	logger.Info("Successfully fetched %d commits from GitHub", len(allCommits))
	return allCommits, nil
}
