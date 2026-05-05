package database

import (
	"time"
)

// Credential represents GitHub credentials
type Credential struct {
	ID             int
	CredID         string
	OrgID          string
	GitHubUsername string
	EncryptedPAT   string
	Password       string
	CreatedAt      time.Time
	IsActive       bool
}

// CreateCredential creates new GitHub credentials
func CreateCredential(credID, orgID, githubUsername, encryptedPAT, password string) (int, time.Time, error) {
	var id int
	var createdAt time.Time

	err := DB.QueryRow(
		`INSERT INTO github_credentials
		 (cred_id, org_id, github_username, encrypted_pat, password, created_at, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at`,
		credID, orgID, githubUsername, encryptedPAT, password, time.Now(), true,
	).Scan(&id, &createdAt)

	return id, createdAt, err
}

// GetCredentialByUsername retrieves credentials by GitHub username
func GetCredentialByUsername(username string) (*Credential, error) {
	cred := &Credential{}
	err := DB.QueryRow(
		`SELECT id, cred_id, org_id, github_username, encrypted_pat, password, created_at, is_active
		 FROM github_credentials
		 WHERE github_username = $1`,
		username,
	).Scan(&cred.ID, &cred.CredID, &cred.OrgID, &cred.GitHubUsername,
		&cred.EncryptedPAT, &cred.Password, &cred.CreatedAt, &cred.IsActive)

	if err != nil {
		return nil, err
	}

	return cred, nil
}

// GetCredentialByID retrieves credentials by cred_id
func GetCredentialByID(credID string) (*Credential, error) {
	cred := &Credential{}
	err := DB.QueryRow(
		`SELECT id, cred_id, org_id, github_username, encrypted_pat, password, created_at, is_active
		 FROM github_credentials
		 WHERE cred_id = $1`,
		credID,
	).Scan(&cred.ID, &cred.CredID, &cred.OrgID, &cred.GitHubUsername,
		&cred.EncryptedPAT, &cred.Password, &cred.CreatedAt, &cred.IsActive)

	if err != nil {
		return nil, err
	}

	return cred, nil
}

// CheckCredIDExists checks if cred_id already exists
func CheckCredIDExists(credID string) (bool, error) {
	var exists bool
	err := DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM github_credentials WHERE cred_id = $1)`,
		credID,
	).Scan(&exists)

	return exists, err
}

// UpdateCredentialStatus updates the active status of credentials
func UpdateCredentialStatus(credID string, isActive bool) error {
	_, err := DB.Exec(
		`UPDATE github_credentials SET is_active = $1 WHERE cred_id = $2`,
		isActive, credID,
	)
	return err
}
