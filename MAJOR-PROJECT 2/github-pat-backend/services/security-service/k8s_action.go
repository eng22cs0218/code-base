package securityservice

import (
	"net/http"

	"github-pat-backend/pkg/analysis"
	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type ActionRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
	ResourceID   string `json:"resource_id,omitempty"`
}

type ActionResponse struct {
	TotalActions int                      `json:"total_actions"`
	Actions      []map[string]interface{} `json:"actions"`
}

// GetActions retrieves recommended actions for K8s resources
func GetActions(c *gin.Context) {
	var req ActionRequest
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

	var query string
	var args []interface{}

	if req.ResourceID != "" {
		// Get actions for specific resource
		query = `SELECT kr.id, kr.resource_id, kr.kind, kr.name, kr.namespace, kr.yaml_content
		         FROM kubernetes_resource kr
		         WHERE kr.org_id = $1 AND kr.resource_id = $2`
		args = []interface{}{orgID, req.ResourceID}
	} else {
		// Get actions for all resources
		query = `SELECT kr.id, kr.resource_id, kr.kind, kr.name, kr.namespace, kr.yaml_content
		         FROM kubernetes_resource kr
		         WHERE kr.org_id = $1
		         ORDER BY kr.kind, kr.name`
		args = []interface{}{orgID}
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		response.InternalError(c, "failed to fetch resources", err)
		return
	}
	defer rows.Close()

	var actions []map[string]interface{}

	for rows.Next() {
		var id int
		var resourceID, kind, name, namespace string
		var yamlContent *string // Use pointer to handle NULL

		if err := rows.Scan(&id, &resourceID, &kind, &name, &namespace, &yamlContent); err != nil {
			continue
		}

		// Convert pointer to string
		yamlStr := ""
		if yamlContent != nil {
			yamlStr = *yamlContent
		}

		// Generate recommended actions using analysis package
		recommendedActions := analysis.GenerateActions(kind, name, yamlStr)

		if len(recommendedActions) > 0 {
			actions = append(actions, map[string]interface{}{
				"resource_id": resourceID,
				"kind":        kind,
				"name":        name,
				"namespace":   namespace,
				"actions":     recommendedActions,
			})
		}
	}

	actionResp := ActionResponse{
		TotalActions: len(actions),
		Actions:      actions,
	}

	response.Success(c, http.StatusOK, "actions retrieved successfully", actionResp)
}
