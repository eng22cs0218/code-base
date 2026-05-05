package database

import (
	"time"
)

// Repository represents a GitHub repository
type Repository struct {
	ID        int
	RepoID    string
	OrgID     string
	CredID    string
	RepoName  string
	RepoURL   string
	CreatedAt time.Time
}

// CreateRepository creates a new repository record
func CreateRepository(repoID, orgID, credID, repoName, repoURL string) error {
	_, err := DB.Exec(
		`INSERT INTO github_repository (repo_id, org_id, cred_id, repo_name, repo_url, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		repoID, orgID, credID, repoName, repoURL, time.Now(),
	)
	return err
}

// GetRepositoriesByOrgID retrieves all repositories for an organization
func GetRepositoriesByOrgID(orgID string) ([]Repository, error) {
	rows, err := DB.Query(
		`SELECT id, repo_id, org_id, cred_id, repo_name, repo_url, created_at
		 FROM github_repository
		 WHERE org_id = $1
		 ORDER BY repo_name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var repo Repository
		if err := rows.Scan(&repo.ID, &repo.RepoID, &repo.OrgID, &repo.CredID,
			&repo.RepoName, &repo.RepoURL, &repo.CreatedAt); err != nil {
			continue
		}
		repos = append(repos, repo)
	}

	return repos, nil
}

// GetRepositoryByID retrieves a repository by repo_id
func GetRepositoryByID(repoID string) (*Repository, error) {
	repo := &Repository{}
	err := DB.QueryRow(
		`SELECT id, repo_id, org_id, cred_id, repo_name, repo_url, created_at
		 FROM github_repository
		 WHERE repo_id = $1`,
		repoID,
	).Scan(&repo.ID, &repo.RepoID, &repo.OrgID, &repo.CredID,
		&repo.RepoName, &repo.RepoURL, &repo.CreatedAt)

	if err != nil {
		return nil, err
	}

	return repo, nil
}

// GetRepositoryByName retrieves a repository by org_id and repo_name
func GetRepositoryByName(orgID, repoName string) (*Repository, error) {
	repo := &Repository{}
	err := DB.QueryRow(
		`SELECT id, repo_id, org_id, cred_id, repo_name, repo_url, created_at
		 FROM github_repository
		 WHERE org_id = $1 AND repo_name = $2`,
		orgID, repoName,
	).Scan(&repo.ID, &repo.RepoID, &repo.OrgID, &repo.CredID,
		&repo.RepoName, &repo.RepoURL, &repo.CreatedAt)

	if err != nil {
		return nil, err
	}

	return repo, nil
}

// CheckRepoIDExists checks if repo_id already exists
func CheckRepoIDExists(repoID string) (bool, error) {
	var exists bool
	err := DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM github_repository WHERE repo_id = $1)`,
		repoID,
	).Scan(&exists)

	return exists, err
}

// CountRepositoriesByOrgID counts repositories for an organization
func CountRepositoriesByOrgID(orgID string) (int, error) {
	var count int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM github_repository WHERE org_id = $1`,
		orgID,
	).Scan(&count)

	return count, err
}

// DeleteRepositoriesByOrgID deletes all repositories for an organization
func DeleteRepositoriesByOrgID(orgID string) error {
	_, err := DB.Exec(
		`DELETE FROM github_repository WHERE org_id = $1`,
		orgID,
	)
	return err
}
