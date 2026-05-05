package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateTerminalToken generates a random token for connecting the terminal
func GenerateTerminalToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateConfigCredential saves an initial placeholder for the config file. No token is used in the DB.
func CreateConfigCredential(orgID string, contextName string) error {
	_, err := DB.Exec(
		`INSERT INTO config_credentials (org_id, context_name, config_file) VALUES ($1, $2, $3)`,
		orgID, contextName, "",
	)
	return err
}

// UpdateConfigCredential saves the actual uploaded config file
func UpdateConfigCredential(orgID string, contextName string, configFile string) error {
	res, err := DB.Exec(
		`UPDATE config_credentials SET config_file = $1 WHERE org_id = $2 AND context_name = $3`,
		configFile, orgID, contextName,
	)
	if err != nil {
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no config_credential found for org %s context %s", orgID, contextName)
	}

	return nil
}

// GetConfigCredential fetches the config string from the DB
func GetConfigCredential(orgID string, contextName string) (string, error) {
	var configFile string
	err := DB.QueryRow(
		`SELECT config_file FROM config_credentials WHERE org_id = $1 AND context_name = $2 ORDER BY created_at DESC LIMIT 1`,
		orgID, contextName,
	).Scan(&configFile)
	if err != nil {
		return "", err
	}

	return configFile, nil
}

// SaveAgentOutput saves the command related to a token
func SaveAgentOutput(orgID string, sessionToken string, command string, correctConfig string, findingID string) error {
	_, err := DB.Exec(
		`INSERT INTO agent_output (org_id, session_token, command, correct_config, finding_id) VALUES ($1, $2, $3, $4, $5)`,
		orgID, sessionToken, command, correctConfig, findingID,
	)
	return err
}

// (Deleted) GetOrgIDByToken and GetLatestConfigToken were removed since tokens are now fully in-memory.

// GetAgentOutput gets the command for a token
func GetAgentOutput(sessionToken string) (string, string, error) {
	var command, correctConfig string
	err := DB.QueryRow(
		`SELECT command, correct_config FROM agent_output WHERE session_token = $1`,
		sessionToken,
	).Scan(&command, &correctConfig)

	if err != nil {
		return "", "", err
	}

	return command, correctConfig, nil
}

// GetLatestAgentOutput gets the most recent remediation command for a token
func GetLatestAgentOutput(sessionToken string) (string, string, error) {
	var command, correctConfig string
	err := DB.QueryRow(
		`SELECT command, COALESCE(correct_config, '') FROM agent_output WHERE session_token = $1 ORDER BY created_at DESC LIMIT 1`,
		sessionToken,
	).Scan(&command, &correctConfig)

	if err != nil {
		return "", "", err
	}

	return command, correctConfig, nil
}

// ─── Phase 2: New DB Functions ───────────────────────────────────────────────
// These functions are additive — all existing functions above remain untouched.

// CreatePendingConfig inserts a new config_credentials row with status='pending'.
func CreatePendingConfig(orgID, contextName string) error {
	_, err := DB.Exec(
		`INSERT INTO config_credentials (org_id, context_name, config_file, status)
		 VALUES ($1, $2, '', 'pending')`,
		orgID, contextName,
	)
	return err
}

// ActivateConfig updates the config_file and sets status='active' for a given org_id + context_name.
func ActivateConfig(orgID, contextName, configContent string) error {
	res, err := DB.Exec(
		`UPDATE config_credentials
		 SET config_file = $1, status = 'active', updated_at = NOW()
		 WHERE org_id = $2 AND context_name = $3 AND status = 'pending'`,
		configContent, orgID, contextName,
	)
	if err != nil {
		return fmt.Errorf("failed to activate config or pending not found: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no pending config found for org %s context %s", orgID, contextName)
	}

	return nil
}

// GetConfigStatusByContext returns the latest status for a given org_id + context_name.
func GetConfigStatusByContext(orgID, contextName string) (string, error) {
	var status string
	err := DB.QueryRow(
		`SELECT status FROM config_credentials
		 WHERE org_id = $1 AND context_name = $2
		 ORDER BY created_at DESC LIMIT 1`,
		orgID, contextName,
	).Scan(&status)
	if err != nil {
		return "", err
	}

	return status, nil
}

