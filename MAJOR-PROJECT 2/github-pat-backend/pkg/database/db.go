package database

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

// Seed RNG once
func init() {
	rand.Seed(time.Now().UnixNano())
}

// --------------------
// Initialize Database
// --------------------
func InitDB(host, port, user, password, dbname string) error {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		return err
	}

	log.Println("✅ Connected to PostgreSQL database")
	return createTables()
}

// --------------------
// Create Tables
// --------------------
func createTables() error {
	query := `
	-- =========================
	-- ORGANIZATION
	-- =========================
	CREATE TABLE IF NOT EXISTS organization (
		id SERIAL PRIMARY KEY,
		org_id VARCHAR(10) UNIQUE NOT NULL,
		org_code VARCHAR(6) UNIQUE NOT NULL,
		name VARCHAR(255) NOT NULL,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
	);

	-- =========================
	-- GITHUB CREDENTIALS
	-- =========================
	CREATE TABLE IF NOT EXISTS github_credentials (
		id SERIAL PRIMARY KEY,
		cred_id VARCHAR(6) UNIQUE NOT NULL,
		org_id VARCHAR(10) NOT NULL,
		github_username VARCHAR(255) UNIQUE NOT NULL,
		encrypted_pat TEXT NOT NULL,
		password TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
		is_active BOOLEAN DEFAULT TRUE,
		CONSTRAINT fk_cred_org
			FOREIGN KEY (org_id)
			REFERENCES organization(org_id)
			ON DELETE CASCADE
	);

	-- =========================
	-- USER SESSIONS
	-- =========================
	CREATE TABLE IF NOT EXISTS user_sessions (
		id SERIAL PRIMARY KEY,
		session_token VARCHAR(64) UNIQUE NOT NULL,
		user_id INTEGER NOT NULL,
		org_id VARCHAR(10) NOT NULL,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
		expires_at TIMESTAMPTZ NOT NULL,
		is_active BOOLEAN DEFAULT TRUE,
		CONSTRAINT fk_session_user
			FOREIGN KEY (user_id)
			REFERENCES github_credentials(id)
			ON DELETE CASCADE,
		CONSTRAINT fk_session_org
			FOREIGN KEY (org_id)
			REFERENCES organization(org_id)
			ON DELETE CASCADE
	);

	-- =========================
	-- GITHUB REPOSITORY
	-- =========================
	CREATE TABLE IF NOT EXISTS github_repository (
		repo_id VARCHAR(6) PRIMARY KEY,
		org_id VARCHAR(10) NOT NULL,
		cred_id VARCHAR(6) NOT NULL,
		repo_name VARCHAR(255) NOT NULL,
		repo_url TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
		CONSTRAINT fk_repo_org
			FOREIGN KEY (org_id)
			REFERENCES organization(org_id)
			ON DELETE CASCADE,
		CONSTRAINT fk_repo_cred
			FOREIGN KEY (cred_id)
			REFERENCES github_credentials(cred_id)
			ON DELETE CASCADE
	);

	-- =========================
	-- GITHUB FILES
	-- =========================
	CREATE TABLE IF NOT EXISTS github_files (
		id SERIAL PRIMARY KEY,
		org_id VARCHAR(10) NOT NULL,
		repo_id VARCHAR(6) NOT NULL,
		branch VARCHAR(255) NOT NULL,
		file_path TEXT NOT NULL,
		commit_sha VARCHAR(255),
		content_hash VARCHAR(255),
		commit_time TIMESTAMPTZ,
		is_deleted BOOLEAN DEFAULT FALSE,
		CONSTRAINT fk_file_org
			FOREIGN KEY (org_id)
			REFERENCES organization(org_id)
			ON DELETE CASCADE,
		CONSTRAINT fk_file_repo
			FOREIGN KEY (repo_id)
			REFERENCES github_repository(repo_id)
			ON DELETE CASCADE,
		CONSTRAINT unique_file_commit
			UNIQUE (commit_sha, content_hash)
	);

	-- Ensure legacy content column is removed (idempotent migration)
	ALTER TABLE github_files DROP COLUMN IF EXISTS content;

	-- =========================
	-- KUBERNETES RESOURCE
	-- =========================
	CREATE TABLE IF NOT EXISTS kubernetes_resource (
		id SERIAL PRIMARY KEY,
		resource_id VARCHAR(6) UNIQUE NOT NULL,
		org_id VARCHAR(10) NOT NULL,
		repo_file_id INTEGER,
		cluster_id VARCHAR(255),
		kind VARCHAR(100) NOT NULL,
		name VARCHAR(255) NOT NULL,
		namespace VARCHAR(255),
		yaml_content JSONB,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
		CONSTRAINT fk_k8s_org
			FOREIGN KEY (org_id)
			REFERENCES organization(org_id)
			ON DELETE CASCADE,
		CONSTRAINT fk_k8s_file
			FOREIGN KEY (repo_file_id)
			REFERENCES github_files(id)
			ON DELETE CASCADE
	);

	-- =========================
	-- FINDINGS
	-- =========================
	CREATE TABLE IF NOT EXISTS findings (
		finding_id VARCHAR(6) PRIMARY KEY,
		org_id VARCHAR(10) NOT NULL,
		resource_id VARCHAR(6),
		severity VARCHAR(50) NOT NULL,
		description TEXT,
		recommendations TEXT,
		status VARCHAR(50) DEFAULT 'open',
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
		CONSTRAINT fk_finding_org
			FOREIGN KEY (org_id)
			REFERENCES organization(org_id)
			ON DELETE CASCADE
	);

	-- Idempotent migrations: add columns required by the validation service
	ALTER TABLE findings DROP CONSTRAINT IF EXISTS fk_finding_resource;
	ALTER TABLE findings ALTER COLUMN resource_id DROP NOT NULL;
	ALTER TABLE findings ADD COLUMN IF NOT EXISTS namespace VARCHAR(255);
	ALTER TABLE findings ADD COLUMN IF NOT EXISTS missing_kind VARCHAR(100);
	ALTER TABLE findings ADD COLUMN IF NOT EXISTS detected_at TIMESTAMPTZ;
	-- Back-fill detected_at from created_at for existing rows
	UPDATE findings SET detected_at = created_at WHERE detected_at IS NULL;

	-- =========================
	-- CONFIG CREDENTIALS (TERMINAL)
	-- =========================
	CREATE TABLE IF NOT EXISTS config_credentials (
		id SERIAL PRIMARY KEY,
		org_id VARCHAR(10) NOT NULL,
		context_name VARCHAR(255) NOT NULL,
		config_file TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
		CONSTRAINT fk_config_cred_org
			FOREIGN KEY (org_id)
			REFERENCES organization(org_id)
			ON DELETE CASCADE
	);

	-- =========================
	-- AGENT OUTPUT
	-- =========================
	CREATE TABLE IF NOT EXISTS agent_output (
		id SERIAL PRIMARY KEY,
		org_id VARCHAR(10) NOT NULL,
		session_token VARCHAR(64) NOT NULL,
		correct_config TEXT,
		command TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
		CONSTRAINT fk_agent_output_org
			FOREIGN KEY (org_id)
			REFERENCES organization(org_id)
			ON DELETE CASCADE
	);


	-- =========================
	-- PHASE 2 & 3 MIGRATIONS (additive only)
	-- =========================
	ALTER TABLE config_credentials ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'pending';
	ALTER TABLE config_credentials ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
	
	-- In-Memory Session Migration
	ALTER TABLE agent_output DROP CONSTRAINT IF EXISTS fk_agent_output_session;
	ALTER TABLE config_credentials DROP COLUMN IF EXISTS token;

	-- =========================
	-- PHASE 3: REMEDIATION EXECUTIONS
	-- =========================
	CREATE TABLE IF NOT EXISTS remediation_executions (
		id SERIAL PRIMARY KEY,
		finding_id VARCHAR(6) NOT NULL,
		org_id VARCHAR(10) NOT NULL,
		command TEXT NOT NULL,
		status VARCHAR(50) DEFAULT 'pending',
		output TEXT DEFAULT '',
		exit_code INTEGER,
		executed_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		completed_at TIMESTAMPTZ,
		CONSTRAINT fk_remediation_org
			FOREIGN KEY (org_id)
			REFERENCES organization(org_id)
			ON DELETE CASCADE
	);

	-- Phase 3 migration: add finding_id to agent_output so remediation can look up by finding
	ALTER TABLE agent_output ADD COLUMN IF NOT EXISTS finding_id VARCHAR(6);
	`

	_, err := DB.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create tables: %v", err)
	}

	log.Println("✅ Database tables ready")
	return nil
}

// --------------------
// Generate Unique org_id (10-digit Numerical)
// --------------------
func GenerateUniqueOrgID() string {
	// Generate 10-digit number (1000000000 to 9999999999)
	min := int64(1000000000)
	max := int64(9999999999)
	orgID := min + rand.Int63n(max-min+1)
	return fmt.Sprintf("%d", orgID)
}

// --------------------
// Generate Unique 6-digit ID (for org_code, cred_id, repo_id, cluster_id, resource_id)
// --------------------
func GenerateUnique6DigitID() string {
	// Generate 6-digit number (100000 to 999999)
	min := int64(100000)
	max := int64(999999)
	id := min + rand.Int63n(max-min+1)
	return fmt.Sprintf("%d", id)
}
