package authentication

import (
	"net/http"

	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type SignoutRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
}

// Signout handles user logout
func Signout(c *gin.Context) {
	var req SignoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "session_token is required", err)
		return
	}

	// Validate and delete session
	result, err := database.DB.Exec(`
		DELETE FROM user_sessions
		WHERE session_token = $1 AND expires_at > NOW()
	`, req.SessionToken)

	if err != nil {
		response.InternalError(c, "failed to signout", err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		response.Unauthorized(c, "invalid or expired session", nil)
		return
	}

	response.Success(c, http.StatusOK, "signout successful", map[string]string{
		"status":   "success",
		"redirect": "/authentication/login",
	})
}
