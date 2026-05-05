package fetchingservice

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type ValidationRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
	ReleaseName  string `json:"release_name"`
}

var executedHelmCommands []string

func Render(c *gin.Context) {
	var req ValidationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "session_token is required", err)
		return
	}

	githubUsername, err := database.ValidateSession(req.SessionToken)
	if err != nil {
		response.Unauthorized(c, "invalid or expired session", err)
		return
	}

	orgID, err := database.GetUserOrgID(githubUsername)
	if err != nil {
		response.InternalError(c, "failed to get organization", err)
		return
	}

	if req.ReleaseName == "" {
		req.ReleaseName = "my-release"
	}

	fmt.Printf("🎯 Starting Helm rendering and K8s ingestion for orgID=%s, release=%s\n", orgID, req.ReleaseName)

	resourceCount, err := RenderHelmAndStoreResources(orgID, req.ReleaseName)
	if err != nil {
		response.InternalError(c, "failed to render helm and store resources", err)
		return
	}

	fmt.Printf("✅ Stored %d Kubernetes resources\n", resourceCount)

	// Get executed commands for response
	executedCommands := getExecutedCommands(orgID)

	response.Success(c, http.StatusOK, "rendering completed", gin.H{
		"status":            "success",
		"resources":         resourceCount,
		"release_name":      req.ReleaseName,
		"org_id":            orgID,
		"message":           fmt.Sprintf("Successfully rendered and stored %d Kubernetes resources", resourceCount),
		"executed_commands": executedCommands,
	})
}

