package authentication

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// signup need
type SignupRequest struct {
	OrganizationName string `json:"organization_name" binding:"required"`
	GitHubUsername   string `json:"github_username" binding:"required"`
	PAT              string `json:"pat" binding:"required"`
	Password         string `json:"password" binding:"required"`
}

// filling the organization table and github_credential table at the same time from the signup
type SignupResponse struct {
	CredID           string `json:"cred_id"`
	OrgID            string `json:"org_id"`
	OrgCode          string `json:"org_code"`
	GitHubUsername   string `json:"github_username"`
	OrganizationName string `json:"organization_name"`
	CreatedAt        string `json:"created_at"`
	Message          string `json:"message"`
}

// Signup function will handles user registration
func Signup(c *gin.Context) {
	// user will be request to signup 
	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// checking the user request is match according to the field json
		response.BadRequest(c, "all fields are required", err)
		return
	}

	// Validate password length
	if len(req.Password) > 12 {
		response.BadRequest(c, "password must be 12 characters or less", nil)
		return
	}

	// Verify GitHub username and PAT(Personal Access Token)
	valid, errMsg := verifyGitHubCredentialsDetailed(req.GitHubUsername, req.PAT)
	if !valid {
		response.Unauthorized(c, errMsg, nil)
		return
	}

	// Check if username already exists in the our database
	var exists bool
	err := database.DB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM github_credentials WHERE github_username = $1)
	`, req.GitHubUsername).Scan(&exists)

	if err != nil {
		response.InternalError(c, "database error", err)
		return
	}

	// if user exist then we will display
	if exists { 
		
		response.Error(c, http.StatusConflict, "github username already exists", nil)
		return
	}

	// Generate unique IDs 
	// we generate the id from the db , as this function is defined within this file
	orgID := generateUniqueID(10)
	orgCode := generateUniqueID(6)
	credID := generateUniqueID(6)

	// Create organization
	_, err = database.DB.Exec(`
		INSERT INTO organization (org_id, org_code, name, created_at)
		VALUES ($1, $2, $3, $4)
	`, orgID, orgCode, req.OrganizationName, time.Now())

	if err != nil {
		response.InternalError(c, "failed to create organization", err)
		return
	}

	// Hash password - we just hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.InternalError(c, "failed to hash password", err)
		return
	}

	// Encrypt PAT (simple encryption for demo) - we cover the pat with help of encypt
	encryptedPAT := encryptPAT(req.PAT)

	// Create user
	var createdAt time.Time
	err = database.DB.QueryRow(`
		INSERT INTO github_credentials (cred_id, org_id, github_username, encrypted_pat, password, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`, credID, orgID, req.GitHubUsername, encryptedPAT, string(hashedPassword), true, time.Now()).Scan(&createdAt)

	if err != nil {
		response.InternalError(c, "failed to create user", err)
		return
	}

	// Return success response (NO auto-login, NO session creation)
	signupResp := SignupResponse{
		CredID:           credID,
		OrgID:            orgID,
		OrgCode:          orgCode,
		GitHubUsername:   req.GitHubUsername,
		OrganizationName: req.OrganizationName,
		CreatedAt:        createdAt.Format(time.RFC3339),
		Message:          "Account created successfully. Please login to continue.",
	}

	response.Success(c, http.StatusCreated, "user created successfully", signupResp)
}

// generateUniqueID generates a random numeric ID of specified length
func generateUniqueID(length int) string {
	max := new(big.Int)
	max.Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, _ := rand.Int(rand.Reader, max)
	return fmt.Sprintf("%0*d", length, n)
}

// verifyGitHubCredentialsDetailed verifies GitHub username and PAT with detailed error
func verifyGitHubCredentialsDetailed(username, pat string) (bool, string) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return false, "Failed to create request"
	}

	req.Header.Set("Authorization", "token "+ pat )
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return false, "Failed to connect to GitHub API"
	}
	defer resp.Body.Close()

	// Check PAT validity
	if resp.StatusCode == 401 {
		// PAT is invalid - we can't verify username
		// Check if username exists on GitHub separately
		usernameExists := checkGitHubUsernameExists(username)
		if !usernameExists {
			return false, "Invalid GitHub username and invalid GitHub Personal Access Token (PAT)"
		}
		return false, "Invalid GitHub Personal Access Token (PAT)"
	}

	if resp.StatusCode != 200 {
		return false, fmt.Sprintf("GitHub API error (status %d)", resp.StatusCode)
	}

	// Parse response to verify username
	var githubUser struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
		return false, "Failed to parse GitHub response"
	}

	// Verify provided username matches the PAT owner
	if githubUser.Login != username {
		return false, "Invalid GitHub username. The provided PAT belongs to a different user"
	}

	return true, ""
}

// checkGitHubUsernameExists checks if a GitHub username exists with Personal Access Token Via API Call
func checkGitHubUsernameExists(username string) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/users/%s", username), nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// 200 = user exists, 404 = user not found
	return resp.StatusCode == 200
}

// encryptPAT encrypts the PAT (simple hex encoding for demo)
func encryptPAT(pat string) string {
	return hex.EncodeToString([]byte(pat))
}



// verifyGitHubCredentials verifies GitHub username and PAT
// func verifyGitHubCredentials(username, pat string) bool {
// 	client := &http.Client{Timeout: 10 * time.Second}
// 	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
// 	req.Header.Set("Authorization", "token "+pat)
// 	req.Header.Set("Accept", "application/vnd.github.v3+json")

// 	resp, err := client.Do(req)
// 	if err != nil || resp.StatusCode != 200 {
// 		return false
// 	}
// 	defer resp.Body.Close()

// 	// Parse response to verify username matches PAT owner
// 	var githubUser struct {
// 		Login string `json:"login"`
// 	}
// 	if err := json.NewDecoder(resp.Body).Decode(&githubUser); err != nil {
// 		return false
// 	}

// 	return githubUser.Login == username
// }

// 	/