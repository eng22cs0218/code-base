package database

import (
	"time"
)

// GitHubFile represents a file from GitHub repository
type GitHubFile struct {
	ID          int
	OrgID       string
	RepoID      string
	Branch      string
	FilePath    string
	CommitSHA   string
	ContentHash string
	CommitTime  time.Time
	IsDeleted   bool
	CreatedAt   time.Time
}

// CreateFile creates a new file record
func CreateFile(orgID, repoID, branch, filePath, commitSHA, contentHash string, commitTime time.Time) error {
	_, err := DB.Exec(
		`INSERT INTO github_files
		 (org_id, repo_id, branch, file_path, commit_sha, content_hash, commit_time, is_deleted, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())`,
		orgID, repoID, branch, filePath, commitSHA, contentHash, commitTime, false,
	)
	return err
}

// GetFileByCommitAndHash retrieves a file by commit_sha and content_hash
func GetFileByCommitAndHash(commitSHA, contentHash string) (*GitHubFile, error) {
	file := &GitHubFile{}
	err := DB.QueryRow(
		`SELECT id, org_id, repo_id, branch, file_path, commit_sha, content_hash, commit_time, is_deleted, created_at
		 FROM github_files
		 WHERE commit_sha = $1 AND content_hash = $2`,
		commitSHA, contentHash,
	).Scan(&file.ID, &file.OrgID, &file.RepoID, &file.Branch, &file.FilePath,
		&file.CommitSHA, &file.ContentHash, &file.CommitTime, &file.IsDeleted, &file.CreatedAt)

	if err != nil {
		return nil, err
	}

	return file, nil
}

// GetFilesByOrgID retrieves all files for an organization
func GetFilesByOrgID(orgID string) ([]GitHubFile, error) {
	rows, err := DB.Query(
		`SELECT id, org_id, repo_id, branch, file_path, commit_sha, content_hash, commit_time, is_deleted, created_at
		 FROM github_files
		 WHERE org_id = $1 AND is_deleted = false
		 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []GitHubFile
	for rows.Next() {
		var file GitHubFile
		if err := rows.Scan(&file.ID, &file.OrgID, &file.RepoID, &file.Branch, &file.FilePath,
			&file.CommitSHA, &file.ContentHash, &file.CommitTime, &file.IsDeleted, &file.CreatedAt); err != nil {
			continue
		}
		files = append(files, file)
	}

	return files, nil
}

// GetFilesByRepoID retrieves all files for a repository
func GetFilesByRepoID(repoID string) ([]GitHubFile, error) {
	rows, err := DB.Query(
		`SELECT id, org_id, repo_id, branch, file_path, commit_sha, content_hash, commit_time, is_deleted, created_at
		 FROM github_files
		 WHERE repo_id = $1 AND is_deleted = false
		 ORDER BY file_path`,
		repoID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []GitHubFile
	for rows.Next() {
		var file GitHubFile
		if err := rows.Scan(&file.ID, &file.OrgID, &file.RepoID, &file.Branch, &file.FilePath,
			&file.CommitSHA, &file.ContentHash, &file.CommitTime, &file.IsDeleted, &file.CreatedAt); err != nil {
			continue
		}
		files = append(files, file)
	}

	return files, nil
}

// CheckFileExists checks if a file with commit_sha and content_hash exists
func CheckFileExists(commitSHA, contentHash string) (bool, int, error) {
	var exists bool
	var fileID int
	err := DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM github_files WHERE commit_sha = $1 AND content_hash = $2), 
		 COALESCE((SELECT id FROM github_files WHERE commit_sha = $1 AND content_hash = $2 LIMIT 1), 0)`,
		commitSHA, contentHash,
	).Scan(&exists, &fileID)

	return exists, fileID, err
}

// MarkFileAsDeleted marks a file as deleted
func MarkFileAsDeleted(fileID int) error {
	_, err := DB.Exec(
		`UPDATE github_files SET is_deleted = true WHERE id = $1`,
		fileID,
	)
	return err
}

// CountFilesByOrgID counts files for an organization
func CountFilesByOrgID(orgID string) (int, error) {
	var count int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM github_files WHERE org_id = $1 AND is_deleted = false`,
		orgID,
	).Scan(&count)

	return count, err
}