func RenderHelmAndStoreResources(orgID string, release string) (int, error) {
	fmt.Printf("🔍 Starting Helm rendering - fetching files from GitHub\n")

	// Get GitHub credentials
	var githubUsername, encryptedPAT string
	err := database.DB.QueryRow(`
		SELECT gc.github_username, gc.encrypted_pat
		FROM github_credentials gc
		WHERE gc.org_id = $1
		LIMIT 1
	`, orgID).Scan(&githubUsername, &encryptedPAT)

	if err != nil {
		return 0, fmt.Errorf("failed to get GitHub credentials: %v", err)
	}

	// Decrypt PAT
	patBytes, _ := hex.DecodeString(encryptedPAT)
	pat := string(patBytes)

	// Get repository info from github_repository table
	var repoName, repoURL string
	err = database.DB.QueryRow(`
		SELECT repo_name, repo_url
		FROM github_repository
		WHERE org_id = $1
		LIMIT 1
	`, orgID).Scan(&repoName, &repoURL)

	if err != nil {
		return 0, fmt.Errorf("failed to get repository: %v", err)
	}

	// Extract owner from repo URL (e.g., https://github.com/owner/repo -> owner/repo)
	// Or construct from username if URL parsing fails
	var fullRepoName string
	if strings.Contains(repoURL, "github.com/") {
		parts := strings.Split(repoURL, "github.com/")
		if len(parts) > 1 {
			fullRepoName = strings.TrimSuffix(parts[1], ".git")
		}
	}

	if fullRepoName == "" {
		// Fallback: use username/reponame
		fullRepoName = fmt.Sprintf("%s/%s", githubUsername, repoName)
	}

	fmt.Printf("📦 Repository: %s\n", fullRepoName)

	tempDir, err := os.MkdirTemp("", "helm-render-*")
	if err != nil {
		return 0, fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("📁 Temp directory: %s\n", tempDir)

	// Fetch Helm files directly from GitHub
	helmFiles, err := fetchHelmFilesFromGitHub(fullRepoName, pat)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch Helm files from GitHub: %v", err)
	}

	fmt.Printf("✅ Fetched %d Helm files from GitHub\n", len(helmFiles))

	// Write files to temp directory
	for path, content := range helmFiles {
		fullPath := filepath.Join(tempDir, path)

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			fmt.Printf("⚠️ Failed to create dir for %s: %v\n", path, err)
			continue
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			fmt.Printf("⚠️ Failed to write %s: %v\n", path, err)
			continue
		}
	}

	chartsDir := filepath.Join(tempDir, "Helm", "charts")
	chartEntries, err := os.ReadDir(chartsDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read charts directory: %v", err)
	}

	var chartFolders []string
	for _, entry := range chartEntries {
		if entry.IsDir() {
			chartFolders = append(chartFolders, filepath.Join(chartsDir, entry.Name()))
		}
	}

	if len(chartFolders) == 0 {
		return 0, fmt.Errorf("no chart folders found in Helm/charts/")
	}

	valuesDir := filepath.Join(tempDir, "Helm", "environments", "dummy-tenant")
	valuesEntries, err := os.ReadDir(valuesDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read values directory: %v", err)
	}

	var valuesFiles []string
	for _, entry := range valuesEntries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") && strings.HasPrefix(entry.Name(), "values-") {
			valuesFiles = append(valuesFiles, filepath.Join(valuesDir, entry.Name()))
		}
	}

	if len(valuesFiles) == 0 {
		return 0, fmt.Errorf("no values files found in Helm/environments/dummy-tenant/")
	}

	fmt.Printf("📁 Found %d chart(s)\n", len(chartFolders))
	fmt.Printf("📄 Found %d values file(s)\n", len(valuesFiles))

	totalResources := 0

	// Build a set of chart folder names for quick lookup
	chartNameSet := make(map[string]string) // chartName -> chartFolder path
	for _, chartFolder := range chartFolders {
		chartNameSet[filepath.Base(chartFolder)] = chartFolder
	}

	// Smart mapping: pair each values file with its intended chart.
	// If the service name (from values-<service>.yaml) matches a chart folder name, use that chart.
	// Otherwise, fall back to the first chart that is NOT an exact match for any values file
	// (typically "backend-common").
	var fallbackChart string
	for _, chartFolder := range chartFolders {
		chartName := filepath.Base(chartFolder)
		isExactMatch := false
		for _, valuesFile := range valuesFiles {
			vName := strings.TrimPrefix(filepath.Base(valuesFile), "values-")
			vName = strings.TrimSuffix(vName, ".yaml")
			if strings.EqualFold(vName, chartName) {
				isExactMatch = true
				break
			}
		}
		if !isExactMatch {
			fallbackChart = chartFolder
			break
		}
	}
	// If every chart matched a values file, just use the first chart as fallback
	if fallbackChart == "" && len(chartFolders) > 0 {
		fallbackChart = chartFolders[0]
	}

	for _, valuesFile := range valuesFiles {
		valuesName := filepath.Base(valuesFile)

		// Extract service name from values file (e.g., values-ecommerce.yaml -> ecommerce)
		serviceName := strings.TrimPrefix(valuesName, "values-")
		serviceName = strings.TrimSuffix(serviceName, ".yaml")

		// Determine which chart to use for this values file
		var chartFolder string
		if matchedChart, ok := chartNameSet[serviceName]; ok {
			// Exact match: values-frontend.yaml -> frontend chart
			chartFolder = matchedChart
		} else {
			// No matching chart name -> use fallback (e.g., backend-common)
			chartFolder = fallbackChart
		}

		chartName := filepath.Base(chartFolder)

		// Create dynamic release name: dummy-<service>
		dynamicRelease := fmt.Sprintf("dummy-%s", serviceName)

		fmt.Printf("🔄 Rendering: release=%s, chart=%s, values=%s\n", dynamicRelease, chartName, valuesName)

		renderedYAML, err := ExecuteHelmRender(dynamicRelease, chartFolder, valuesFile, valuesDir)
		if err != nil {
			fmt.Printf("❌ Helm render failed: %v\n", err)
			continue
		}

		if strings.TrimSpace(renderedYAML) == "" {
			fmt.Printf("⚠️ Rendered YAML is empty\n")
			continue
		}

		fmt.Printf("📝 Rendered YAML length: %d bytes\n", len(renderedYAML))

		// Log first 500 chars of rendered YAML for debugging
		if len(renderedYAML) > 500 {
			fmt.Printf("📄 Preview:\n%s\n...\n", renderedYAML[:500])
		} else {
			fmt.Printf("📄 Full output:\n%s\n", renderedYAML)
		}

		resources, err := ParseRenderedYAML(renderedYAML, chartName)
		if err != nil {
			fmt.Printf("❌ YAML parse failed: %v\n", err)
			continue
		}

		fmt.Printf("✅ Parsed %d resources from %s + %s\n", len(resources), chartName, valuesName)

		if len(resources) == 0 {
			continue
		}

		inserted, err := InsertK8sResources(orgID, resources)
		if err != nil {
			fmt.Printf("❌ Insert failed: %v\n", err)
			continue
		}

		totalResources += inserted
	}

	if totalResources == 0 {
		return 0, fmt.Errorf("no resources were rendered and stored")
	}

	return totalResources, nil
}

