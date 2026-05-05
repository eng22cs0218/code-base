package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Generate secure session token
func GenerateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Create session for user
func CreateSession(githubUsername string) (string, error) {
	token, err := GenerateSessionToken()
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(24 * time.Hour) // 24 hour session

	var userID int
	var orgID string
	err = DB.QueryRow(`SELECT id, org_id FROM github_credentials WHERE github_username = $1`, githubUsername).Scan(&userID, &orgID)
	if err != nil {
		return "", err
	}

	_, err = DB.Exec(
		`INSERT INTO user_sessions (user_id, org_id, session_token, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, orgID, token, expiresAt, time.Now(),
	)

	if err != nil {
		return "", err
	}

	return token, nil
}

// Validate session token and return github_username
func ValidateSession(token string) (string, error) {
	var githubUsername string
	var expiresAt time.Time

	err := DB.QueryRow(
		`SELECT gc.github_username, us.expires_at 
		 FROM user_sessions us
		 JOIN github_credentials gc ON us.user_id = gc.id
		 WHERE us.session_token = $1`,
		token,
	).Scan(&githubUsername, &expiresAt)

	if err != nil {
		return "", err
	}

	if time.Now().After(expiresAt) {
		return "", fmt.Errorf("session expired")
	}

	return githubUsername, nil
}

// Invalidate session (logout)
func InvalidateSession(token string) error {
	_, err := DB.Exec(
		`DELETE FROM user_sessions WHERE session_token = $1`,
		token,
	)
	return err
}

// Invalidate all user sessions (logout from all devices)
func InvalidateAllUserSessions(githubUsername string) error {
	// Find the user_id first
	var userID int
	err := DB.QueryRow(`SELECT id FROM github_credentials WHERE github_username = $1`, githubUsername).Scan(&userID)
	if err != nil {
		return err
	}
	
	_, err = DB.Exec(
		`DELETE FROM user_sessions WHERE user_id = $1`,
		userID,
	)
	return err
}

// GetUserCredID gets cred_id from github_username
func GetUserCredID(githubUsername string) (string, error) {
	var credID string
	err := DB.QueryRow(
		`SELECT cred_id FROM github_credentials WHERE github_username = $1`,
		githubUsername,
	).Scan(&credID)

	if err != nil {
		return "", err
	}

	return credID, nil
}

// GetUserOrgID gets org_id from github_username
func GetUserOrgID(githubUsername string) (string, error) {
	var orgID string
	err := DB.QueryRow(
		`SELECT org_id FROM github_credentials WHERE github_username = $1`,
		githubUsername,
	).Scan(&orgID)

	if err != nil {
		return "", err
	}

	return orgID, nil
}
