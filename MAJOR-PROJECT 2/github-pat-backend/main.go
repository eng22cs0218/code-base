package main

import (
	"log"
	"os"
	"time"

	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/logger"
	"github-pat-backend/services/authentication"
	fetchingservice "github-pat-backend/services/fetching-service"
	kubeagentservice "github-pat-backend/services/kube-agent-service"
	securityservice "github-pat-backend/services/security-service"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	_ = godotenv.Load()

	// Get configuration from environment
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	serverPort := os.Getenv("PORT")
	frontendURL := os.Getenv("FRONTEND_URL")

	if dbHost == "" || dbPort == "" || dbUser == "" || dbPass == "" || dbName == "" {
		log.Fatal("❌ Database environment variables are not set")
	}

	if serverPort == "" {
		serverPort = "8080"
	}

	if frontendURL == "" {
		frontendURL = "http://localhost:8081"
	}

	// Initialize database
	if err := database.InitDB(dbHost, dbPort, dbUser, dbPass, dbName); err != nil {
		log.Fatal("❌ Failed to connect to database:", err)
	}

	// Initialize logger
	appLogger := logger.New("aegios-backend")

	// Initialize Gin router
	router := gin.Default()

	// CORS Middleware
	router.Use(corsMiddleware(frontendURL))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"success": true,
			"message": "Aegios Backend is running",
			"data": gin.H{
				"status":    "healthy",
				"timestamp": time.Now().Format(time.RFC3339),
				"services": []string{
					"authentication",
					"fetching-service",
					"security-service",
				},
			},
		})
	})

	// Register service routes
	authGroup := router.Group("/authentication")
	authentication.RegisterRoutes(authGroup)

	fetchGroup := router.Group("/fetching-service")
	fetchingservice.RegisterRoutes(fetchGroup)

	securityGroup := router.Group("/security-service")
	securityservice.RegisterRoutes(securityGroup)

	sessionGroup := router.Group("/session")
	kubeagentservice.RegisterRoutes(sessionGroup)

	// Phase 2: global route for config upload via Bearer token (outside /session group)
	router.POST("/api/upload-config", kubeagentservice.UploadConfigPhase2Handler)

	// Start server
	appLogger.Info("🚀 Aegios Backend starting...")
	appLogger.Info("📍 Server running on http://localhost:" + serverPort)
	appLogger.Info("🔗 Frontend URL: " + frontendURL)
	appLogger.Info("📊 Services:")
	appLogger.Info("   - Authentication: /authentication/*")
	appLogger.Info("   - Fetching Service: /fetching-service/*")
	appLogger.Info("   - Security Service: /security-service/*")
	appLogger.Info("   - Session (Kube Agent): /session/*")

	if err := router.Run(":" + serverPort); err != nil {
		log.Fatal("❌ Server failed to start:", err)
	}
}

// corsMiddleware handles CORS
func corsMiddleware(frontendURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Max-Age", "3600")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}

		c.Next()
	}
}