func ExecuteHelmRender(release, chartFolder, valuesFile, valuesDir string) (string, error) {
	helmPath := "helm"

	possiblePaths := []string{
		"helm",
		"C:\\Users\\alive\\bin\\helm.exe",
		"C:\\Program Files\\Helm\\helm.exe",
		"C:\\ProgramData\\chocolatey\\bin\\helm.exe",
	}

	for _, path := range possiblePaths {
		if _, err := exec.LookPath(path); err == nil {
			helmPath = path
			break
		}
		if _, err := os.Stat(path); err == nil {
			helmPath = path
			break
		}
	}

	// Calculate relative path from valuesDir to chartFolder
	// User wants: helm template dummy-ecommerce ../../charts/backend-common -f values-ecommerce.yaml
	// This means: from Helm/environments/dummy-tenant/ -> ../../charts/backend-common

	relChartPath, err := filepath.Rel(valuesDir, chartFolder)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path: %v", err)
	}

	// Convert Windows backslashes to forward slashes for consistency
	relChartPath = filepath.ToSlash(relChartPath)

	// Get just the values filename (not full path)
	valuesFileName := filepath.Base(valuesFile)

	helmCommand := fmt.Sprintf("helm template %s %s -f %s", release, relChartPath, valuesFileName)

	fmt.Printf("   📍 Working dir: %s\n", valuesDir)
	fmt.Printf("   📍 Chart path: %s\n", relChartPath)
	fmt.Printf("   📍 Values file: %s\n", valuesFileName)
	fmt.Printf("   📍 Command: %s\n", helmCommand)

	// Store command for response
	executedHelmCommands = append(executedHelmCommands, helmCommand)

	cmd := exec.Command(helmPath, "template", release, relChartPath, "-f", valuesFileName)
	cmd.Dir = valuesDir // Set working directory to values directory

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("helm command failed: %v, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func ParseRenderedYAML(output string, chartFolderName string) ([]K8sResourceItem, error) {
	// Split by YAML document separator
	documents := strings.Split(output, "\n---")
	var resources []K8sResourceItem

	for i, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Remove comment lines but keep "# Source:" for debugging
		lines := strings.Split(doc, "\n")
		var cleanedLines []string
		var sourceFile string

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Extract source file for debugging
			if strings.HasPrefix(trimmed, "# Source:") {
				sourceFile = strings.TrimPrefix(trimmed, "# Source:")
				sourceFile = strings.TrimSpace(sourceFile)
				continue
			}

			// Skip other comment lines
			if strings.HasPrefix(trimmed, "#") {
				continue
			}

			cleanedLines = append(cleanedLines, line)
		}

		doc = strings.Join(cleanedLines, "\n")
		doc = strings.TrimSpace(doc)

		if doc == "" {
			continue
		}

		// Parse YAML to map - this preserves ALL Helm-rendered values
		var parsed map[string]any
		if err := yaml.Unmarshal([]byte(doc), &parsed); err != nil {
			fmt.Printf("   ⚠️ Doc %d: YAML unmarshal failed: %v\n", i, err)
			continue
		}

		// Extract kind
		kind, _ := parsed["kind"].(string)
		if kind == "" {
			fmt.Printf("   ⚠️ Doc %d: No kind found, skipping\n", i)
			continue
		}

		// Extract metadata
		metadata, ok := parsed["metadata"].(map[string]any)
		if !ok {
			fmt.Printf("   ⚠️ Doc %d (%s): No metadata found, skipping\n", i, kind)
			continue
		}

		// Extract name and namespace
		name, _ := metadata["name"].(string)
		namespace, _ := metadata["namespace"].(string)

		if name == "" {
			name = "unnamed"
		}
		if namespace == "" {
			namespace = "default"
		}

		// Convert the ENTIRE parsed map to JSON
		// This preserves ALL Helm-rendered values including:
		// - labels, annotations
		// - selector.matchLabels
		// - template.metadata.labels
		// - image, tag, pullPolicy
		// - resources, probes
		// - env, affinity, tolerations
		// - Everything from .Values.* and .Chart.*
		jsonBytes, err := json.Marshal(parsed)
		if err != nil {
			fmt.Printf("   ⚠️ Doc %d (%s/%s): JSON marshal failed: %v\n", i, kind, name, err)
			continue
		}

		if sourceFile != "" {
			fmt.Printf("   ✅ Parsed %s/%s from %s\n", kind, name, sourceFile)
		} else {
			fmt.Printf("   ✅ Parsed %s/%s\n", kind, name)
		}

		resources = append(resources, K8sResourceItem{
			Kind:        kind,
			Name:        name,
			Namespace:   namespace,
			SourceFile:  normalizeSourcePathForRepo(sourceFile, chartFolderName),
			YAMLContent: string(jsonBytes),
		})
	}

	return resources, nil
}

