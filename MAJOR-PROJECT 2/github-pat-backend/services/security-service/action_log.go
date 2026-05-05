package securityservice

import (
	"database/sql"
	"net/http"
	"time"

	"github-pat-backend/pkg/database"

	"github.com/gin-gonic/gin"
)

type AddActionLogRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
	FindingID    string `json:"finding_id" binding:"required"`
	Command      string `json:"command" binding:"required"`
}

type GetActionLogsRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
	FindingID    string `json:"finding_id" binding:"required"`
}

type ActionLog struct {
	ID           int       `json:"id"`
	SessionToken string    `json:"session_token"`
	FindingID    string    `json:"finding_id"`
	Command      string    `json:"command"`
	CreatedAt    time.Time `json:"created_at"`
}

func AddActionLog(c *gin.Context) {
	var req AddActionLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request"})
		return
	}

	query := `INSERT INTO action_logs (session_token, finding_id, command) VALUES ($1, $2, $3)`
	_, err := database.DB.Exec(query, req.SessionToken, req.FindingID, req.Command)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to store action log"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Action logged successfully"})
}

func GetActionLogs(c *gin.Context) {
	var req GetActionLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request"})
		return
	}

	query := `SELECT id, session_token, finding_id, command, created_at FROM action_logs WHERE finding_id = $1 ORDER BY created_at DESC`
	rows, err := database.DB.Query(query, req.FindingID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": []ActionLog{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch action logs"})
		return
	}
	defer rows.Close()

	var logs []ActionLog
	for rows.Next() {
		var log ActionLog
		if err := rows.Scan(&log.ID, &log.SessionToken, &log.FindingID, &log.Command, &log.CreatedAt); err == nil {
			logs = append(logs, log)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": logs})
}
