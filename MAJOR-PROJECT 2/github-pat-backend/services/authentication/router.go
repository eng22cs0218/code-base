package authentication

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers authentication service routes
func RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/login", Login)
	router.POST("/signup", Signup)
	router.POST("/signout", Signout)
}