type K8sResourceItem struct {
	Kind        string
	Name        string
	Namespace   string
	SourceFile  string
	YAMLContent string
}

func InsertK8sResources(orgID string, resources []K8sResourceItem) (int, error) {
	inserted := 0
	clusterID := resolveClusterID(orgID)
	repoFilePathMap, err := loadRepoFilePathMap(orgID)
	if err != nil {
		fmt.Printf("⚠️ Failed to load github file path map: %v\n", err)
	}

	for _, res := range resources {
		resourceID := generateK8sResourceID(orgID, res.Kind, res.Name, res.Namespace)
		repoFileID := resolveRepoFileID(res.SourceFile, repoFilePathMap)

		var repoFileIDArg interface{}
		if repoFileID > 0 {
			repoFileIDArg = repoFileID
		}

		_, _ = database.DB.Exec(`
			UPDATE kubernetes_resource
			SET
				repo_file_id = COALESCE(repo_file_id, $1),
				cluster_id = CASE
					WHEN cluster_id IS NULL OR cluster_id = '' OR cluster_id = '1' THEN $2
					ELSE cluster_id
				END
			WHERE org_id = $3 AND kind = $4 AND name = $5 AND namespace = $6
		`, repoFileIDArg, clusterID, orgID, res.Kind, res.Name, res.Namespace)

		_, err = database.DB.Exec(`
			INSERT INTO kubernetes_resource (resource_id, org_id, repo_file_id, cluster_id, kind, name, namespace, yaml_content, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (resource_id) DO UPDATE SET
				repo_file_id = COALESCE(EXCLUDED.repo_file_id, kubernetes_resource.repo_file_id),
				cluster_id = COALESCE(EXCLUDED.cluster_id, kubernetes_resource.cluster_id),
				yaml_content = EXCLUDED.yaml_content,
				created_at = EXCLUDED.created_at
		`, resourceID, orgID, repoFileIDArg, clusterID, res.Kind, res.Name, res.Namespace, res.YAMLContent, time.Now())

		if err != nil {
			fmt.Printf("❌ Failed to insert resource %s/%s: %v\n", res.Kind, res.Name, err)
			continue
		}

		inserted++
	}

	return inserted, nil
}

func generateK8sResourceID(orgID, kind, name, namespace string) string {
	input := fmt.Sprintf("%s|%s|%s|%s", orgID, kind, name, namespace)
	hash := uint32(0)
	for _, char := range input {
		hash = hash*31 + uint32(char)
	}
	return fmt.Sprintf("%06d", hash%1000000)
}

