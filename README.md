# GitHub Repository Monitoring Service

A Go service that monitors GitHub repositories, tracks commits, and stores data in a persistent database.

## Features

- 📦 Fetches repository metadata and commit history from GitHub API  
- 🗄️ Stores data in PostgreSQL with efficient querying capabilities  
- 🔄 Continuous monitoring with configurable intervals
- 📡 REST API for data access  
- ⚙️ Configurable through environment variables  

## Prerequisites

- Go 1.24+
- GitHub Personal Access Token

## Quick Start

### Clone the repository

- git clone <https://github.com/KOFI-GYIMAH/github-monitor.git>
- cd github-monitor
- go mod tidy

### Create .env file and edit

- cp .env.example .env
- edit .env with your GitHub token and other config values

### Run the application

- go run cmd/server/main.go

### 📘 API Endpoints

Access the full API documentation at:  
🔗 [http://localhost:8081/v1/swagger/index.html](http://localhost:8081/v1/swagger/index.html)

---

## 📂 Repository Information

### 🔹 Get Repository Metadata

**GET** `/v1/repositories/{owner}/{repo}`  
→ Retrieves metadata for a specific repository.

---

### 🔹 Reset Repository Data Collection

**POST** `/v1/repositories/{owner}/reset`  
→ Resets and re-syncs data for a given repository.

**Request Body:**
```json
{
  "since": "2023-01-01T00:00:00Z"
}
```

## 📦 Database Schema

### 🗂️ `repositories`

Stores metadata about GitHub repositories.

| Column                   | Type                 | Description                          |
|--------------------------|----------------------|--------------------------------------|
| `id`                     | `SERIAL PRIMARY KEY` | Unique identifier                    |
| `name`                   | `VARCHAR(255)`       | Full name (`owner/repo`), unique     |
| `description`            | `TEXT`               | Repository description               |
| `url`                    | `VARCHAR(255)`       | GitHub URL of the repository         |
| `language`               | `VARCHAR(100)`       | Primary programming language         |
| `forks_count`            | `INTEGER`            | Number of forks                      |
| `stars_count`            | `INTEGER`            | Number of stars                      |
| `open_issues_count`      | `INTEGER`            | Number of open issues                |
| `watchers_count`         | `INTEGER`            | Number of watchers                   |
| `created_at`             | `TIMESTAMP`          | Repository creation time             |
| `updated_at`             | `TIMESTAMP`          | Last updated time on GitHub          |
| `last_commit_fetched_at` | `TIMESTAMP`          | Time of last commit sync             |

---

### 📝 `commits`

Stores commits related to a repository.

| Column          | Type                 | Description                          |
|------------------|----------------------|--------------------------------------|
| `id`             | `SERIAL PRIMARY KEY` | Unique identifier                    |
| `sha`            | `VARCHAR(40)`        | Commit SHA                           |
| `repository_id`  | `INTEGER`            | References `repositories(id)`        |
| `message`        | `TEXT`               | Commit message                       |
| `author_name`    | `VARCHAR(255)`       | Author's GitHub username             |
| `author_email`   | `VARCHAR(255)`       | Author's email address               |
| `author_date`    | `TIMESTAMP`          | Timestamp of the authored commit     |
| `commit_url`     | `VARCHAR(255)`       | URL to the commit on GitHub          |

🔒 **Unique Constraint**:  
`UNIQUE (sha, repository_id)` — Ensures no duplicate commit entries per repository.


### Run tests
go test ./...