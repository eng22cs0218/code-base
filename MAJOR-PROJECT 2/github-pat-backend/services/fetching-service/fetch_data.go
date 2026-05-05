package fetchingservice

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/parser"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type FetchDataRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
}

type FetchDataResponse struct {
	Status       string `json:"status"`
	Message      string `json:"message"`
	TotalRepos   int    `json:"total_repos"`
	ScannedFiles int    `json:"scanned_files"`
	K8sResources int    `json:"k8s_resources"`
}

// FetchData triggers GitHub repository scanning and K8s resource extraction
func FetchData(c *gin.Context) {
	var req FetchDataRequest
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

	// Get user's cred_id and org_id
	credID, err := database.GetUserCredID(githubUsername)
	if err != nil {
		response.InternalError(c, "failed to get user credentials", err)
		return
	}

	orgID, err := database.GetUserOrgID(githubUsername)
	if err != nil {
		response.InternalError(c, "failed to get organization", err)
		return
	}

	fmt.Printf("🔍 FetchData: githubUsername=%s, credID=%s, orgID=%s\n", githubUsername, credID, orgID)

	// Get user's GitHub PAT
	var encryptedPAT string
	err = database.DB.QueryRow(`
		SELECT encrypted_pat
		FROM github_credentials
		WHERE cred_id = $1
	`, credID).Scan(&encryptedPAT)

	if err != nil {
		response.InternalError(c, "failed to get credentials", err)
		return
	}

	fmt.Printf("👤 GitHub Username: %s, CredID: %s, OrgID: %s\n", githubUsername, credID, orgID)

	// Keep orgID for repository (foreign key constraint)
	// Use credID for resource isolation

	// Decrypt PAT
	patBytes, _ := hex.DecodeString(encryptedPAT)
	pat := string(patBytes)

	// Fetch repositories from GitHub
	repos, err := fetchGitHubRepositories(githubUsername, pat)
	if err != nil {
		response.InternalError(c, "failed to fetch repositories from GitHub", err)
		return
	}

	totalRepos := len(repos)
	scannedFiles := 0
	k8sResources := 0

	fmt.Printf("📦 Found %d repositories\n", totalRepos)

	// Detect if this is FIRST-TIME or SECOND-TIME fetch
	var repoCount, fileCount int
	database.DB.QueryRow(`SELECT COUNT(*) FROM github_repository WHERE org_id = $1 AND cred_id = $2`, orgID, credID).Scan(&repoCount)
	database.DB.QueryRow(`SELECT COUNT(*) FROM github_files WHERE org_id = $1`, orgID).Scan(&fileCount)

	isFirstTimeFetch := (repoCount == 0 && fileCount == 0)

	if isFirstTimeFetch {
		fmt.Printf("🟢 FIRST-TIME FETCH: Performing full GitHub scan\n")
	} else {
		fmt.Printf("🟡 SECOND-TIME FETCH: Performing incremental sync (repos=%d, files=%d)\n", repoCount, fileCount)
	}

	// Process each repository
	for _, repo := range repos {
		fmt.Printf("🔄 Processing repo: %s\n", repo.FullName)

		// Store repository
		repoID := generateRepoID()

		// Check if repository already exists
		var existingRepoID string
		err = database.DB.QueryRow(`
			SELECT repo_id FROM github_repository 
			WHERE repo_name = $1 AND org_id = $2
		`, repo.Name, orgID).Scan(&existingRepoID)

		if err == nil {
			// Repository exists, use existing ID
			repoID = existingRepoID
			fmt.Printf("   ♻️  Using existing repo_id: %s\n", repoID)
		} else {
			// Insert new repository with cred_id
			_, err = database.DB.Exec(`
				INSERT INTO github_repository (repo_id, org_id, cred_id, repo_name, repo_url, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, repoID, orgID, credID, repo.Name, repo.URL, time.Now())

			if err != nil {
				fmt.Printf("   ❌ Failed to insert repo: %v\n", err)
				continue
			}
			fmt.Printf("   ✅ Created new repo_id: %s\n", repoID)
		}

		// Scan files from repository using full_name (owner/repo)
		files, err := scanRepositoryFiles(repo.FullName, pat, isFirstTimeFetch, repoID, orgID)
		if err != nil {
			fmt.Printf("   ❌ Failed to scan files: %v\n", err)
			continue
		}

		fmt.Printf("   📁 Found %d files\n", len(files))
		scannedFiles += len(files)

		// Store files in database and extract K8s resources
		k8sCount := storeFilesAndExtractK8s(files, repoID, orgID, repo.FullName)
		k8sResources += k8sCount
		fmt.Printf("   ☸️  Extracted %d K8s resources\n", k8sCount)
	}

	fmt.Printf("✅ Fetch completed: repos=%d, files=%d, k8s=%d\n", totalRepos, scannedFiles, k8sResources)

	fetchResp := FetchDataResponse{
		Status:       "completed",
		Message:      "Data fetching completed successfully",
		TotalRepos:   totalRepos,
		ScannedFiles: scannedFiles,
		K8sResources: k8sResources,
	}

	response.Success(c, http.StatusOK, "fetch data completed", fetchResp)
}

// fetchGitHubRepositories fetches user's repositories from GitHub
func fetchGitHubRepositories(username, pat string) ([]Repository, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Use /user/repos to get all repositories (public + private) for authenticated user
	req, _ := http.NewRequest("GET", "https://api.github.com/user/repos?per_page=100", nil)
	req.Header.Set("Authorization", "token "+pat)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var repos []Repository
	json.Unmarshal(body, &repos)

	return repos, nil
}

// scanRepositoryFiles scans files from a repository and fetches YAML content
func scanRepositoryFiles(fullRepoName, pat string, isFirstTimeFetch bool, repoID, orgID string) ([]File, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Get default branch
	branchReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s", fullRepoName), nil)
	branchReq.Header.Set("Authorization", "token "+pat)
	branchReq.Header.Set("Accept", "application/vnd.github.v3+json")

	branchResp, err := client.Do(branchReq)
	if err != nil {
		return nil, err
	}
	defer branchResp.Body.Close()

	var repoInfo struct {
		DefaultBranch string `json:"default_branch"`
	}
	json.NewDecoder(branchResp.Body).Decode(&repoInfo)
	branch := repoInfo.DefaultBranch
	if branch == "" {
		branch = "main"
	}

	// Get file tree
	treeReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/git/trees/%s?recursive=1", fullRepoName, branch), nil)
	treeReq.Header.Set("Authorization", "token "+pat)
	treeReq.Header.Set("Accept", "application/vnd.github.v3+json")

	treeResp, err := client.Do(treeReq)
	if err != nil {
		return nil, err
	}
	defer treeResp.Body.Close()

	var treeData struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"tree"`
	}
	json.NewDecoder(treeResp.Body).Decode(&treeData)

	var files []File
	yamlCount := 0

	for _, item := range treeData.Tree {
		if item.Type == "blob" {
			// Apply dynamic extension filtering
			if !isAllowedFileExtension(item.Path) {
				continue
			}

			// Only count YAML files for limit
			if parser.IsKubernetesFile(item.Path) {
				yamlCount++
			}

			var shouldFetch bool
			var githubCommitTime time.Time
			var githubCommitSHA string

			if isFirstTimeFetch {
				// FIRST-TIME: Fetch all allowed files (limit to 50 YAML files)
				shouldFetch = yamlCount <= 50
				if shouldFetch {
					githubCommitTime, githubCommitSHA = fetchFileCommitTime(fullRepoName, item.Path, pat)
				}
			} else {
				// SECOND-TIME: Apply new incremental sync logic
				// Step 1: Fetch commit time and SHA from GitHub for this file
				githubCommitTime, githubCommitSHA = fetchFileCommitTime(fullRepoName, item.Path, pat)

				// Step 2: Get DB values
				var dbCommitSHA, dbContentHash string
				var dbCommitTime time.Time
				err := database.DB.QueryRow(`
					SELECT commit_sha, content_hash, commit_time 
					FROM github_files 
					WHERE repo_id = $1 AND org_id = $2 AND file_path = $3
				`, repoID, orgID, item.Path).Scan(&dbCommitSHA, &dbContentHash, &dbCommitTime)

				if err != nil {
					// File not in DB - NEW FILE
					shouldFetch = true
					fmt.Printf("      🆕 NEW FILE: %s\n", item.Path)
				} else {
					// Step 3: Generate GitHub content hash
					githubContentHash := generateFileHash(item.Path, item.SHA)

					// Step 4: Apply new comparison logic
					if githubCommitTime.Equal(dbCommitTime) {
						// Commit time same → IGNORE
						shouldFetch = false
						fmt.Printf("      ✓ SAME COMMIT TIME: %s\n", item.Path)
					} else if githubCommitSHA == dbCommitSHA {
						// Commit SHA same → IGNORE
						shouldFetch = false
						fmt.Printf("      ✓ SAME COMMIT SHA: %s\n", item.Path)
					} else if githubContentHash == dbContentHash {
						// Content hash same → IGNORE
						shouldFetch = false
						fmt.Printf("      ✓ SAME CONTENT HASH: %s\n", item.Path)
					} else {
						// Everything different → FETCH
						shouldFetch = true
						fmt.Printf("      🔄 FILE CHANGED: %s\n", item.Path)
					}
				}
			}

			if shouldFetch {
				// Fetch commit time and SHA only (no content needed)
				githubCommitTime, githubCommitSHA = fetchFileCommitTime(fullRepoName, item.Path, pat)
				fmt.Printf("      ✅ Tracked: %s\n", item.Path)
				files = append(files, File{
					Path:       item.Path,
					SHA:        item.SHA,
					CommitSHA:  githubCommitSHA,
					CommitTime: githubCommitTime,
				})
			} else {
				// Don't fetch - file unchanged
				files = append(files, File{
					Path:       item.Path,
					SHA:        item.SHA,
					CommitSHA:  "",
					CommitTime: time.Time{}, // Zero time indicates no update needed
				})
			}
		}
	}

	fmt.Printf("      📊 Total files: %d, YAML files: %d, Tracked: %d\n", len(files), yamlCount, len(files))
	return files, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// fetchFileCommitTime fetches the real commit time and commit SHA for a file from GitHub
func fetchFileCommitTime(fullRepoName, filePath, pat string) (time.Time, string) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Get the latest commit for this file
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/commits?path=%s&per_page=1", fullRepoName, filePath), nil)
	if err != nil {
		fmt.Printf("         ❌ Failed to create commit request for %s: %v\n", filePath, err)
		istLocation := time.FixedZone("IST", 5*60*60+30*60)
		return time.Now().In(istLocation), ""
	}

	req.Header.Set("Authorization", "token "+pat)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("         ❌ Failed to fetch commit time for %s: %v\n", filePath, err)
		istLocation := time.FixedZone("IST", 5*60*60+30*60)
		return time.Now().In(istLocation), ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("         ❌ GitHub API returned %d for commit time of %s\n", resp.StatusCode, filePath)
		istLocation := time.FixedZone("IST", 5*60*60+30*60)
		return time.Now().In(istLocation), ""
	}

	var commits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Committer struct {
				Date time.Time `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		fmt.Printf("         ❌ Failed to parse commit time for %s: %v\n", filePath, err)
		istLocation := time.FixedZone("IST", 5*60*60+30*60)
		return time.Now().In(istLocation), ""
	}

	if len(commits) > 0 {
		commitSHA := commits[0].SHA
		commitTimeUTC := commits[0].Commit.Committer.Date
		// Convert UTC to IST (UTC + 5:30)
		istLocation := time.FixedZone("IST", 5*60*60+30*60)
		commitTimeIST := commitTimeUTC.In(istLocation)
		fmt.Printf("         ⏰ Real commit SHA: %s, time (IST): %s\n", commitSHA, commitTimeIST.Format("2006-01-02 15:04:05"))
		return commitTimeIST, commitSHA
	}

	// Return current time in IST
	istLocation := time.FixedZone("IST", 5*60*60+30*60)
	return time.Now().In(istLocation), ""
}

// storeFilesAndExtractK8s stores file metadata in database (no content storage)
func storeFilesAndExtractK8s(files []File, repoID, orgID, fullRepoName string) int {
	for _, file := range files {
		if file.CommitTime.IsZero() {
			continue
		}

		commitTime := file.CommitTime
		if commitTime.IsZero() {
			istLocation := time.FixedZone("IST", 5*60*60+30*60)
			commitTime = time.Now().In(istLocation)
		}

		commitSHA := file.CommitSHA
		if commitSHA == "" {
			commitSHA = file.SHA
		}

		var existingFileID int
		var existingCommitSHA, existingContentHash string
		err := database.DB.QueryRow(`
			SELECT id, commit_sha, content_hash 
			FROM github_files 
			WHERE repo_id = $1 AND org_id = $2 AND file_path = $3
		`, repoID, orgID, file.Path).Scan(&existingFileID, &existingCommitSHA, &existingContentHash)

		if err != nil {
			database.DB.QueryRow(`
				INSERT INTO github_files (org_id, repo_id, branch, file_path, commit_sha, content_hash, commit_time, is_deleted)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				RETURNING id
			`, orgID, repoID, "main", file.Path, commitSHA, generateFileHash(file.Path, file.SHA), commitTime, false).Scan(&existingFileID)
		} else {
			newContentHash := generateFileHash(file.Path, file.SHA)
			if commitSHA != existingCommitSHA || newContentHash != existingContentHash {
				database.DB.Exec(`
					UPDATE github_files 
					SET commit_sha = $1, content_hash = $2, commit_time = $3
					WHERE id = $4
				`, commitSHA, newContentHash, commitTime, existingFileID)
				fmt.Printf("      🔄 Updated file: %s\n", file.Path)
			}
		}
	}

	return 0
}

// generateRepoID generates a unique repository ID
func generateRepoID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
}

type Repository struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"` // owner/repo format
	URL      string `json:"html_url"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type File struct {
	Path       string
	SHA        string // Blob SHA (for backward compatibility)
	CommitSHA  string // Actual commit SHA
	CommitTime time.Time
}

// generateFileHash generates a deterministic hash for file based on path and SHA
func generateFileHash(path string, sha string) string {
	// Use path + SHA to create deterministic hash
	input := fmt.Sprintf("%s|%s", path, sha)
	hash := uint32(0)
	for _, char := range input {
		hash = hash*31 + uint32(char)
	}
	return fmt.Sprintf("hash_%06d", hash%1000000)
}

// generateResourceID generates a deterministic resource ID based on file and resource info
func generateResourceID(fileID int, kind, name, namespace string) string {
	// Create deterministic ID: hash(fileID + kind + name + namespace)
	// This ensures same resource from same file always gets same 6-digit ID
	input := fmt.Sprintf("%d|%s|%s|%s", fileID, kind, name, namespace)
	hash := uint32(0)
	for _, char := range input {
		hash = hash*31 + uint32(char)
	}
	// Return 6-digit ID (000000-999999)
	return fmt.Sprintf("%06d", hash%1000000)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// isAllowedFileExtension checks if a file should be fetched based on ALLOWED_EXTENSIONS env var
func isAllowedFileExtension(filePath string) bool {
	allowedExts := os.Getenv("ALLOWED_EXTENSIONS")

	// If empty or not set, allow all files
	if allowedExts == "" {
		return true
	}

	// Parse comma-separated extensions
	extensions := strings.Split(allowedExts, ",")

	// Get file extension
	parts := strings.Split(filePath, ".")
	if len(parts) < 2 {
		return false // No extension
	}
	fileExt := "." + strings.ToLower(parts[len(parts)-1])

	// Check if file extension is in allowed list
	for _, ext := range extensions {
		ext = strings.TrimSpace(ext)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if strings.ToLower(ext) == fileExt {
			return true
		}
	}

	return false
}
