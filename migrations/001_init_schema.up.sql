-- repositories table
CREATE TABLE IF NOT EXISTS repositories (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    url TEXT NOT NULL,
    language TEXT,
    forks_count INTEGER,
    stars_count INTEGER,
    open_issues_count INTEGER,
    watchers_count INTEGER,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE,
    last_commit_fetched_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT unique_repo_name UNIQUE (name)
);

-- commits table
CREATE TABLE IF NOT EXISTS commits (
    id SERIAL PRIMARY KEY,
    sha TEXT NOT NULL,
    repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    author_name TEXT,
    author_email TEXT,
    author_date TIMESTAMP WITH TIME ZONE NOT NULL,
    commit_url TEXT NOT NULL,
    CONSTRAINT unique_commit_per_repo UNIQUE (sha, repository_id)
);

CREATE INDEX IF NOT EXISTS idx_commits_repository_id ON commits(repository_id);
CREATE INDEX IF NOT EXISTS idx_commits_author_date ON commits(author_date);
CREATE INDEX IF NOT EXISTS idx_commits_author_name ON commits(author_name);
