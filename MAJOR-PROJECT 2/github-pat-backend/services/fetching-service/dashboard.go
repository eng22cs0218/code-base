package fetchingservice

import (
	"net/http"

	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type DashboardRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
}

type DashboardResponse struct {
	User         UserInfo         `json:"user"`
	Organization OrganizationInfo `json:"organization"`
	Stats        Stats            `json:"stats"`
}

type UserInfo struct {
	CredID         string `json:"cred_id"`
	GitHubUsername string `json:"github_username"`
	CreatedAt      string `json:"created_at"`
	IsActive       bool   `json:"is_active"`
}

type OrganizationInfo struct {
	OrgID   string `json:"org_id"`
	OrgCode string `json:"org_code"`
	Name    string `json:"name"`
}

type Stats struct {
	TotalRepos    int `json:"total_repos"`
	TotalFiles    int `json:"total_files"`
	K8sResources  int `json:"k8s_resources"`
	TotalFindings int `json:"total_findings"`
}

// Dashboard returns dashboard summary
func Dashboard(c *gin.Context) {
	var req DashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "session_token is required", err)
		return
	}

	// Validate session
	githubUsername, err := database.ValidateSession(req.SessionToken)
	if err != nil {
		response.Unauthorized(c, "invalid or expired session", err)
		return
	}

	// Get user's cred_id and org_id
	credID, err := database.GetUserCredID(githubUsername)
	if err != nil {
		response.InternalError(c, "failed to get user credentials", err)
		return
	}

	orgID, err := database.GetUserOrgID(githubUsername)
	if err != nil {
		response.InternalError(c, "failed to get organization", err)
		return
	}

	// Get user info
	var user UserInfo
	err = database.DB.QueryRow(`
		SELECT cred_id, github_username, created_at, is_active
		FROM github_credentials
		WHERE cred_id = $1
	`, credID).Scan(&user.CredID, &user.GitHubUsername, &user.CreatedAt, &user.IsActive)

	if err != nil {
		response.InternalError(c, "failed to get user info", err)
		return
	}

	// Get organization info
	var org OrganizationInfo
	err = database.DB.QueryRow(`
		SELECT org_id, org_code, name
		FROM organization
		WHERE org_id = $1
	`, orgID).Scan(&org.OrgID, &org.OrgCode, &org.Name)

	if err != nil {
		response.InternalError(c, "failed to get organization info", err)
		return
	}

	// Get stats
	var stats Stats
	database.DB.QueryRow(`SELECT COUNT(*) FROM github_repository WHERE org_id = $1`, orgID).Scan(&stats.TotalRepos)
	database.DB.QueryRow(`
		SELECT COUNT(*) FROM github_files gf
		JOIN github_repository gr ON gf.repo_id = gr.repo_id
		WHERE gr.org_id = $1
	`, orgID).Scan(&stats.TotalFiles)
	database.DB.QueryRow(`SELECT COUNT(*) FROM kubernetes_resource WHERE org_id = $1`, orgID).Scan(&stats.K8sResources)
	database.DB.QueryRow(`SELECT COUNT(*) FROM findings WHERE org_id = $1`, orgID).Scan(&stats.TotalFindings)

	dashboardResp := DashboardResponse{
		User:         user,
		Organization: org,
		Stats:        stats,
	}

	response.Success(c, http.StatusOK, "dashboard data retrieved successfully", dashboardResp)
}
