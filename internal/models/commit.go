package models

import "time"

// * GitHub commit
type Commit struct {
	ID           int       `json:"id"`
	SHA          string    `json:"sha"`
	RepositoryID int       `json:"repository_id"`
	Message      string    `json:"message"`
	AuthorName   string    `json:"author_name"`
	AuthorEmail  string    `json:"author_email"`
	AuthorDate   time.Time `json:"author_date"`
	CommitURL    string    `json:"commit_url"`
}

type AuthorCommitCount struct {
	AuthorName  string `json:"author_name"`
	CommitCount int    `json:"commit_count"`
}
