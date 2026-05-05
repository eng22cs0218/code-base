package securityservice

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers security service routes
func RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/k8s-score", GetScore)
	router.POST("/k8s-posture", GetPosture)
	router.POST("/k8s-posture-findings", GetPostureFindings)
	router.POST("/k8s-action", GetActions)
	router.POST("/k8s-agentic", ApplyAgentic)
	router.POST("/validate-namespaces", ValidateNamespaces)
	router.POST("/raise-pr", RaisePR)
	router.POST("/list-branches", ListBranches)
}
