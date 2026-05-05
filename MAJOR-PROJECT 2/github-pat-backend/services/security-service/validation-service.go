package securityservice

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ValidationRequest represents the request payload for namespace validation
type ValidationRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
}

// Resource represents a Kubernetes resource with minimal fields
type Resource struct {
	Kind        string                 `json:"kind"`
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	YAMLContent map[string]interface{} `json:"yaml_content"`
}

// ValidationResult represents the validation result for a namespace
type ValidationResult struct {
	Namespace    string   `json:"namespace"`
	MissingKinds []string `json:"missing_kinds"`
}

// KindMap maps Kubernetes kinds to their resources
type KindMap map[string][]Resource

// RequiredKinds defines the required Kubernetes kinds for validation
var RequiredKinds = []string{
	"Role",
	"ClusterRole",
	"RoleBinding",
	"ClusterRoleBinding",
	"ServiceAccount",
	"Deployment",
	"Secret",
	"Service",
	"NetworkPolicy",
	"ConfigMap",
}

// ValidateNamespaces is the main handler for namespace-level Kubernetes validation
// this is main function
func ValidateNamespaces(c *gin.Context) {
	var req ValidationRequest
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

	// Clear previous validation findings before running a new validation cycle.
	// This keeps results idempotent while preserving findings generated during this run.
	if err := database.DeleteValidationFindingsByOrgID(orgID); err != nil {
		fmt.Printf("⚠️ Failed to delete old validation findings: %v\n", err)
	}

	// Use database time as a stable cutoff to keep findings created during this run.
	validationStartedAt, err := getCurrentDatabaseTime()
	if err != nil {
		response.InternalError(c, "failed to initialize validation window", err)
		return
	}

	// Execute validation workflow
	validationResults, err := validateResources(orgID)
	if err != nil {
		response.InternalError(c, "validation failed", err)
		return
	}

	// Record findings for missing kinds
	totalFindings, err := recordValidationFindings(orgID, validationResults, validationStartedAt)
	if err != nil {
		fmt.Printf("⚠️ Failed to record some findings: %v\n", err)
	}

	response.Success(c, http.StatusOK, "namespace validation completed", gin.H{
		"org_id":             orgID,
		"total_namespaces":   len(validationResults),
		"validation_results": validationResults,
		"findings_recorded":  totalFindings,
	})
}

func getCurrentDatabaseTime() (time.Time, error) {
	var currentTime time.Time
	err := database.DB.QueryRow(`SELECT NOW()`).Scan(&currentTime)
	if err != nil {
		return time.Time{}, err
	}

	return currentTime, nil
}

// validateResources orchestrates the entire validation workflow
func validateResources(orgID string) ([]ValidationResult, error) {
	// STEP 1: Fetch all resources in a single query (optimized)
	resources, err := getAllResourcesByOrgID(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch resources: %w", err)
	}

	if len(resources) == 0 {
		return []ValidationResult{}, nil
	}

	// STEP 2: Build namespace map (group resources by namespace)
	namespaceMap := buildNamespaceMap(resources)

	// STEP 3: Validate each namespace
	var results []ValidationResult
	for namespace, nsResources := range namespaceMap {
		// Build kind map for this namespace
		kindMap := buildKindMap(nsResources)

		// Check for missing kinds
		missingKinds := checkMissingKinds(kindMap)

		// NEW: Run security checks (resource misconfiguration)
		runSecurityChecks(orgID, namespace, kindMap)

		results = append(results, ValidationResult{
			Namespace:    namespace,
			MissingKinds: missingKinds,
		})
	}

	return results, nil
}

