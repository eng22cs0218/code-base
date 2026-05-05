package securityservice

import (
	"net/http"

	"github-pat-backend/pkg/analysis"
	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type PostureRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
}

type PostureResponse struct {
	TotalResources int                      `json:"total_resources"`
	TotalIssues    int                      `json:"total_issues"`
	Critical       int                      `json:"critical"`
	High           int                      `json:"high"`
	Medium         int                      `json:"medium"`
	Low            int                      `json:"low"`
	Resources      []map[string]interface{} `json:"resources"`
}

// GetPosture analyzes security posture of K8s resources
func GetPosture(c *gin.Context) {
	var req PostureRequest
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

	// Get user's org_id
	orgID, err := database.GetUserOrgID(githubUsername)
	if err != nil {
		response.InternalError(c, "failed to get user organization", err)
		return
	}

	// Get all K8s resources for posture analysis (filtered by org_id)
	rows, err := database.DB.Query(`
		SELECT kr.id, kr.resource_id, kr.kind, kr.name, kr.namespace, kr.yaml_content,
		       gf.file_path, gr.repo_name
		FROM kubernetes_resource kr
		JOIN github_files gf ON kr.repo_file_id = gf.id
		JOIN github_repository gr ON gf.repo_id = gr.repo_id
		WHERE kr.org_id = $1
		ORDER BY kr.kind, kr.name
	`, orgID)

	if err != nil {
		response.InternalError(c, "failed to fetch resources", err)
		return
	}
	defer rows.Close()

	var resources []map[string]interface{}
	postureIssues := 0
	criticalIssues := 0
	highIssues := 0
	mediumIssues := 0
	lowIssues := 0

	for rows.Next() {
		var id int
		var resourceID, kind, name, namespace, filePath, repoName string
		var yamlContent *string // Use pointer to handle NULL

		if err := rows.Scan(&id, &resourceID, &kind, &name, &namespace, &yamlContent, &filePath, &repoName); err != nil {
			continue
		}

		// Convert pointer to string
		yamlStr := ""
		if yamlContent != nil {
			yamlStr = *yamlContent
		}

		// Analyze posture using analysis package
		issues := analysis.AnalyzeResourcePosture(kind, yamlStr)
		severity := analysis.CalculateSeverity(issues)

		if len(issues) > 0 {
			postureIssues++
			switch severity {
			case "critical":
				criticalIssues++
			case "high":
				highIssues++
			case "medium":
				mediumIssues++
			case "low":
				lowIssues++
			}
		}

		resources = append(resources, map[string]interface{}{
			"id":          id,
			"resource_id": resourceID,
			"kind":        kind,
			"name":        name,
			"namespace":   namespace,
			"file_path":   filePath,
			"repo_name":   repoName,
			"issues":      issues,
			"severity":    severity,
		})
	}

	postureResp := PostureResponse{
		TotalResources: len(resources),
		TotalIssues:    postureIssues,
		Critical:       criticalIssues,
		High:           highIssues,
		Medium:         mediumIssues,
		Low:            lowIssues,
		Resources:      resources,
	}

	response.Success(c, http.StatusOK, "security posture analyzed successfully", postureResp)
}
