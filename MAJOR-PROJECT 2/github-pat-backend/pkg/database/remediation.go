package database

import (
	"fmt"
	"time"
)

// RemediationExecution represents a single remediation execution attempt
type RemediationExecution struct {
	ID          int
	FindingID   string
	OrgID       string
	Command     string
	Status      string
	Output      string
	ExitCode    *int
	ExecutedAt  time.Time
	CompletedAt *time.Time
}

// CreateRemediationExecution inserts a new execution record and returns the ID.
func CreateRemediationExecution(findingID, orgID, command string) (int, error) {
	var id int
	err := DB.QueryRow(
		`INSERT INTO remediation_executions (finding_id, org_id, command, status)
		 VALUES ($1, $2, $3, 'pending')
		 RETURNING id`,
		findingID, orgID, command,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create remediation execution: %w", err)
	}
	return id, nil
}

// UpdateRemediationStatus updates only the status of an execution.
func UpdateRemediationStatus(id int, status string) error {
	_, err := DB.Exec(
		`UPDATE remediation_executions SET status = $1 WHERE id = $2`,
		status, id,
	)
	return err
}

// AppendRemediationOutput appends a line of output to the execution record.
func AppendRemediationOutput(id int, line string) error {
	_, err := DB.Exec(
		`UPDATE remediation_executions SET output = COALESCE(output, '') || $1 || E'\n' WHERE id = $2`,
		line, id,
	)
	return err
}

// CompleteRemediation marks an execution as done with final status, output, and exit code.
func CompleteRemediation(id int, status string, exitCode int) error {
	_, err := DB.Exec(
		`UPDATE remediation_executions
		 SET status = $1, exit_code = $2, completed_at = NOW()
		 WHERE id = $3`,
		status, exitCode, id,
	)
	return err
}

// GetRemediationByID retrieves a single execution by ID.
func GetRemediationByID(id int) (*RemediationExecution, error) {
	r := &RemediationExecution{}
	err := DB.QueryRow(
		`SELECT id, finding_id, org_id, command, status, COALESCE(output,''), exit_code, executed_at, completed_at
		 FROM remediation_executions WHERE id = $1`,
		id,
	).Scan(&r.ID, &r.FindingID, &r.OrgID, &r.Command, &r.Status, &r.Output, &r.ExitCode, &r.ExecutedAt, &r.CompletedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// GetRemediationCommandByFinding fetches the command and correct_config from agent_output for a given finding_id.
func GetRemediationCommandByFinding(findingID string) (string, string, error) {
	var command, correctConfig string
	err := DB.QueryRow(
		`SELECT COALESCE(command, ''), COALESCE(correct_config, '')
		 FROM agent_output
		 WHERE finding_id = $1
		 ORDER BY created_at DESC LIMIT 1`,
		findingID,
	).Scan(&command, &correctConfig)
	if err != nil {
		return "", "", fmt.Errorf("no remediation command found for finding %s: %w", findingID, err)
	}
	if command == "" {
		return "", "", fmt.Errorf("no remediation command found for finding %s", findingID)
	}
	return command, correctConfig, nil
}

// GetRecentRemediations returns the most recent executions for an org.
func GetRecentRemediations(orgID string, limit int) ([]RemediationExecution, error) {
	rows, err := DB.Query(
		`SELECT id, finding_id, org_id, command, status, COALESCE(output,''), exit_code, executed_at, completed_at
		 FROM remediation_executions
		 WHERE org_id = $1
		 ORDER BY executed_at DESC
		 LIMIT $2`,
		orgID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RemediationExecution
	for rows.Next() {
		var r RemediationExecution
		if err := rows.Scan(&r.ID, &r.FindingID, &r.OrgID, &r.Command, &r.Status, &r.Output, &r.ExitCode, &r.ExecutedAt, &r.CompletedAt); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}