// getAllResourcesByOrgID fetches all resources for an organization in a single query
func getAllResourcesByOrgID(orgID string) ([]Resource, error) {
	query := `
		SELECT kind, name, namespace, yaml_content
		FROM kubernetes_resource 
		WHERE org_id = $1
		ORDER BY namespace, kind, name
	`

	rows, err := database.DB.Query(query, orgID)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	var resources []Resource
	for rows.Next() {
		var resource Resource
		var namespace *string // Handle NULL namespace
		var yamlContentJSON []byte

		if err := rows.Scan(&resource.Kind, &resource.Name, &namespace, &yamlContentJSON); err != nil {
			continue
		}

		if len(yamlContentJSON) > 0 {
			var yamlContent map[string]interface{}
			if err := json.Unmarshal(yamlContentJSON, &yamlContent); err != nil {
				fmt.Printf("⚠️ Failed to unmarshal YAML for %s/%s: %v\n", resource.Kind, resource.Name, err)
			} else {
				resource.YAMLContent = yamlContent
			}
		}

		// Handle NULL or empty namespace - default to "default"
		if namespace == nil || *namespace == "" {
			resource.Namespace = "default"
		} else {
			resource.Namespace = *namespace
		}

		resources = append(resources, resource)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return resources, nil
}

// getNamespaces fetches distinct namespaces for an organization
func getNamespaces(orgID string) ([]string, error) {
	query := `
		SELECT DISTINCT namespace 
		FROM kubernetes_resource 
		WHERE org_id = $1 
		ORDER BY namespace
	`

	rows, err := database.DB.Query(query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch namespaces: %w", err)
	}
	defer rows.Close()

	var namespaces []string
	for rows.Next() {
		var namespace *string
		if err := rows.Scan(&namespace); err != nil {
			continue
		}

		// Handle NULL namespace
		if namespace == nil || *namespace == "" {
			namespaces = append(namespaces, "default")
		} else {
			namespaces = append(namespaces, *namespace)
		}
	}

	return namespaces, nil
}

// getResourcesByNamespace fetches resources for a specific namespace
func getResourcesByNamespace(orgID, namespace string) ([]Resource, error) {
	query := `
		SELECT kind, name, namespace 
		FROM kubernetes_resource 
		WHERE org_id = $1 AND namespace = $2
		ORDER BY kind, name
	`

	rows, err := database.DB.Query(query, orgID, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch resources for namespace %s: %w", namespace, err)
	}
	defer rows.Close()

	var resources []Resource
	for rows.Next() {
		var resource Resource
		if err := rows.Scan(&resource.Kind, &resource.Name, &resource.Namespace); err != nil {
			continue
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

// buildNamespaceMap groups resources by namespace
func buildNamespaceMap(resources []Resource) map[string][]Resource {
	namespaceMap := make(map[string][]Resource)

	for _, resource := range resources {
		namespace := resource.Namespace
		if namespace == "" {
			namespace = "default"
		}
		namespaceMap[namespace] = append(namespaceMap[namespace], resource)
	}

	return namespaceMap
}

// buildKindMap groups resources by kind
func buildKindMap(resources []Resource) KindMap {
	kindMap := make(KindMap)

	for _, resource := range resources {
		kindMap[resource.Kind] = append(kindMap[resource.Kind], resource)

		normalizedKind := strings.ToLower(resource.Kind)
		if normalizedKind != resource.Kind {
			kindMap[normalizedKind] = append(kindMap[normalizedKind], resource)
		}
	}

	return kindMap
}

// checkMissingKinds returns the list of required kinds that are missing
func checkMissingKinds(kindMap KindMap) []string {
	var missingKinds []string

	for _, requiredKind := range RequiredKinds {
		if !hasKindInNamespace(kindMap, requiredKind) {
			missingKinds = append(missingKinds, requiredKind)
		}
	}

	return missingKinds
}

func hasKindInNamespace(kindMap KindMap, requiredKind string) bool {
	if resources, exists := kindMap[requiredKind]; exists && len(resources) > 0 {
		return true
	}

	lowerKind := strings.ToLower(requiredKind)
	if resources, exists := kindMap[lowerKind]; exists && len(resources) > 0 {
		return true
	}

	return false
}

// recordValidationFindings records missing kinds as findings in the database
func recordValidationFindings(orgID string, validationResults []ValidationResult, validationStartedAt time.Time) (int, error) {
	// Delete previous-run validation findings only, preserving findings generated in the current run.
	if err := deleteValidationFindingsBefore(orgID, validationStartedAt); err != nil {
		fmt.Printf("⚠️ Failed to delete old validation findings: %v\n", err)
	}

	totalFindings := 0

	for _, result := range validationResults {
		for _, missingKind := range result.MissingKinds {
			// Generate unique finding ID
			findingID := generateFindingID()

			// Determine severity based on the missing kind
			severity := getSeverityForMissingKind(missingKind)

			// Generate description
			description := fmt.Sprintf("Missing required Kubernetes kind '%s' in namespace '%s'. This resource type is required for proper security and operational compliance.", missingKind, result.Namespace)

			// Generate recommendations
			recommendations := getRecommendationsForMissingKind(missingKind, result.Namespace)

			// Create validation finding
			err := database.CreateValidationFinding(
				findingID,
				orgID,
				result.Namespace,
				missingKind,
				severity,
				description,
				recommendations,
			)

			if err != nil {
				fmt.Printf("❌ Failed to create finding for %s in %s: %v\n", missingKind, result.Namespace, err)
				continue
			}

			totalFindings++
			fmt.Printf("✅ Recorded finding: %s missing in %s (severity: %s)\n", missingKind, result.Namespace, severity)
		}
	}

	return totalFindings, nil
}

func deleteValidationFindingsBefore(orgID string, cutoff time.Time) error {
	_, err := database.DB.Exec(
		`DELETE FROM findings
		 WHERE org_id = $1
		   AND missing_kind IS NOT NULL
		   AND detected_at < $2`,
		orgID,
		cutoff,
	)

	return err
}

// generateFindingID generates a unique 6-digit finding ID
func generateFindingID() string {
	return database.GenerateUnique6DigitID()
}

// getSeverityForMissingKind determines the severity level for a missing Kubernetes kind
func getSeverityForMissingKind(kind string) string {
	// Critical security-related resources
	criticalKinds := map[string]bool{
		"Role":               true,
		"ClusterRole":        true,
		"RoleBinding":        true,
		"ClusterRoleBinding": true,
		"ServiceAccount":     true,
		"Secret":             true,
	}

	// High priority operational resources
	highKinds := map[string]bool{}

	if criticalKinds[kind] {
		return "critical"
	} else if highKinds[kind] {
		return "high"
	}

	return "low"
}

// getRecommendationsForMissingKind provides specific recommendations for each missing kind
func getRecommendationsForMissingKind(kind, namespace string) string {
	recommendations := map[string]string{
		"Role": fmt.Sprintf("Create a Role in namespace '%s' to define permissions for resources within the namespace. Roles are essential for implementing least-privilege access control.", namespace),

		"ClusterRole": fmt.Sprintf("Create a ClusterRole to define cluster-wide permissions. ClusterRoles are required for accessing cluster-scoped resources or for permissions that span multiple namespaces."),

		"RoleBinding": fmt.Sprintf("Create a RoleBinding in namespace '%s' to bind a Role to users, groups, or service accounts. RoleBindings are required to grant the permissions defined in Roles.", namespace),

		"ClusterRoleBinding": fmt.Sprintf("Create a ClusterRoleBinding to bind a ClusterRole to users, groups, or service accounts at the cluster level. This is required for granting cluster-wide permissions. "),

		"ServiceAccount": fmt.Sprintf("Create a ServiceAccount in namespace '%s' to provide an identity for processes running in Pods. ServiceAccounts are essential for pod-to-API-server authentication and authorization.", namespace),

		"Deployment": fmt.Sprintf("Create a Deployment in namespace '%s' to manage your application workloads. Deployments provide declarative updates for Pods and ReplicaSets.", namespace),

		"Secret": fmt.Sprintf("Create Secrets in namespace '%s' to store sensitive information such as passwords, tokens, and keys. Secrets should be used instead of storing sensitive data in Pod specifications or ConfigMaps.", namespace),

		"Service": fmt.Sprintf("Create a Service in namespace '%s' to expose your application workloads. Services provide stable networking endpoints for Pods.", namespace),
	}

	if rec, exists := recommendations[kind]; exists {
		return rec
	}

	return fmt.Sprintf("Create the missing '%s' resource in namespace '%s' according to your security and operational requirements.", kind, namespace)
}

// ============================================================================
// EXTENSION: Resource Misconfiguration Security Checks
// ============================================================================

// runSecurityChecks is the central security engine that orchestrates all security checks
func runSecurityChecks(orgID string, namespace string, kindMap KindMap) {
	// Run resource and secret misconfiguration checks.
	checkResourceMisconfiguration(orgID, namespace, kindMap)
	checkSecretMisconfiguration(orgID, namespace, kindMap)
	checkContainerSecurity(orgID, namespace, kindMap)
	checkServiceExposure(orgID, namespace, kindMap)
	checkNetworkPolicy(orgID, namespace, kindMap)
	checkRBACPermissions(orgID, namespace, kindMap)
}

// checkResourceMisconfiguration checks Deployment resources for resource configuration issues
func checkResourceMisconfiguration(orgID, namespace string, kindMap KindMap) {
	// STEP 3: Only process Deployment kind
	deployments, exists := kindMap["deployment"]
	if !exists || len(deployments) == 0 {
		return
	}

	// STEP 6: Loop through each deployment
	for _, deployment := range deployments {
		checkDeploymentContainers(orgID, namespace, deployment)
	}
}

// checkSecretMisconfiguration orchestrates all secret-related misconfiguration checks.
/*
	runSecurityChecks
    ↓
checkSecretMisconfiguration
    ↓
    ├── checkWorkloadSecrets
    │       ↓
    │   detect plain text env
    │
    ├── checkConfigMapSecrets
    │       ↓
    │   detect sensitive keys
    │
    └── checkSecretEncoding
            ↓
        detect invalid base64
                ↓
        createSecretMisconfigurationFinding
                ↓
        database.CreateValidationFinding
*/
func checkSecretMisconfiguration(orgID, namespace string, kindMap KindMap) {
	deploymentResources := kindMap["deployment"]
	daemonSetResources := kindMap["daemonset"]

	var workloadResources []Resource
	if len(deploymentResources) > 0 {
		workloadResources = append(workloadResources, deploymentResources...)
	}
	if len(daemonSetResources) > 0 {
		workloadResources = append(workloadResources, daemonSetResources...)
	}
	checkWorkloadSecrets(orgID, namespace, workloadResources)

	configMapResources := kindMap["configmap"]
	checkConfigMapSecrets(orgID, namespace, configMapResources)

	secretResources := kindMap["secret"]
	checkSecretEncoding(orgID, namespace, secretResources)
}

// checkContainerSecurity checks container-level security misconfigurations in workload resources.
/*
runSecurityChecks
    ↓
checkContainerSecurity
    ↓
getKindResources
    ↓
loop workloads (Deployment/DaemonSet)
    ↓
extract containers
    ↓
extract securityContext
    ↓
    ├── privileged == true → CRITICAL
    ├── runAsUser == 0 → HIGH
    └── allowPrivilegeEscalation == true → HIGH
            ↓
    createContainerSecurityFinding
            ↓
    database.CreateValidationFinding
*/
func checkContainerSecurity(orgID, namespace string, kindMap KindMap) {
	deployments := getKindResources(kindMap, "Deployment")
	daemonSets := getKindResources(kindMap, "DaemonSet")

	if len(deployments) == 0 && len(daemonSets) == 0 {
		return
	}

	workloadResources := make([]Resource, 0, len(deployments)+len(daemonSets))
	workloadResources = append(workloadResources, deployments...)
	workloadResources = append(workloadResources, daemonSets...)

	for _, resource := range workloadResources {
		spec, ok := resource.YAMLContent["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		template, ok := spec["template"].(map[string]interface{})
		if !ok {
			continue
		}

		templateSpec, ok := template["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		containersInterface, ok := templateSpec["containers"].([]interface{})
		if !ok {
			continue
		}

		for _, containerInterface := range containersInterface {
			container, ok := containerInterface.(map[string]interface{})
			if !ok {
				continue
			}

			containerName, _ := container["name"].(string)
			if containerName == "" {
				containerName = "unnamed"
			}

			securityContextInterface, exists := container["securityContext"]
			if !exists || securityContextInterface == nil {
				// Missing securityContext is itself a security issue
				description := fmt.Sprintf("Container '%s' in %s '%s' has no securityContext defined. This means no security restrictions are applied.", containerName, resource.Kind, resource.Name)
				recommendation := "Add a securityConftext with runAsNonRoot: true, allowPrivilegeEscalation: false, and readOnlyRootFilesystem: true."
				createContainerSecurityFinding(orgID, namespace, resource.Kind, "high", description, recommendation)
				continue
			}

			securityContext, ok := securityContextInterface.(map[string]interface{})
			if !ok {
				continue
			}

			// CONDITION 1: privileged == true
			if privileged, ok := securityContext["privileged"].(bool); ok && privileged {
				description := fmt.Sprintf("Container '%s' in %s '%s' is running in privileged mode, which grants full access to the host.", containerName, resource.Kind, resource.Name)
				recommendation := "Avoid using privileged: true. Use least privilege principle."
				createContainerSecurityFinding(orgID, namespace, resource.Kind, "critical", description, recommendation)
			}

			// CONDITION 2: runAsUser == 0
			if runAsUser, exists := securityContext["runAsUser"]; exists && isRootRunAsUser(runAsUser) {
				description := fmt.Sprintf("Container '%s' in %s '%s' is running as root user (UID 0), which is a security risk.", containerName, resource.Kind, resource.Name)
				recommendation := "Run containers as non-root user using runAsUser > 0."
				createContainerSecurityFinding(orgID, namespace, resource.Kind, "high", description, recommendation)
			}

			// CONDITION 3: allowPrivilegeEscalation == true
			if allowPrivilegeEscalation, ok := securityContext["allowPrivilegeEscalation"].(bool); ok && allowPrivilegeEscalation {
				description := fmt.Sprintf("Container '%s' in %s '%s' allows privilege escalation, which can lead to security vulnerabilities.", containerName, resource.Kind, resource.Name)
				recommendation := "Set allowPrivilegeEscalation: false to prevent privilege escalation."
				createContainerSecurityFinding(orgID, namespace, resource.Kind, "high", description, recommendation)
			}
		}
	}
}

func getKindResources(kindMap KindMap, kind string) []Resource {
	if resources, exists := kindMap[kind]; exists && len(resources) > 0 {
		return resources
	}

	normalizedKind := strings.ToLower(kind)
	if resources, exists := kindMap[normalizedKind]; exists && len(resources) > 0 {
		return resources
	}

	return nil
}

func isRootRunAsUser(runAsUser interface{}) bool {
	switch value := runAsUser.(type) {
	case int:
		return value == 0
	case int32:
		return value == 0
	case int64:
		return value == 0
	case float32:
		return value == 0
	case float64:
		return value == 0
	default:
		return false
	}
}

func createContainerSecurityFinding(orgID, namespace, kind, severity, description, recommendation string) {
	findingID := generateFindingID()

	err := database.CreateValidationFinding(
		findingID,
		orgID,
		namespace,
		kind,
		severity,
		description,
		recommendation,
	)

	if err != nil {
		fmt.Printf("❌ Failed to create container security finding for kind %s: %v\n", kind, err)
		return
	}

	fmt.Printf("✅ Recorded container security finding: %s (severity: %s)\n", kind, severity)
}

// checkServiceExposure checks Service exposure and routing misconfigurations.
/* runSecurityChecks
    ↓
checkServiceExposure
    ↓
extract services + deployments
    ↓
build:
    deploymentPorts
    deploymentLabels
    ↓
for each service:
    ├── NodePort → HIGH
    ├── No selector → MEDIUM
    ├── targetPort mismatch → HIGH
            ↓
    createServiceExposureFinding
            ↓
    database.CreateValidationFinding */
func checkServiceExposure(orgID, namespace string, kindMap KindMap) {
	services := getKindResources(kindMap, "Service")
	deployments := getKindResources(kindMap, "Deployment")

	if len(services) == 0 {
		return
	}

	deploymentPorts := make(map[string][]int)
	deploymentLabels := make(map[string]map[string]string)

	for _, deployment := range deployments {
		if deployment.Name == "" {
			continue
		}

		deploymentPorts[deployment.Name] = extractDeploymentPorts(deployment)
		deploymentLabels[deployment.Name] = extractDeploymentLabels(deployment)
	}

	for _, service := range services {
		spec, ok := service.YAMLContent["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		serviceType, _ := spec["type"].(string)
		servicePortNumbers := extractServicePorts(spec)
		if strings.EqualFold(serviceType, "NodePort") {
			description := fmt.Sprintf("Service '%s' is exposed using NodePort, which may expose internal services externally.", service.Name)
			recommendation := fmt.Sprintf("Service '%s' currently uses type=%s with service ports=%s. Use ClusterIP or LoadBalancer appropriately and avoid NodePort unless explicitly required.", service.Name, serviceType, formatIntSlice(servicePortNumbers))
			createServiceExposureFinding(orgID, namespace, "high", description, recommendation)
		}

		selector := toStringMap(spec["selector"])
		if len(selector) == 0 {
			description := fmt.Sprintf("Service '%s' does not define a selector, so it cannot route traffic to any pods.", service.Name)
			recommendation := fmt.Sprintf("Service '%s' selector is empty. Add a selector matching deployment labels. Current service ports: %s.", service.Name, formatIntSlice(servicePortNumbers))
			createServiceExposureFinding(orgID, namespace, "low", description, recommendation)
			continue
		}

		servicePorts, ok := spec["ports"].([]interface{})
		if !ok {
			continue
		}

		matchedPorts := collectMatchedDeploymentPorts(selector, deploymentPorts, deploymentLabels)

		for _, servicePortInterface := range servicePorts {
			servicePort, ok := servicePortInterface.(map[string]interface{})
			if !ok {
				continue
			}

			targetPortValue, hasTargetPort := servicePort["targetPort"]
			if !hasTargetPort || targetPortValue == nil {
				continue
			}

			targetPort, ok := toInt(targetPortValue)
			if !ok {
				// String targetPorts (named ports) are intentionally skipped safely.
				continue
			}

			if !containsPort(matchedPorts, targetPort) {
				description := fmt.Sprintf("Service '%s' targetPort '%d' does not match any containerPort in associated Deployment.", service.Name, targetPort)
				recommendation := fmt.Sprintf("Update Service '%s' targetPort from %d to one of deployment container ports: %s. Current service ports: %s.", service.Name, targetPort, formatIntSlice(matchedPorts), formatIntSlice(servicePortNumbers))
				createServiceExposureFinding(orgID, namespace, "high", description, recommendation)
			}
		}
	}
}

func extractDeploymentPorts(resource Resource) []int {
	spec, ok := resource.YAMLContent["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return nil
	}

	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	containers, ok := templateSpec["containers"].([]interface{})
	if !ok {
		return nil
	}

	portSet := make(map[int]struct{})
	for _, containerInterface := range containers {
		container, ok := containerInterface.(map[string]interface{})
		if !ok {
			continue
		}

		portsInterface, hasPorts := container["ports"]
		if !hasPorts || portsInterface == nil {
			continue
		}

		ports, ok := portsInterface.([]interface{})
		if !ok {
			continue
		}

		for _, portInterface := range ports {
			portMap, ok := portInterface.(map[string]interface{})
			if !ok {
				continue
			}

			containerPortValue, hasContainerPort := portMap["containerPort"]
			if !hasContainerPort || containerPortValue == nil {
				continue
			}

			containerPort, ok := toInt(containerPortValue)
			if !ok {
				continue
			}

			portSet[containerPort] = struct{}{}
		}
	}

	ports := make([]int, 0, len(portSet))
	for port := range portSet {
		ports = append(ports, port)
	}

	return ports
}

func extractDeploymentLabels(resource Resource) map[string]string {
	// Extract pod template labels (spec.template.metadata.labels)
	// because Service selectors match against pod labels, not deployment-level labels.
	spec, ok := resource.YAMLContent["spec"].(map[string]interface{})
	if !ok {
		return map[string]string{}
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return map[string]string{}
	}

	metadata, ok := template["metadata"].(map[string]interface{})
	if !ok {
		return map[string]string{}
	}

	return toStringMap(metadata["labels"])
}

func toStringMap(value interface{}) map[string]string {
	result := make(map[string]string)
	if value == nil {
		return result
	}

	valueMap, ok := value.(map[string]interface{})
	if !ok {
		return result
	}

	for key, raw := range valueMap {
		text, ok := raw.(string)
		if !ok {
			continue
		}
		result[key] = text
	}

	return result
}

func collectMatchedDeploymentPorts(selector map[string]string, deploymentPorts map[string][]int, deploymentLabels map[string]map[string]string) []int {
	portSet := make(map[int]struct{})

	for deploymentName, labels := range deploymentLabels {
		if !selectorMatches(selector, labels) {
			continue
		}

		for _, port := range deploymentPorts[deploymentName] {
			portSet[port] = struct{}{}
		}
	}

	ports := make([]int, 0, len(portSet))
	for port := range portSet {
		ports = append(ports, port)
	}

	return ports
}

func selectorMatches(selector, labels map[string]string) bool {
	if len(selector) == 0 || len(labels) == 0 {
		return false
	}

	for key, expectedValue := range selector {
		actualValue, exists := labels[key]
		if !exists || actualValue != expectedValue {
			return false
		}
	}

	return true
}

func containsPort(ports []int, target int) bool {
	for _, port := range ports {
		if port == target {
			return true
		}
	}

	return false
}

func extractServicePorts(spec map[string]interface{}) []int {
	portsValue, ok := spec["ports"].([]interface{})
	if !ok {
		return nil
	}

	portSet := make(map[int]struct{})
	for _, item := range portsValue {
		portMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if port, ok := toInt(portMap["port"]); ok {
			portSet[port] = struct{}{}
		}
		if nodePort, ok := toInt(portMap["nodePort"]); ok {
			portSet[nodePort] = struct{}{}
		}
	}

	ports := make([]int, 0, len(portSet))
	for port := range portSet {
		ports = append(ports, port)
	}

	return ports
}

func formatIntSlice(values []int) string {
	if len(values) == 0 {
		return "[]"
	}

	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}

	return "[" + strings.Join(parts, ",") + "]"
}

func toInt(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

func createServiceExposureFinding(orgID, namespace, severity, description, recommendation string) {
	findingID := generateFindingID()

	err := database.CreateValidationFinding(
		findingID,
		orgID,
		namespace,
		"Service",
		severity,
		description,
		recommendation,
	)

	if err != nil {
		fmt.Printf("❌ Failed to create service exposure finding: %v\n", err)
		return
	}

	fmt.Printf("✅ Recorded service exposure finding (severity: %s)\n", severity)
}

// checkNetworkPolicy checks NetworkPolicy ingress/egress rules for open traffic.
/*
runSecurityChecks
    ↓
checkNetworkPolicy
    ↓
get NetworkPolicy resources
    ↓
for each policy:
    ↓
    extract spec
    ↓
    ├── ingress check
    │       └── open → HIGH
    │
    └── egress check
            └── open → HIGH
                    ↓
        createNetworkPolicyFinding
                    ↓
        database.CreateValidationFinding
*/

func checkNetworkPolicy(orgID, namespace string, kindMap KindMap) {
	policies := kindMap["NetworkPolicy"]
	if len(policies) == 0 {
		policies = kindMap["networkpolicy"]
	}

	if len(policies) == 0 {
		return
	}

	for _, policy := range policies {
		spec, ok := policy.YAMLContent["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		ingressValue, hasIngress := spec["ingress"]
		if hasIngress && isOpenPolicyRuleSet(ingressValue) {
			description := fmt.Sprintf("NetworkPolicy '%s' allows all incoming traffic (open ingress).", policy.Name)
			recommendation := "Restrict ingress rules by specifying allowed sources and ports."
			createNetworkPolicyFinding(orgID, namespace, "high", description, recommendation)
		}

		egressValue, hasEgress := spec["egress"]
		if hasEgress && isOpenPolicyRuleSet(egressValue) {
			description := fmt.Sprintf("NetworkPolicy '%s' allows all outgoing traffic (open egress).", policy.Name)
			recommendation := "Restrict egress rules by specifying allowed destinations and ports."
			createNetworkPolicyFinding(orgID, namespace, "high", description, recommendation)
		}
	}
}

func isOpenPolicyRuleSet(value interface{}) bool {
	rules, ok := value.([]interface{})
	if !ok {
		return false
	}

	if len(rules) == 0 {
		return true
	}

	for _, ruleInterface := range rules {
		rule, ok := ruleInterface.(map[string]interface{})
		if ok && len(rule) == 0 {
			return true
		}
	}

	return false
}

func createNetworkPolicyFinding(orgID, namespace, severity, description, recommendation string) {
	findingID := generateFindingID()

	err := database.CreateValidationFinding(
		findingID,
		orgID,
		namespace,
		"NetworkPolicy",
		severity,
		description,
		recommendation,
	)

	if err != nil {
		fmt.Printf("❌ Failed to create network policy finding: %v\n", err)
		return
	}

	fmt.Printf("✅ Recorded network policy finding (severity: %s)\n", severity)
}

// checkRBACPermissions inspects Role and ClusterRole resources for overly permissive rules.
/*
runSecurityChecks
    ↓
checkRBACPermissions
    ↓
get Role + ClusterRole resources
    ↓
for each role:
    ↓
    extract rules[]
    ↓
    ├── resources contains "*" → CRITICAL
    ├── verbs contains "*" → CRITICAL
    ├── apiGroups "*" + resources "*" → CRITICAL (full cluster access)
    └── resources contains "secrets" with get/list/watch → HIGH
            ↓
    createRBACFinding
            ↓
    database.CreateValidationFinding
*/
func checkRBACPermissions(orgID, namespace string, kindMap KindMap) {
	roles := getKindResources(kindMap, "Role")
	clusterRoles := getKindResources(kindMap, "ClusterRole")

	var allRoles []Resource
	if len(roles) > 0 {
		allRoles = append(allRoles, roles...)
	}
	if len(clusterRoles) > 0 {
		allRoles = append(allRoles, clusterRoles...)
	}

	if len(allRoles) == 0 {
		return
	}

	for _, role := range allRoles {
		rulesInterface, hasRules := role.YAMLContent["rules"]
		if !hasRules || rulesInterface == nil {
			continue
		}

		rules, ok := rulesInterface.([]interface{})
		if !ok {
			continue
		}

		for _, ruleInterface := range rules {
			rule, ok := ruleInterface.(map[string]interface{})
			if !ok {
				continue
			}

			verbs := extractStringSlice(rule["verbs"])
			resources := extractStringSlice(rule["resources"])
			apiGroups := extractStringSlice(rule["apiGroups"])

			hasWildcardVerbs := containsString(verbs, "*")
			hasWildcardResources := containsString(resources, "*")
			hasWildcardAPIGroups := containsString(apiGroups, "*")

			// CHECK 1: Full cluster access — wildcard apiGroups + wildcard resources
			if hasWildcardAPIGroups && hasWildcardResources {
				description := fmt.Sprintf("%s '%s' grants access to all resources in all API groups (apiGroups: *, resources: *). This is equivalent to full cluster admin access.", role.Kind, role.Name)
				recommendation := fmt.Sprintf("Restrict %s '%s' to specific API groups and resource types following the principle of least privilege.", role.Kind, role.Name)
				createRBACFinding(orgID, namespace, role.Kind, "critical", description, recommendation)
				continue // Skip individual checks — this is the most severe
			}

			// CHECK 2: Wildcard resources
			if hasWildcardResources {
				description := fmt.Sprintf("%s '%s' grants access to all resources (resources: *). This is overly permissive.", role.Kind, role.Name)
				recommendation := fmt.Sprintf("Restrict %s '%s' to only the specific resource types needed (e.g., pods, deployments, services).", role.Kind, role.Name)
				createRBACFinding(orgID, namespace, role.Kind, "critical", description, recommendation)
			}

			// CHECK 3: Wildcard verbs
			if hasWildcardVerbs {
				resourceList := strings.Join(resources, ", ")
				if resourceList == "" {
					resourceList = "(unspecified)"
				}
				description := fmt.Sprintf("%s '%s' grants all verbs (verbs: *) on resources [%s]. This allows create, delete, and escalate operations.", role.Kind, role.Name, resourceList)
				recommendation := fmt.Sprintf("Restrict %s '%s' to only the specific verbs needed (e.g., get, list, watch) instead of using wildcard.", role.Kind, role.Name)
				createRBACFinding(orgID, namespace, role.Kind, "critical", description, recommendation)
			}

			// CHECK 4: Secrets access with read permissions
			if containsString(resources, "secrets") {
				hasReadAccess := containsString(verbs, "get") || containsString(verbs, "list") || containsString(verbs, "watch") || hasWildcardVerbs
				if hasReadAccess {
					description := fmt.Sprintf("%s '%s' grants read access to Secrets. This may expose sensitive credentials and tokens.", role.Kind, role.Name)
					recommendation := fmt.Sprintf("Review whether %s '%s' truly needs access to Secrets. Consider using more restrictive resource targeting.", role.Kind, role.Name)
					createRBACFinding(orgID, namespace, role.Kind, "high", description, recommendation)
				}
			}
		}
	}
}

func extractStringSlice(value interface{}) []string {
	if value == nil {
		return nil
	}

	slice, ok := value.([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(slice))
	for _, item := range slice {
		str, ok := item.(string)
		if ok {
			result = append(result, str)
		}
	}
	return result
}

func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

func createRBACFinding(orgID, namespace, kind, severity, description, recommendation string) {
	findingID := generateFindingID()

	err := database.CreateValidationFinding(
		findingID,
		orgID,
		namespace,
		kind,
		severity,
		description,
		recommendation,
	)

	if err != nil {
		fmt.Printf("❌ Failed to create RBAC finding: %v\n", err)
		return
	}

	fmt.Printf("✅ Recorded RBAC finding: %s '%s' (severity: %s)\n", kind, description[:min(len(description), 60)], severity)
}

// checkWorkloadSecrets checks plain text env vars in Deployment/DaemonSet containers.
func checkWorkloadSecrets(orgID, namespace string, resources []Resource) {
	for _, resource := range resources {
		spec, ok := resource.YAMLContent["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		template, ok := spec["template"].(map[string]interface{})
		if !ok {
			continue
		}

		templateSpec, ok := template["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		containersInterface, ok := templateSpec["containers"].([]interface{})
		if !ok {
			continue
		}

		for _, containerInterface := range containersInterface {
			container, ok := containerInterface.(map[string]interface{})
			if !ok {
				continue
			}

			containerName, _ := container["name"].(string)
			if containerName == "" {
				containerName = "unnamed"
			}

			envInterface, hasEnv := container["env"]
			if !hasEnv || envInterface == nil {
				continue
			}

			envVars, ok := envInterface.([]interface{})
			if !ok {
				continue
			}

			for _, envVarInterface := range envVars {
				envVar, ok := envVarInterface.(map[string]interface{})
				if !ok {
					continue
				}

				value, hasValue := envVar["value"]
				if !hasValue || value == nil {
					continue
				}

				hasSecretKeyRef := false
				valueFromInterface, hasValueFrom := envVar["valueFrom"]
				if hasValueFrom && valueFromInterface != nil {
					valueFromMap, ok := valueFromInterface.(map[string]interface{})
					if ok {
						secretKeyRef, exists := valueFromMap["secretKeyRef"]
						hasSecretKeyRef = exists && secretKeyRef != nil
					}
				}

				if hasSecretKeyRef {
					continue
				}

				description := fmt.Sprintf("Container '%s' in %s '%s' is using plain text environment variable instead of secretKeyRef.", containerName, resource.Kind, resource.Name)
				recommendation := "Use Kubernetes Secret with valueFrom.secretKeyRef instead of plain text environment variables."
				createSecretMisconfigurationFinding(orgID, namespace, resource.Kind, "high", description, recommendation)
			}
		}
	}
}

// checkConfigMapSecrets checks ConfigMaps for sensitive keys.
func checkConfigMapSecrets(orgID, namespace string, resources []Resource) {
	sensitiveKeywords := []string{"password", "token", "secret", "key", "apikey"}

	for _, resource := range resources {
		dataInterface, hasData := resource.YAMLContent["data"]
		if !hasData || dataInterface == nil {
			continue
		}

		dataMap, ok := dataInterface.(map[string]interface{})
		if !ok {
			continue
		}

		for key := range dataMap {
			lowerKey := strings.ToLower(key)
			for _, keyword := range sensitiveKeywords {
				if !strings.Contains(lowerKey, keyword) {
					continue
				}

				description := fmt.Sprintf("ConfigMap '%s' contains sensitive key '%s'. Sensitive data should not be stored in ConfigMaps.", resource.Name, key)
				recommendation := "Use Kubernetes Secrets instead of ConfigMaps for storing sensitive information."
				createSecretMisconfigurationFinding(orgID, namespace, resource.Kind, "high", description, recommendation)
				break
			}
		}
	}
}

// checkSecretEncoding checks Secret.data values for valid base64 encoding.
func checkSecretEncoding(orgID, namespace string, resources []Resource) {
	for _, resource := range resources {
		dataInterface, hasData := resource.YAMLContent["data"]
		if !hasData || dataInterface == nil {
			continue
		}

		dataMap, ok := dataInterface.(map[string]interface{})
		if !ok {
			continue
		}

		for key, valueInterface := range dataMap {
			value, ok := valueInterface.(string)
			if !ok {
				description := fmt.Sprintf("Secret '%s' contains non-base64 encoded value for key '%s'.", resource.Name, key)
				recommendation := "Encode secret values using base64 before storing them in Kubernetes Secret."
				createSecretMisconfigurationFinding(orgID, namespace, resource.Kind, "low", description, recommendation)
				continue
			}

			if _, err := base64.StdEncoding.DecodeString(value); err != nil {
				description := fmt.Sprintf("Secret '%s' contains non-base64 encoded value for key '%s'.", resource.Name, key)
				recommendation := "Encode secret values using base64 before storing them in Kubernetes Secret."
				createSecretMisconfigurationFinding(orgID, namespace, resource.Kind, "low", description, recommendation)
			}
		}
	}
}

func createSecretMisconfigurationFinding(orgID, namespace, kind, severity, description, recommendation string) {
	findingID := generateFindingID()

	err := database.CreateValidationFinding(
		findingID,
		orgID,
		namespace,
		kind,
		severity,
		description,
		recommendation,
	)

	if err != nil {
		fmt.Printf("❌ Failed to create secret misconfiguration finding for kind %s: %v\n", kind, err)
		return
	}

	fmt.Printf("✅ Recorded secret misconfiguration finding: %s (severity: %s)\n", kind, severity)
}

// checkDeploymentContainers checks all containers in a deployment for resource configuration issues
func checkDeploymentContainers(orgID, namespace string, deployment Resource) {
	// STEP 5: Extract containers - Navigate safely: spec → template → spec → containers
	spec, ok := deployment.YAMLContent["spec"].(map[string]interface{})
	if !ok {
		return
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return
	}

	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return
	}

	containersInterface, ok := templateSpec["containers"]
	if !ok {
		return
	}

	containers, ok := containersInterface.([]interface{})
	if !ok {
		return
	}

	// STEP 6: Loop through each container
	for _, containerInterface := range containers {
		container, ok := containerInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract container name
		containerName, _ := container["name"].(string)
		if containerName == "" {
			containerName = "unnamed"
		}

		// Extract resources
		resourcesInterface, hasResources := container["resources"]
		if !hasResources {
			resourcesInterface = nil
		}

		// STEP 7: Apply 4 conditions
		checkContainerResourceConfiguration(orgID, namespace, deployment.Name, containerName, resourcesInterface)
	}
}

// checkContainerResourceConfiguration applies all 4 resource configuration checks
/*
ValidateNamespaces
    ↓
validateResources
    ↓
runSecurityChecks
    ↓
checkResourceMisconfiguration
    ↓
checkDeploymentContainers
    ↓
checkContainerResourceConfiguration
    ↓
createResourceFinding
    ↓
database.CreateValidationFinding
*/

func checkContainerResourceConfiguration(orgID, namespace, deploymentName, containerName string, resourcesInterface interface{}) {
	// CONDITION 1: resources missing -
	if resourcesInterface == nil {
		createResourceFinding(
			orgID,
			namespace,
			deploymentName,
			containerName,
			"high",
			fmt.Sprintf("Container '%s' in Deployment '%s' has no resource configuration.", containerName, deploymentName),
			fmt.Sprintf("Add resource requests and limits to container '%s' in Deployment '%s'. Example:\nresources:\n  requests:\n    cpu: \"100m\"\n    memory: \"128Mi\"\n  limits:\n    cpu: \"500m\"\n    memory: \"512Mi\"", containerName, deploymentName),
		)
		return
	}
	/* CONDITION 2: EMPTY RESOURCES

	resources, ok := resourcesInterface.(map[string]interface{})
	if !ok || resources == nil || len(resources) == 0
	Meaning:
	resources: {}
	ACTION:
	createResourceFinding(...)

	*/
	resources, ok := resourcesInterface.(map[string]interface{})
	if !ok || resources == nil || len(resources) == 0 {
		createResourceFinding(
			orgID,
			namespace,
			deploymentName,
			containerName,
			"high",
			fmt.Sprintf("Container '%s' in Deployment '%s' has empty resource configuration.", containerName, deploymentName),
			fmt.Sprintf("Add resource requests and limits to container '%s' in Deployment '%s'.", containerName, deploymentName),
		)
		return
	}

	// Extract requests and limits
	/* 🚨 CONDITION 3: MISSING REQUESTS / LIMITS */
	requestsInterface, hasRequests := resources["requests"]
	limitsInterface, hasLimits := resources["limits"]

	//CASE 1: Missing requests
	if !hasRequests || requestsInterface == nil {
		createResourceFinding(
			orgID,
			namespace,
			deploymentName,
			containerName,
			"high",
			fmt.Sprintf("Container '%s' in Deployment '%s' is missing resource requests.", containerName, deploymentName),
			fmt.Sprintf("Add resource requests to container '%s' in Deployment '%s'. Example:\nrequests:\n  cpu: \"100m\"\n  memory: \"128Mi\"", containerName, deploymentName),
		)
	}

	//CASE 2: Missing limits
	if !hasLimits || limitsInterface == nil {
		createResourceFinding(
			orgID,
			namespace,
			deploymentName,
			containerName,
			"high",
			fmt.Sprintf("Container '%s' in Deployment '%s' is missing resource limits.", containerName, deploymentName),
			fmt.Sprintf("Add resource limits to container '%s' in Deployment '%s'. Example:\nlimits:\n  cpu: \"500m\"\n  memory: \"512Mi\"", containerName, deploymentName),
		)
	}

	// If either is missing, skip further checks
	if !hasRequests || !hasLimits || requestsInterface == nil || limitsInterface == nil {
		return
	}

	requests, ok1 := requestsInterface.(map[string]interface{})
	limits, ok2 := limitsInterface.(map[string]interface{})

	if !ok1 || !ok2 {
		return
	}

	// 🚨 CONDITION 4: CPU / MEMORY MISSING
	requestsCPU, hasRequestsCPU := requests["cpu"]
	requestsMemory, hasRequestsMemory := requests["memory"]
	limitsCPU, hasLimitsCPU := limits["cpu"]
	limitsMemory, hasLimitsMemory := limits["memory"]

	if !hasRequestsCPU || requestsCPU == nil {
		createResourceFinding(
			orgID,
			namespace,
			deploymentName,
			containerName,
			"high",
			fmt.Sprintf("Container '%s' in Deployment '%s' is missing requests.cpu.", containerName, deploymentName),
			fmt.Sprintf("Add CPU request to container '%s'. Example: requests.cpu: \"100m\"", containerName),
		)
	}

	if !hasRequestsMemory || requestsMemory == nil {
		createResourceFinding(
			orgID,
			namespace,
			deploymentName,
			containerName,
			"high",
			fmt.Sprintf("Container '%s' in Deployment '%s' is missing requests.memory.", containerName, deploymentName),
			fmt.Sprintf("Add memory request to container '%s'. Example: requests.memory: \"128Mi\"", containerName),
		)
	}

	if !hasLimitsCPU || limitsCPU == nil {
		createResourceFinding(
			orgID,
			namespace,
			deploymentName,
			containerName,
			"high",
			fmt.Sprintf("Container '%s' in Deployment '%s' is missing limits.cpu.", containerName, deploymentName),
			fmt.Sprintf("Add CPU limit to container '%s'. Example: limits.cpu: \"500m\"", containerName),
		)
	}

	if !hasLimitsMemory || limitsMemory == nil {
		createResourceFinding(
			orgID,
			namespace,
			deploymentName,
			containerName,
			"high",
			fmt.Sprintf("Container '%s' in Deployment '%s' is missing limits.memory.", containerName, deploymentName),
			fmt.Sprintf("Add memory limit to container '%s'. Example: limits.memory: \"512Mi\"", containerName),
		)
	}

	// CONDITION 5: REQUEST > LIMIT (CRITICAL)
	if hasRequestsCPU && hasLimitsCPU && requestsCPU != nil && limitsCPU != nil {
		requestsCPUStr, ok1 := requestsCPU.(string)
		limitsCPUStr, ok2 := limitsCPU.(string)

		if ok1 && ok2 {
			//. PARSING FUNCTIONS
			requestsCPUValue := parseCPU(requestsCPUStr)
			limitsCPUValue := parseCPU(limitsCPUStr)

			if requestsCPUValue == 0 || limitsCPUValue == 0 {
				// Skip comparison when either value is empty or unparsable.
			} else if requestsCPUValue > limitsCPUValue {
				createResourceFinding(
					orgID,
					namespace,
					deploymentName,
					containerName,
					"critical",
					fmt.Sprintf("Container '%s' in Deployment '%s' has requests.cpu (%s) greater than limits.cpu (%s).", containerName, deploymentName, requestsCPUStr, limitsCPUStr),
					fmt.Sprintf("Ensure requests.cpu is less than or equal to limits.cpu for container '%s'. Current: requests=%s, limits=%s", containerName, requestsCPUStr, limitsCPUStr),
				)
			}
		}
	}

	if hasRequestsMemory && hasLimitsMemory && requestsMemory != nil && limitsMemory != nil {
		requestsMemoryStr, ok1 := requestsMemory.(string)
		limitsMemoryStr, ok2 := limitsMemory.(string)

		if ok1 && ok2 {
			/*
				MEMORY Parsing:
				parseMemory("128Mi") → bytes
				parseMemory("1Gi") → bytes
			*/
			requestsMemoryValue := parseMemory(requestsMemoryStr)
			limitsMemoryValue := parseMemory(limitsMemoryStr)

			if requestsMemoryValue == 0 || limitsMemoryValue == 0 {
				// Skip comparison when either value is empty or unparsable.
			} else if requestsMemoryValue > limitsMemoryValue {
				createResourceFinding(
					orgID,
					namespace,
					deploymentName,
					containerName,
					"critical",
					fmt.Sprintf("Container '%s' in Deployment '%s' has requests.memory (%s) greater than limits.memory (%s).", containerName, deploymentName, requestsMemoryStr, limitsMemoryStr),
					fmt.Sprintf("Ensure requests.memory is less than or equal to limits.memory for container '%s'. Current: requests=%s, limits=%s", containerName, requestsMemoryStr, limitsMemoryStr),
				)
			}
		}
	}
}

// createResourceFinding creates a finding for resource misconfiguration
func createResourceFinding(orgID, namespace, deploymentName, containerName, severity, description, recommendations string) {
	findingID := generateFindingID()

	err := database.CreateValidationFinding(
		findingID,
		orgID,
		namespace,
		"Deployment",
		severity,
		description,
		recommendations,
	)

	if err != nil {
		fmt.Printf("❌ Failed to create resource finding for %s/%s: %v\n", deploymentName, containerName, err)
		return
	}

	fmt.Printf("✅ Recorded resource finding: %s/%s (severity: %s)\n", deploymentName, containerName, severity)
}

// parseCPU parses Kubernetes CPU format to float64
// Supports: "500m" = 0.5, "1" = 1.0, "2000m" = 2.0
func parseCPU(value string) float64 {
	if value == "" {
		return 0
	}

	// Handle millicores (e.g., "500m")
	if strings.HasSuffix(value, "m") {
		milliStr := strings.TrimSuffix(value, "m")
		milli, err := strconv.ParseFloat(milliStr, 64)
		if err != nil {
			return 0
		}
		return milli / 1000.0
	}

	// Handle cores (e.g., "1", "2.5")
	cores, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return cores
}

// parseMemory parses Kubernetes memory format to bytes
// Supports: "128Mi", "1Gi", "512M", "1G"
func parseMemory(value string) float64 {
	if value == "" {
		return 0
	}

	value = strings.TrimSpace(value)

	// Binary units (Ki, Mi, Gi, Ti)
	if strings.HasSuffix(value, "Ki") {
		numStr := strings.TrimSuffix(value, "Ki")
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0
		}
		return num * 1024
	}

	if strings.HasSuffix(value, "Mi") {
		numStr := strings.TrimSuffix(value, "Mi")
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0
		}
		return num * 1024 * 1024
	}

	if strings.HasSuffix(value, "Gi") {
		numStr := strings.TrimSuffix(value, "Gi")
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0
		}
		return num * 1024 * 1024 * 1024
	}

	if strings.HasSuffix(value, "Ti") {
		numStr := strings.TrimSuffix(value, "Ti")
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0
		}
		return num * 1024 * 1024 * 1024 * 1024
	}

	// Decimal units (K, M, G, T)
	if strings.HasSuffix(value, "K") {
		numStr := strings.TrimSuffix(value, "K")
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0
		}
		return num * 1000
	}

	if strings.HasSuffix(value, "M") {
		numStr := strings.TrimSuffix(value, "M")
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0
		}
		return num * 1000 * 1000
	}

	if strings.HasSuffix(value, "G") {
		numStr := strings.TrimSuffix(value, "G")
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0
		}
		return num * 1000 * 1000 * 1000
	}

	if strings.HasSuffix(value, "T") {
		numStr := strings.TrimSuffix(value, "T")
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0
		}
		return num * 1000 * 1000 * 1000 * 1000
	}

	// Plain bytes
	bytes, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return bytes
}
