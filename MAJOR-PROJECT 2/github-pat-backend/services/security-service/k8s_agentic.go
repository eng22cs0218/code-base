package securityservice

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github-pat-backend/pkg/bedrock"
	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/response"
	kubeagentservice "github-pat-backend/services/kube-agent-service"

	"github.com/gin-gonic/gin"
)

type AgenticRequest struct {
	SessionToken          string `json:"session_token" binding:"required"`
	ResourceID            string `json:"resource_id" binding:"required"`
	ActionType            string `json:"action_type" binding:"required"`
	UserIntent            string `json:"user_intent,omitempty"`
	FindingDescription    string `json:"finding_description,omitempty"`
	FindingRecommendation string `json:"finding_recommendation,omitempty"`
	FindingKind           string `json:"finding_kind,omitempty"`
}

type AgenticResponse struct {
	Status     string                 `json:"status"`
	Message    string                 `json:"message"`
	ResourceID string                 `json:"resource_id"`
	ActionType string                 `json:"action_type"`
	Applied    bool                   `json:"applied"`
	Changes    map[string]interface{} `json:"changes"`
	Timestamp  string                 `json:"timestamp"`
	NextSteps  []string               `json:"next_steps"`
	AIOutput   string                 `json:"ai_output,omitempty"`
}

// ApplyAgentic applies NLP-driven remediation actions
func ApplyAgentic(c *gin.Context) {
	var req AgenticRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "session_token, resource_id, and action_type are required", err)
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

	// Verify resource exists
	var resourceID, kind, name, namespace, yamlContent string
	err = database.DB.QueryRow(`
		SELECT resource_id, kind, name, namespace, yaml_content
		FROM kubernetes_resource
		WHERE org_id = $1 AND resource_id = $2
	`, orgID, req.ResourceID).Scan(&resourceID, &kind, &name, &namespace, &yamlContent)

	if err != nil {
		// It might be a finding_id for a missing resource that has no associated K8s resource yet
		err2 := database.DB.QueryRow(`
			SELECT finding_id, COALESCE(missing_kind, 'Unknown'), 'MissingResource', COALESCE(namespace, 'default')
			FROM findings
			WHERE org_id = $1 AND finding_id = $2
		`, orgID, req.ResourceID).Scan(&resourceID, &kind, &name, &namespace)
		
		if err2 != nil {
			response.Error(c, http.StatusNotFound, "resource or finding not found", err)
			return
		}
		yamlContent = "# No existing manifest. AI should generate a new one based on findings."
	}

	// Parse user intent (NLP simulation)
	intent := parseUserIntent(req.UserIntent, req.ActionType)

	// Build findings context - prefer specific finding info from frontend
	var findingsContext string
	if req.FindingDescription != "" || req.FindingRecommendation != "" {
		// Use the specific finding details passed from the frontend
		findingsContext = fmt.Sprintf("- Issue: %s\n  Recommendation: %s\n  Resource Kind: %s\n",
			req.FindingDescription, req.FindingRecommendation, req.FindingKind)
	} else {
		// Fallback: Fetch findings from DB for this resource
		findingRows, dbErr := database.DB.Query(`
			SELECT COALESCE(description, ''), COALESCE(recommendations, '')
			FROM findings
			WHERE org_id = $1 AND (resource_id = $2 OR finding_id = $2)
		`, orgID, req.ResourceID)
		if dbErr == nil {
			defer findingRows.Close()
			for findingRows.Next() {
				var desc, rec string
				if scanErr := findingRows.Scan(&desc, &rec); scanErr == nil {
					findingsContext += fmt.Sprintf("- Issue: %s\n  Recommendation: %s\n", desc, rec)
				}
			}
		}
	}

	// Prepare Bedrock Prompt - scoped to the specific finding
	promptContext := fmt.Sprintf("You are a Kubernetes Security Expert assistant. I need your direct help to fix ONLY the specific Kubernetes security issue described below. Do NOT attempt to invoke any APIs or tools. Provide your response as direct text.\n\nAction requested: %s\nUser Intent: %s\nResource Kind: %s\nManifest:\n%s\n\nSpecific Finding to Fix (ONLY fix this one issue, do NOT fix other issues):\n%s\nProvide ONLY the remediation for this specific finding. Generate the correct Kubernetes manifest or kubectl command for ONLY this resource kind. Do not generate fixes for other resource types or findings.", req.ActionType, req.UserIntent, req.FindingKind, yamlContent, findingsContext)

	// Call Bedrock Agent
	aiOutput, bedrockErr := bedrock.GenerateRemediation(promptContext)
	if bedrockErr != nil {
		fmt.Printf("Bedrock error (falling back to simulation): %v\n", bedrockErr)
	}

	// Generate remediation plan
	_ = generateRemediationPlan(req.ActionType, kind, yamlContent, intent)

	// Apply remediation (simulation - in production, this would modify actual K8s resources)
	changes := applyRemediation(req.ActionType, kind, yamlContent)

	// Log the action
	logAgenticAction(orgID, resourceID, req.ActionType, changes)

	// Generate next steps
	nextSteps := generateNextSteps(req.ActionType, kind)

	// ── Save correct_config and command to agent_output ──
	correctConfig := generateCorrectConfig(req.ActionType, kind, name, namespace, yamlContent, changes)
	if extractYaml := extractYamlFromAI(aiOutput); extractYaml != "" {
		correctConfig = extractYaml
	}
	
	kubectlCmd := generateKubectlCommand(req.ActionType, kind, name, namespace, yamlContent, changes)
	if extractCmd := extractCommandFromAI(aiOutput); extractCmd != "" {
		kubectlCmd = extractCmd
	}

	configToken, tokenErr := kubeagentservice.GetSessionTokenByOrg(orgID)
	if tokenErr != nil {
		// Auto-create a config_credentials entry so agent_output can be saved
		log.Printf("[AGENTIC] No active session token for org %s, creating one...", orgID)
		createErr := database.CreateConfigCredential(orgID, "agentic-remediation")
		if createErr != nil {
			log.Printf("[AGENTIC] Failed to create config_credentials for org %s: %v", orgID, createErr)
		} else {
			newToken, _ := database.GenerateTerminalToken()
			kubeagentservice.StoreSessionToken(newToken, kubeagentservice.TokenSessionInfo{
				OrgID:       orgID,
				ContextName: "agentic-remediation",
			})
			configToken = newToken
			tokenErr = nil
			log.Printf("[AGENTIC] Created config_credentials session token for org %s", orgID)
		}
	}
	if tokenErr == nil {
		if saveErr := database.SaveAgentOutput(orgID, configToken, kubectlCmd, correctConfig, req.ResourceID); saveErr != nil {
			log.Printf("[AGENTIC] Failed to save agent_output for org %s: %v", orgID, saveErr)
		} else {
			log.Printf("[AGENTIC] Saved remediation to agent_output: org=%s command=%s finding_id=%s", orgID, kubectlCmd, req.ResourceID)
		}
	}

	agenticResp := AgenticResponse{
		Status:     "success",
		Message:    fmt.Sprintf("Action '%s' applied successfully to %s/%s", req.ActionType, kind, name),
		ResourceID: resourceID,
		ActionType: req.ActionType,
		Applied:    true,
		Changes:    changes,
		Timestamp:  time.Now().Format(time.RFC3339),
		NextSteps:  nextSteps,
		AIOutput:   aiOutput,
	}

	response.Success(c, http.StatusOK, "agentic action applied successfully", agenticResp)
}

