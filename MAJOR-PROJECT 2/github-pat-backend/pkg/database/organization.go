package database

import (
	"time"
)

// Organization represents an organization entity
type Organization struct {
	ID        int
	OrgID     string
	OrgCode   string
	Name      string
	CreatedAt time.Time
}

// CreateOrganization creates a new organization
func CreateOrganization(orgID, orgCode, name string) (int, error) {
	var id int
	err := DB.QueryRow(
		`INSERT INTO organization (org_id, org_code, name, created_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		orgID, orgCode, name, time.Now(),
	).Scan(&id)

	return id, err
}

// GetOrganizationByID retrieves organization by org_id
func GetOrganizationByID(orgID string) (*Organization, error) {
	org := &Organization{}
	err := DB.QueryRow(
		`SELECT id, org_id, org_code, name, created_at
		 FROM organization
		 WHERE org_id = $1`,
		orgID,
	).Scan(&org.ID, &org.OrgID, &org.OrgCode, &org.Name, &org.CreatedAt)

	if err != nil {
		return nil, err
	}

	return org, nil
}

// GetOrganizationByCode retrieves organization by org_code
func GetOrganizationByCode(orgCode string) (*Organization, error) {
	org := &Organization{}
	err := DB.QueryRow(
		`SELECT id, org_id, org_code, name, created_at
		 FROM organization
		 WHERE org_code = $1`,
		orgCode,
	).Scan(&org.ID, &org.OrgID, &org.OrgCode, &org.Name, &org.CreatedAt)

	if err != nil {
		return nil, err
	}

	return org, nil
}

// CheckOrgIDExists checks if org_id already exists
func CheckOrgIDExists(orgID string) (bool, error) {
	var exists bool
	err := DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM organization WHERE org_id = $1)`,
		orgID,
	).Scan(&exists)

	return exists, err
}

// CheckOrgCodeExists checks if org_code already exists
func CheckOrgCodeExists(orgCode string) (bool, error) {
	var exists bool
	err := DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM organization WHERE org_code = $1)`,
		orgCode,
	).Scan(&exists)

	return exists, err
}
