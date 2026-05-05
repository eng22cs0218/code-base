package kubeagentservice

import "github.com/gin-gonic/gin"

func RegisterRoutes(router *gin.RouterGroup) {
	// Config & command generation
	router.POST("/generate-command", GenerateCommandHandler)
	router.POST("/upload", UploadConfigHandler)
	router.GET("/config-status", ConfigStatusHandler)
	router.GET("/cluster-mode", ClusterModeHandler)

	// Frontend terminal WebSocket
	router.GET("/ws", WsHandler)

	// Agent WebSocket (primary — real-time)
	router.GET("/agent-ws", AgentWsHandler)

	// Agent HTTP fallback (if WebSocket is unavailable)
	router.GET("/agent-poll", AgentPollHandler)
	router.POST("/agent-result", AgentResultHandler)

	// Remediation actions
	router.POST("/take-action", TakeActionHandler)
	router.GET("/action-status", GetActionStatusHandler)
	router.GET("/remediation-status", RemediationStatusHandler)

	// Script serving
	router.GET("/agent-script", AgentScriptHandler)
	router.GET("/agent-script-py", AgentScriptPyHandler)

	// Phase 2 endpoints (additive — existing routes above are untouched)
	router.POST("/init", SessionInitHandler)
	router.GET("/status", SessionStatusHandler)
	router.GET("/agent-script-v2", AgentScriptV2Handler)
}