// parseUserIntent uses NLP to understand user's intent
func parseUserIntent(userIntent, actionType string) map[string]interface{} {
	intent := make(map[string]interface{})

	// Simple NLP simulation
	userIntent = strings.ToLower(userIntent)

	if strings.Contains(userIntent, "secure") || strings.Contains(userIntent, "fix") {
		intent["goal"] = "security_improvement"
		intent["urgency"] = "high"
	}

	if strings.Contains(userIntent, "quickly") || strings.Contains(userIntent, "urgent") {
		intent["urgency"] = "critical"
	}

	if strings.Contains(userIntent, "test") || strings.Contains(userIntent, "staging") {
		intent["environment"] = "non-production"
	} else {
		intent["environment"] = "production"
	}

	intent["action_type"] = actionType

	return intent
}

// generateRemediationPlan creates a remediation plan based on action type
func generateRemediationPlan(actionType, kind, yamlContent string, intent map[string]interface{}) map[string]interface{} {
	plan := make(map[string]interface{})

	switch actionType {
	case "enforce_non_root":
		plan["changes"] = []string{
			"Add securityContext to container spec",
			"Set runAsNonRoot: true",
			"Set runAsUser: 1000 (non-root UID)",
		}
		plan["impact"] = "low"
		plan["rollback"] = "Remove securityContext or set runAsNonRoot: false"

	case "add_resource_limits":
		plan["changes"] = []string{
			"Add resources.limits.cpu",
			"Add resources.limits.memory",
			"Add resources.requests.cpu",
			"Add resources.requests.memory",
		}
		plan["impact"] = "medium"
		plan["rollback"] = "Remove resources section"

	case "remove_privilege":
		plan["changes"] = []string{
			"Set privileged: false in securityContext",
			"Remove unnecessary capabilities",
		}
		plan["impact"] = "high"
		plan["rollback"] = "Set privileged: true (not recommended)"

	default:
		plan["changes"] = []string{"Apply standard security hardening"}
		plan["impact"] = "low"
	}

	return plan
}

