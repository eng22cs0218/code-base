package fetchingservice

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers fetching service routes
func RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/dashboard", Dashboard)
	router.POST("/fetch-data", FetchDataOptimized) // Use optimized parallel version
	router.POST("/rendering", Render)
}
