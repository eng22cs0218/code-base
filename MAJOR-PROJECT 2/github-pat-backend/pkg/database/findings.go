package database

import (
	"regexp"
	"time"
)

var (
	reInKindResourceName  = regexp.MustCompile(`(?i)\bin\s+[A-Za-z]+\s+'([^']+)'`)
	reLeadingResourceName = regexp.MustCompile(`(?i)^(?:Service|ConfigMap|Secret|NetworkPolicy|Deployment|DaemonSet)\s+'([^']+)'`)
)

// Finding represents a security finding
type Finding struct {
	ID              int
	FindingID       string
	OrgID           string
	ResourceID      *string
	Severity        string
	Description     string
	Recommendations string
	Status          string
	DetectedAt      time.Time
	Namespace       *string
	MissingKind     *string
}

// CreateFinding creates a new finding record
func CreateFinding(findingID, orgID, resourceID, severity, description, recommendations string) error {
	_, err := DB.Exec(
		`INSERT INTO findings
		 (finding_id, org_id, resource_id, severity, description, recommendations, status, detected_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
		findingID, orgID, resourceID, severity, description, recommendations, "open",
	)
	return err
}

// CreateValidationFinding creates a namespace-level validation finding for missing kinds
func CreateValidationFinding(findingID, orgID, namespace, missingKind, severity, description, recommendations string) error {
	resolvedResourceID := resolveFindingResourceID(orgID, namespace, missingKind, description)

	var resourceIDArg interface{}
	if resolvedResourceID != "" {
		resourceIDArg = resolvedResourceID
	}

	_, err := DB.Exec(
		`INSERT INTO findings
		 (finding_id, org_id, resource_id, namespace, missing_kind, severity, description, recommendations, status, detected_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())`,
		findingID, orgID, resourceIDArg, namespace, missingKind, severity, description, recommendations, "open",
	)
	return err
}

// DeleteValidationFindingsByOrgID deletes all validation findings for an organization
func DeleteValidationFindingsByOrgID(orgID string) error {
	_, err := DB.Exec(
		`DELETE FROM findings WHERE org_id = $1 AND missing_kind IS NOT NULL`,
		orgID,
	)
	return err
}

func resolveFindingResourceID(orgID, namespace, kind, description string) string {
	resourceName := extractResourceName(description)

	if resourceName != "" {
		var resourceID string
		err := DB.QueryRow(
			`SELECT resource_id
			 FROM kubernetes_resource
			 WHERE org_id = $1 AND namespace = $2 AND LOWER(kind) = LOWER($3) AND name = $4
			 ORDER BY created_at DESC
			 LIMIT 1`,
			orgID, namespace, kind, resourceName,
		).Scan(&resourceID)
		if err == nil {
			return resourceID
		}
	}

	if namespace != "" {
		var resourceID string
		err := DB.QueryRow(
			`SELECT resource_id
			 FROM kubernetes_resource
			 WHERE org_id = $1 AND namespace = $2
			 ORDER BY created_at DESC
			 LIMIT 1`,
			orgID, namespace,
		).Scan(&resourceID)
		if err == nil {
			return resourceID
		}
	}

	var resourceID string
	err := DB.QueryRow(
		`SELECT resource_id
		 FROM kubernetes_resource
		 WHERE org_id = $1
		 ORDER BY created_at DESC
		 LIMIT 1`,
		orgID,
	).Scan(&resourceID)
	if err == nil {
		return resourceID
	}

	return ""
}

func extractResourceName(description string) string {
	matches := reInKindResourceName.FindStringSubmatch(description)
	if len(matches) > 1 {
		return matches[1]
	}

	matches = reLeadingResourceName.FindStringSubmatch(description)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// GetFindingByID retrieves a finding by finding_id
func GetFindingByID(findingID string) (*Finding, error) {
	finding := &Finding{}
	err := DB.QueryRow(
		`SELECT id, finding_id, org_id, resource_id, severity, description, recommendations, status, detected_at
		 FROM findings
		 WHERE finding_id = $1`,
		findingID,
	).Scan(&finding.ID, &finding.FindingID, &finding.OrgID, &finding.ResourceID,
		&finding.Severity, &finding.Description, &finding.Recommendations,
		&finding.Status, &finding.DetectedAt)

	if err != nil {
		return nil, err
	}

	return finding, nil
}

// GetFindingsByOrgID retrieves all findings for an organization
func GetFindingsByOrgID(orgID string) ([]Finding, error) {
	rows, err := DB.Query(
		`SELECT id, finding_id, org_id, resource_id, severity, description, recommendations, status, detected_at
		 FROM findings
		 WHERE org_id = $1
		 ORDER BY detected_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []Finding
	for rows.Next() {
		var finding Finding
		if err := rows.Scan(&finding.ID, &finding.FindingID, &finding.OrgID, &finding.ResourceID,
			&finding.Severity, &finding.Description, &finding.Recommendations,
			&finding.Status, &finding.DetectedAt); err != nil {
			continue
		}
		findings = append(findings, finding)
	}

	return findings, nil
}

// GetFindingsByResourceID retrieves all findings for a specific resource
func GetFindingsByResourceID(resourceID string) ([]Finding, error) {
	rows, err := DB.Query(
		`SELECT id, finding_id, org_id, resource_id, severity, description, recommendations, status, detected_at
		 FROM findings
		 WHERE resource_id = $1
		 ORDER BY detected_at DESC`,
		resourceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []Finding
	for rows.Next() {
		var finding Finding
		if err := rows.Scan(&finding.ID, &finding.FindingID, &finding.OrgID, &finding.ResourceID,
			&finding.Severity, &finding.Description, &finding.Recommendations,
			&finding.Status, &finding.DetectedAt); err != nil {
			continue
		}
		findings = append(findings, finding)
	}

	return findings, nil
}

// UpdateFindingStatus updates the status of a finding
func UpdateFindingStatus(findingID, status string) error {
	_, err := DB.Exec(
		`UPDATE findings SET status = $1 WHERE finding_id = $2`,
		status, findingID,
	)
	return err
}

// CheckFindingIDExists checks if finding_id already exists
func CheckFindingIDExists(findingID string) (bool, error) {
	var exists bool
	err := DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM findings WHERE finding_id = $1)`,
		findingID,
	).Scan(&exists)

	return exists, err
}

// CountFindingsByOrgID counts findings for an organization
func CountFindingsByOrgID(orgID string) (int, error) {
	var count int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM findings WHERE org_id = $1`,
		orgID,
	).Scan(&count)

	return count, err
}

// CountFindingsBySeverity counts findings by severity for an organization
func CountFindingsBySeverity(orgID, severity string) (int, error) {
	var count int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM findings WHERE org_id = $1 AND severity = $2`,
		orgID, severity,
	).Scan(&count)

	return count, err
}

// DeleteFindingsByOrgID deletes all findings for an organization
func DeleteFindingsByOrgID(orgID string) error {
	_, err := DB.Exec(
		`DELETE FROM findings WHERE org_id = $1`,
		orgID,
	)
	return err
}