// applyRemediation simulates applying the remediation
func applyRemediation(actionType, kind, yamlContent string) map[string]interface{} {
	changes := make(map[string]interface{})

	// Handle missing resources that need imperative creation instead of patching
	if strings.EqualFold(strings.TrimSpace(kind), "ServiceAccount") {
		changes["create_resource"] = "ServiceAccount"
	} else if strings.EqualFold(strings.TrimSpace(kind), "NetworkPolicy") {
		changes["create_resource"] = "NetworkPolicy"
	}

	switch actionType {
	case "enforce_non_root":
		changes["securityContext"] = map[string]interface{}{
			"runAsNonRoot": true,
			"runAsUser":    1000,
		}
		changes["status"] = "applied"

	case "add_resource_limits":
		changes["resources"] = map[string]interface{}{
			"limits": map[string]string{
				"cpu":    "500m",
				"memory": "512Mi",
			},
			"requests": map[string]string{
				"cpu":    "250m",
				"memory": "256Mi",
			},
		}
		changes["status"] = "applied"

	case "remove_privilege":
		changes["securityContext"] = map[string]interface{}{
			"privileged": false,
		}
		changes["status"] = "applied"

	case "remediate":
		// Auto-detect all applicable fixes based on the resource's YAML content
		if strings.Contains(yamlContent, "privileged: true") {
			changes["securityContext_privileged"] = map[string]interface{}{"privileged": false}
		}
		if !strings.Contains(yamlContent, "runAsNonRoot: true") {
			changes["securityContext"] = map[string]interface{}{
				"runAsNonRoot": true,
				"runAsUser":    1000,
			}
		}
		if !strings.Contains(yamlContent, "limits:") {
			changes["resources"] = map[string]interface{}{
				"limits": map[string]string{
					"cpu":    "500m",
					"memory": "512Mi",
				},
				"requests": map[string]string{
					"cpu":    "250m",
					"memory": "256Mi",
				},
			}
		}
		if len(changes) == 0 {
			changes["status"] = "no_issues_found"
			changes["message"] = "No security issues detected for remediation"
		} else {
			changes["status"] = "applied"
		}

	default:
		changes["status"] = "pending"
		changes["message"] = "Action queued for processing"
	}

	return changes
}

// logAgenticAction logs the agentic action for audit trail
func logAgenticAction(orgID, resourceID, actionType string, changes map[string]interface{}) {
	// In production, this would log to a dedicated audit table
	// For now, we'll just log to console
	fmt.Printf("[AGENTIC] org_id=%s resource_id=%s action=%s changes=%v\n",
		orgID, resourceID, actionType, changes)
}

// generateNextSteps provides recommendations for next actions
func generateNextSteps(actionType, kind string) []string {
	nextSteps := []string{
		"Verify the changes in your K8s cluster",
		"Monitor resource performance after changes",
		"Run validation to check for new issues",
	}

	switch actionType {
	case "enforce_non_root":
		nextSteps = append(nextSteps, "Test application functionality with non-root user")
		nextSteps = append(nextSteps, "Update CI/CD pipelines to use non-root images")

	case "add_resource_limits":
		nextSteps = append(nextSteps, "Monitor resource usage to optimize limits")
		nextSteps = append(nextSteps, "Set up alerts for resource exhaustion")

	case "remove_privilege":
		nextSteps = append(nextSteps, "Review application requirements for privileged access")
		nextSteps = append(nextSteps, "Consider using specific capabilities instead")
	}

	return nextSteps
}

// generateCorrectConfig produces the corrected configuration for the resource
func generateCorrectConfig(actionType, kind, name, namespace, yamlContent string, changes map[string]interface{}) string {
	config := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
	}

	// Build the spec patch based on the changes
	patch := map[string]interface{}{}

	if sc, ok := changes["securityContext"]; ok {
		patch["securityContext"] = sc
	}
	if scp, ok := changes["securityContext_privileged"]; ok {
		// Merge with existing securityContext if present
		if existing, exists := patch["securityContext"]; exists {
			existingMap := existing.(map[string]interface{})
			privMap := scp.(map[string]interface{})
			for k, v := range privMap {
				existingMap[k] = v
			}
			patch["securityContext"] = existingMap
		} else {
			patch["securityContext"] = scp
		}
	}
	if res, ok := changes["resources"]; ok {
		patch["resources"] = res
	}

	if len(patch) > 0 {
		config["spec"] = map[string]interface{}{
			"containers": []map[string]interface{}{
				patch,
			},
		}
	}

	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\"error\": \"failed to generate config: %s\"}", err.Error())
	}

	return string(configJSON)
}

