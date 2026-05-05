package authentication

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	GitHubUsername string `json:"github_username" binding:"required"`
	Password       string `json:"password" binding:"required"`
}

type LoginResponse struct {
	SessionToken     string `json:"session_token"`
	CredID           string `json:"cred_id"`
	OrgID            string `json:"org_id"`
	OrgCode          string `json:"org_code"`
	OrganizationName string `json:"organization_name"`
	GitHubUsername   string `json:"github_username"`
	CreatedAt        string `json:"created_at"`
	IsActive         bool   `json:"is_active"`
	Redirect         string `json:"redirect"`
}

// Login handles user authentication (NO GitHub fetching)
func Login(c *gin.Context) {
	var req LoginRequest

	// necessary to check - user should put the username and password to click login button and enter into it
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "github_username and password are required", err)
		return
	}

	// not nesssary
	// // Validate password length
	// if len(req.Password) > 12 {
	// 	response.BadRequest(c, "password must be 12 characters or less", nil)
	// 	return
	// }

	// Get user credentials
	var credID, orgID, passwordHash string
	var isActive bool
	var createdAt time.Time


	var userID int
	err := database.DB.QueryRow(`
		SELECT id, cred_id, org_id, password, is_active, created_at
		FROM github_credentials
		WHERE github_username = $1
	`, req.GitHubUsername).Scan(&userID, &credID, &orgID, &passwordHash, &isActive, &createdAt)

	if err != nil {
		response.Unauthorized(c, "invalid credentials", err)
		return
	}

	// Check if account is active
	if !isActive {
		response.Error(c, http.StatusForbidden, "account is inactive", nil)
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		response.Unauthorized(c, "invalid credentials", err)
		return
	}

	// Verify GitHub PAT is still valid
	// patBytes, _ := hex.DecodeString(encryptedPAT)
	// pat := string(patBytes)

	// if !verifyGitHubCredentials(req.GitHubUsername, pat) {
	// 	response.Unauthorized(c, "GitHub Personal Access Token (PAT) is invalid or expired. Please update your credentials", nil)
	// 	return
	// }

	// Create session token
	sessionToken := generateSessionToken(req.GitHubUsername)
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err = database.DB.Exec(`
		INSERT INTO user_sessions (user_id, org_id, session_token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, orgID, sessionToken, expiresAt, time.Now())

	if err != nil {
		response.InternalError(c, "failed to create session", err)
		return
	}

	// Get organization details
	//organization table se org_id ke basis par organization ka code aur naam nikal raha hai aur variables me store kar raha hai.
	var orgCode, orgName string
	err = database.DB.QueryRow(`
		SELECT org_code, name FROM organization WHERE org_id = $1
	`, orgID).Scan(&orgCode, &orgName)

	if err != nil {
		response.InternalError(c, "failed to get organization details", err)
		return
	}

	// Return success response (NO GitHub fetching)
	loginResp := LoginResponse{
		SessionToken:     sessionToken,
		CredID:           credID,
		OrgID:            orgID,
		OrgCode:          orgCode,
		OrganizationName: orgName,
		GitHubUsername:   req.GitHubUsername,
		CreatedAt:        createdAt.Format(time.RFC3339),
		IsActive:         isActive,
		Redirect:         "/fetching-service/dashboard",
	}

	response.Success(c, http.StatusOK, "login successful", loginResp)
}

// generateSessionToken creates a unique session token
func generateSessionToken(username string) string {
	data := fmt.Sprintf("%s-%d", username, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
