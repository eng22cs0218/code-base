package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// StandardResponse represents the standard API response format
type StandardResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Success sends a successful response
func Success(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, StandardResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Error sends an error response
func Error(c *gin.Context, statusCode int, message string, err error) {
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}

	c.JSON(statusCode, StandardResponse{
		Success: false,
		Message: message,
		Error:   errorMsg,
	})
}

// BadRequest sends a 400 error
func BadRequest(c *gin.Context, message string, err error) {
	Error(c, http.StatusBadRequest, message, err) //
}

// Unauthorized sends a 401 error
func Unauthorized(c *gin.Context, message string, err error) {
	Error(c, http.StatusUnauthorized, message, err) //
}

// InternalError sends a 500 error
func InternalError(c *gin.Context, message string, err error) {
	Error(c, http.StatusInternalServerError, message, err) //
}