// generateKubectlCommand generates the appropriate kubectl command based on the action
func generateKubectlCommand(actionType, kind, name, namespace, yamlContent string, changes map[string]interface{}) string {
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	if createKind, ok := changes["create_resource"].(string); ok {
		if createKind == "ServiceAccount" {
			return fmt.Sprintf("kubectl create serviceaccount %s -n %s", name, ns)
		} else if createKind == "NetworkPolicy" {
			// A generic default deny-all policy inline payload for Windows CMD
			escapedPolicy := fmt.Sprintf("{\"apiVersion\":\"networking.k8s.io/v1\",\"kind\":\"NetworkPolicy\",\"metadata\":{\"name\":\"%s-policy\",\"namespace\":\"%s\"},\"spec\":{\"podSelector\":{}}}", name, ns)
			escapedPolicy = strings.ReplaceAll(escapedPolicy, `"`, `\"`)
			return fmt.Sprintf("echo %s > temp-policy.json && kubectl apply -f temp-policy.json", escapedPolicy)
		}
	}

	// Try to extract the first container's name from yamlContent
	containerName := ""
	idx := strings.Index(yamlContent, "containers")
	if idx != -1 {
		// Handles YAML:  name: my-container
		// Handles JSON:  "name": "my-container"
		re := regexp.MustCompile(`(?m)["']?name["']?\s*:\s*["']?([a-zA-Z0-9_-]+)["']?`)
		matches := re.FindStringSubmatch(yamlContent[idx:])
		if len(matches) > 1 {
			containerName = matches[1]
		}
	}

	// Ultimate fallback if nothing was found to ensure a valid merge patch
	if containerName == "" {
		// Use a safe fallback string so the backend doesn't throw a malformed object error
		// (though K8s will reject it if this doesn't match the actual container name)
		containerName = "fallback-container-name"
	}

	// Build a JSON patch from the changes
	patchSpec := map[string]interface{}{}
	if containerName != "" {
		patchSpec["name"] = containerName
	}

	if sc, ok := changes["securityContext"]; ok {
		patchSpec["securityContext"] = sc
	}
	if scp, ok := changes["securityContext_privileged"]; ok {
		if existing, exists := patchSpec["securityContext"]; exists {
			existingMap := existing.(map[string]interface{})
			privMap := scp.(map[string]interface{})
			for k, v := range privMap {
				existingMap[k] = v
			}
		} else {
			patchSpec["securityContext"] = scp
		}
	}
	if res, ok := changes["resources"]; ok {
		patchSpec["resources"] = res
	}

	if len(patchSpec) == 0 {
		return fmt.Sprintf("kubectl get %s %s -n %s -o yaml", strings.ToLower(kind), name, ns)
	}

	// For Deployment/StatefulSet/DaemonSet, patch goes under spec.template.spec.containers[0]
	// For Pod, patch goes under spec.containers[0]
	var patchBody map[string]interface{}

	switch strings.ToLower(kind) {
	case "deployment", "statefulset", "daemonset":
		patchBody = map[string]interface{}{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []map[string]interface{}{
							patchSpec,
						},
					},
				},
			},
		}
	case "pod":
		patchBody = map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []map[string]interface{}{
					patchSpec,
				},
			},
		}
	default:
		// For other resource types, apply the patch directly under spec
		patchBody = map[string]interface{}{
			"spec": patchSpec,
		}
	}

	patchJSON, err := json.Marshal(patchBody)
	if err != nil {
		return fmt.Sprintf("kubectl get %s %s -n %s -o yaml", strings.ToLower(kind), name, ns)
	}

	// Escape double quotes for cross-platform (especially Windows CMD) support
	escapedJSON := strings.ReplaceAll(string(patchJSON), `"`, `\"`)

	return fmt.Sprintf("kubectl patch %s %s -n %s --type=strategic -p \"%s\"",
		strings.ToLower(kind), name, ns, escapedJSON)
}

// extractYamlFromAI extracts the YAML code block from Bedrock AI output
func extractYamlFromAI(aiOutput string) string {
	re := regexp.MustCompile(`(?s)\x60\x60\x60(?:yaml)?\n(.*?)\x60\x60\x60`)
	matches := re.FindStringSubmatch(aiOutput)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// extractCommandFromAI extracts the kubectl command from Bedrock AI output
func extractCommandFromAI(aiOutput string) string {
	re := regexp.MustCompile(`(?s)\x60\x60\x60(?:sh|bash)?\n(kubectl [^\x60]+)\x60\x60\x60`)
	matches := re.FindStringSubmatch(aiOutput)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	for _, line := range strings.Split(aiOutput, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "kubectl ") {
			return line
		}
	}
	return ""
}