func resolveClusterID(orgID string) string {
	var repoID string
	err := database.DB.QueryRow(`
		SELECT repo_id
		FROM github_repository
		WHERE org_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, orgID).Scan(&repoID)
	if err != nil || strings.TrimSpace(repoID) == "" {
		return "1"
	}

	return repoID
}

func loadRepoFilePathMap(orgID string) (map[string]int, error) {
	rows, err := database.DB.Query(`
		SELECT id, file_path
		FROM github_files
		WHERE org_id = $1
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pathMap := make(map[string]int)
	for rows.Next() {
		var id int
		var filePath string
		if err := rows.Scan(&id, &filePath); err != nil {
			continue
		}
		pathMap[normalizeRepoPath(filePath)] = id
	}

	return pathMap, nil
}

func resolveRepoFileID(sourceFile string, repoFilePathMap map[string]int) int {
	source := normalizeRepoPath(sourceFile)
	if source == "" || len(repoFilePathMap) == 0 {
		return 0
	}

	candidates := []string{source}
	trimmedSource := strings.TrimPrefix(source, "./")
	if trimmedSource != source {
		candidates = append(candidates, trimmedSource)
	}
	if !strings.HasPrefix(trimmedSource, "helm/charts/") {
		candidates = append(candidates, "helm/charts/"+trimmedSource)
	}

	for _, candidate := range candidates {
		if id, ok := repoFilePathMap[candidate]; ok {
			return id
		}
	}

	bestMatchID := 0
	bestMatchPathLen := 0
	for path, id := range repoFilePathMap {
		if path == source || strings.HasSuffix(path, "/"+source) {
			if len(path) > bestMatchPathLen {
				bestMatchPathLen = len(path)
				bestMatchID = id
			}
		}
	}

	return bestMatchID
}

func normalizeRepoPath(path string) string {
	normalized := strings.TrimSpace(path)
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	return strings.ToLower(normalized)
}

func normalizeSourcePathForRepo(sourceFile, chartFolderName string) string {
	normalizedSource := strings.TrimSpace(strings.ReplaceAll(sourceFile, "\\", "/"))
	if normalizedSource == "" {
		return ""
	}

	parts := strings.SplitN(normalizedSource, "/", 2)
	if len(parts) == 2 {
		normalizedSource = filepath.ToSlash(filepath.Join("Helm", "charts", chartFolderName, parts[1]))
	} else {
		normalizedSource = filepath.ToSlash(filepath.Join("Helm", "charts", chartFolderName, normalizedSource))
	}

	return normalizedSource
}

func getExecutedCommands(orgID string) []string {
	return executedHelmCommands
}

func fetchHelmFilesFromGitHub(fullRepoName, pat string) (map[string]string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	fmt.Printf("🔍 Fetching from repository: %s\n", fullRepoName)

	// Get default branch
	branchReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s", fullRepoName), nil)
	branchReq.Header.Set("Authorization", "token "+pat)
	branchReq.Header.Set("Accept", "application/vnd.github.v3+json")

	branchResp, err := client.Do(branchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %v", err)
	}
	defer branchResp.Body.Close()

	if branchResp.StatusCode != 200 {
		body, _ := io.ReadAll(branchResp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", branchResp.StatusCode, string(body))
	}

	var repoInfo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(branchResp.Body).Decode(&repoInfo); err != nil {
		return nil, fmt.Errorf("failed to decode repository info: %v", err)
	}

	branch := repoInfo.DefaultBranch
	if branch == "" {
		branch = "main"
	}

	fmt.Printf("📍 Using branch: %s\n", branch)

	// Get file tree
	treeReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/git/trees/%s?recursive=1", fullRepoName, branch), nil)
	treeReq.Header.Set("Authorization", "token "+pat)
	treeReq.Header.Set("Accept", "application/vnd.github.v3+json")

	treeResp, err := client.Do(treeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get file tree: %v", err)
	}
	defer treeResp.Body.Close()

	if treeResp.StatusCode != 200 {
		body, _ := io.ReadAll(treeResp.Body)
		return nil, fmt.Errorf("GitHub tree API returned %d: %s", treeResp.StatusCode, string(body))
	}

	var treeData struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.NewDecoder(treeResp.Body).Decode(&treeData); err != nil {
		return nil, fmt.Errorf("failed to decode tree data: %v", err)
	}

	helmFiles := make(map[string]string)

	// Fetch only Helm files
	for _, item := range treeData.Tree {
		if item.Type == "blob" && strings.HasPrefix(item.Path, "Helm/") {
			// Fetch file content
			contentReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", fullRepoName, item.Path), nil)
			contentReq.Header.Set("Authorization", "token "+pat)
			contentReq.Header.Set("Accept", "application/vnd.github.v3.raw")

			contentResp, err := client.Do(contentReq)
			if err != nil {
				fmt.Printf("⚠️ Failed to fetch %s: %v\n", item.Path, err)
				continue
			}

			if contentResp.StatusCode == 200 {
				content, _ := io.ReadAll(contentResp.Body)
				helmFiles[item.Path] = string(content)
			}
			contentResp.Body.Close()
		}
	}

	return helmFiles, nil
}
