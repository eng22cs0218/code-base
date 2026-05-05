package fetchingservice

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// Optimized HTTP client with connection pooling (Optimization #3)
var optimizedHTTPClient = &http.Client{
	Timeout: 50 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false, // Enable compression	
	},
}

// Worker pool size for parallel processing
const (
	RepoWorkers = 5   // Process 5 repos in parallel
	FileWorkers = 10  // Process 10 files in parallel per repo
	BatchSize   = 100 // Batch insert size
)

// FetchDataOptimized - Ultra-fast optimized version
func FetchDataOptimized(c *gin.Context) {
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

	fmt.Printf("🔍 FetchData (OPTIMIZED): credID=%s, orgID=%s\n", credID, orgID)

	// Get user's GitHub PAT and username
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

	// Decrypt PAT
	patBytes, _ := hex.DecodeString(encryptedPAT)
	pat := string(patBytes)

	// Fetch repositories from GitHub using GraphQL (Optimization #5)
	repos, err := fetchGitHubRepositoriesGraphQL(githubUsername, pat)
	if err != nil {
		fmt.Printf("⚠️  GraphQL failed: %v, falling back to REST API\n", err)
		// Fallback to REST API
		repos, err = fetchGitHubRepositories(githubUsername, pat)
		if err != nil {
			response.InternalError(c, "failed to fetch repositories from GitHub", err)
			return
		}
		fmt.Printf("✅ REST API fallback: Fetched %d repos\n", len(repos))
	}

	totalRepos := len(repos)
	fmt.Printf("📦 Found %d repositories\n", totalRepos)

	// Detect if this is FIRST-TIME or SECOND-TIME fetch
	var repoCount, fileCount int
	database.DB.QueryRow(`SELECT COUNT(*) FROM github_repository WHERE org_id = $1 AND cred_id = $2`, orgID, credID).Scan(&repoCount)
	database.DB.QueryRow(`SELECT COUNT(*) FROM github_files WHERE org_id = $1`, orgID).Scan(&fileCount)

	isFirstTimeFetch := (repoCount == 0 && fileCount == 0)

	if isFirstTimeFetch {
		fmt.Printf("🟢 FIRST-TIME FETCH (OPTIMIZED): Performing full GitHub scan\n")
	} else {
		fmt.Printf("🟡 SECOND-TIME FETCH (OPTIMIZED): Performing incremental sync (repos=%d, files=%d)\n", repoCount, fileCount)
	}

	// Parallel processing with worker pool (Optimization #1)
	var wg sync.WaitGroup
	repoChannel := make(chan Repository, totalRepos)
	resultChannel := make(chan RepoResult, totalRepos)

	// Start worker pool - pass isFirstTimeFetch to workers
	for i := 0; i < RepoWorkers; i++ {
		wg.Add(1)
		go repoWorker(&wg, repoChannel, resultChannel, orgID, credID, pat, isFirstTimeFetch)
	}

	// Feed repos to workers
	for _, repo := range repos {
		repoChannel <- repo
	}
	close(repoChannel)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChannel)
	}()

	// Collect results
	totalScannedFiles := 0
	totalK8sResources := 0
	for result := range resultChannel {
		totalScannedFiles += result.ScannedFiles
		totalK8sResources += result.K8sResources
	}

	fmt.Printf("✅ Fetch completed (OPTIMIZED): repos=%d, files=%d, k8s=%d\n", totalRepos, totalScannedFiles, totalK8sResources)

	fetchResp := FetchDataResponse{
		Status:       "completed",
		Message:      "Data fetching completed successfully (optimized)",
		TotalRepos:   totalRepos,
		ScannedFiles: totalScannedFiles,
		K8sResources: totalK8sResources,
	}

	response.Success(c, http.StatusOK, "fetch data completed", fetchResp)
}

type RepoResult struct {
	RepoName     string
	ScannedFiles int
	K8sResources int
}

// repoWorker processes repositories in parallel
func repoWorker(wg *sync.WaitGroup, repos <-chan Repository, results chan<- RepoResult, orgID, credID, pat string, isFirstTimeFetch bool) {
	defer wg.Done()

	for repo := range repos {
		fmt.Printf("🔄 Processing repo (parallel): %s\n", repo.FullName)

		// Check for incremental sync (Optimization #6)
		lastSync, repoID := getLastSyncInfo(repo.Name, orgID)

		if repoID == "" {
			// New repository
			repoID = generateRepoID()
			_, err := database.DB.Exec(`
				INSERT INTO github_repository (repo_id, org_id, cred_id, repo_name, repo_url, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, repoID, orgID, credID, repo.Name, repo.URL, time.Now())

			if err != nil {
				fmt.Printf("   ❌ Failed to insert repo: %v\n", err)
				continue
			}
			fmt.Printf("   ✅ Created new repo_id: %s\n", repoID)
		} else {
			fmt.Printf("   ♻️  Using existing repo_id: %s (last sync: %v)\n", repoID, lastSync)
		}

		// Scan files with parallel fetching and incremental sync
		files, err := scanRepositoryFilesParallel(repo.FullName, pat, isFirstTimeFetch, repoID, orgID)
		if err != nil {
			fmt.Printf("   ❌ Failed to scan files: %v\n", err)
			continue
		}

		fmt.Printf("   📁 Found %d files\n", len(files))

		// Batch insert files and extract K8s resources (Optimization #2)
		k8sCount := storeFilesAndExtractK8sBatch(files, repoID, orgID)

		fmt.Printf("   ☸️  Extracted %d K8s resources\n", k8sCount)

		// Update last sync timestamp
		updateLastSync(repoID)

		results <- RepoResult{
			RepoName:     repo.Name,
			ScannedFiles: len(files),
			K8sResources: k8sCount,
		}
	}
}

// fetchGitHubRepositoriesGraphQL uses GraphQL API for faster fetching (Optimization #5)
func fetchGitHubRepositoriesGraphQL(username, pat string) ([]Repository, error) {
	query := `
	{
		viewer {
			repositories(first: 100, orderBy: {field: UPDATED_AT, direction: DESC}) {
				nodes {
					name
					nameWithOwner
					url
					defaultBranchRef {
						name
					}
				}
			}
		}
	}`

	reqBody := map[string]string{"query": query}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", "https://api.github.com/graphql", strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "bearer "+pat)
	req.Header.Set("Content-Type", "application/json")

	resp, err := optimizedHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("❌ GraphQL API error (status %d): %s\n", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("GraphQL API returned status %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Viewer struct {
				Repositories struct {
					Nodes []struct {
						Name          string `json:"name"`
						NameWithOwner string `json:"nameWithOwner"`
						URL           string `json:"url"`
					} `json:"nodes"`
				} `json:"repositories"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		fmt.Printf("❌ GraphQL parse error: %v\n", err)
		return nil, err
	}

	// Check for GraphQL errors
	if len(result.Errors) > 0 {
		fmt.Printf("❌ GraphQL returned errors: %v\n", result.Errors[0].Message)
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	var repos []Repository
	for _, node := range result.Data.Viewer.Repositories.Nodes {
		repos = append(repos, Repository{
			Name:     node.Name,
			FullName: node.NameWithOwner,
			URL:      node.URL,
		})
	}

	fmt.Printf("📊 GraphQL API: Fetched %d repos in single query\n", len(repos))

	// If GraphQL returns 0 repos, return error to trigger REST fallback
	if len(repos) == 0 {
		fmt.Printf("⚠️  GraphQL returned 0 repos, will fallback to REST API\n")
		return nil, fmt.Errorf("GraphQL returned 0 repositories")
	}

	return repos, nil
}

// scanRepositoryFilesParallel fetches files in parallel with incremental sync (Optimization #1)
func scanRepositoryFilesParallel(fullRepoName, pat string, isFirstTimeFetch bool, repoID, orgID string) ([]File, error) {
	// Get file tree
	treeReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/git/trees/HEAD?recursive=1", fullRepoName), nil)
	treeReq.Header.Set("Authorization", "token "+pat)
	treeReq.Header.Set("Accept", "application/vnd.github.v3+json")

	treeResp, err := optimizedHTTPClient.Do(treeReq)
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

	// Filter YAML files and apply incremental sync logic
	var filesToFetch []struct {
		Path             string
		SHA              string
		ShouldFetch      bool
		GitHubCommitTime time.Time
		GitHubCommitSHA  string
	}

	for _, item := range treeData.Tree {
		if item.Type == "blob" && isAllowedFileExtension(item.Path) {
			var shouldFetch bool
			var githubCommitTime time.Time
			var githubCommitSHA string

			if isFirstTimeFetch {
				// FIRST-TIME: Fetch all files (limit to 100)
				shouldFetch = len(filesToFetch) < 100
				if shouldFetch {
					githubCommitTime, githubCommitSHA = fetchFileCommitTimeOptimized(fullRepoName, item.Path, pat)
				}
			} else {
				// SECOND-TIME: Apply new incremental sync logic
				// Step 1: Fetch commit time and SHA from GitHub for this file
				githubCommitTime, githubCommitSHA = fetchFileCommitTimeOptimized(fullRepoName, item.Path, pat)

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

			filesToFetch = append(filesToFetch, struct {
				Path             string
				SHA              string
				ShouldFetch      bool
				GitHubCommitTime time.Time
				GitHubCommitSHA  string
			}{Path: item.Path, SHA: item.SHA, ShouldFetch: shouldFetch, GitHubCommitTime: githubCommitTime, GitHubCommitSHA: githubCommitSHA})
		}
	}

	// Parallel file fetching for files that need to be fetched
	var wg sync.WaitGroup
	fileChannel := make(chan struct {
		Path             string
		SHA              string
		ShouldFetch      bool
		GitHubCommitTime time.Time
		GitHubCommitSHA  string
	}, len(filesToFetch))
	resultChannel := make(chan File, len(filesToFetch))

	// Start file workers
	for i := 0; i < FileWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fileInfo := range fileChannel {
				// Only track metadata, no content fetching
				resultChannel <- File{
					Path:       fileInfo.Path,
					SHA:        fileInfo.SHA,
					CommitSHA:  fileInfo.GitHubCommitSHA,
					CommitTime: fileInfo.GitHubCommitTime,
				}
			}
		}()
	}

	// Feed files to workers
	for _, fileInfo := range filesToFetch {
		fileChannel <- fileInfo
	}
	close(fileChannel)

	// Wait and collect results
	go func() {
		wg.Wait()
		close(resultChannel)
	}()

	var files []File
	for file := range resultChannel {
		files = append(files, file)
		fmt.Printf("      ✅ Tracked: %s\n", file.Path)
	}

	fmt.Printf("      📊 Total files tracked: %d\n", len(files))
	return files, nil
}

func countFilesWithContent(files []File) int {
	// No longer needed but keeping for compatibility
	return len(files)
}

// fetchFileContentOptimized uses streaming parser (Optimization #7)
func fetchFileContentOptimized(fullRepoName, filePath, pat string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", fullRepoName, filePath), nil)
	if err != nil {
		return ""
	}

	req.Header.Set("Authorization", "token "+pat)
	req.Header.Set("Accept", "application/vnd.github.v3.raw")

	resp, err := optimizedHTTPClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	// Stream content directly
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	return string(content)
}

// fetchFileCommitTimeOptimized fetches the real commit time and commit SHA for a file from GitHub
func fetchFileCommitTimeOptimized(fullRepoName, filePath, pat string) (time.Time, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get the latest commit for this file
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.github.com/repos/%s/commits?path=%s&per_page=1", fullRepoName, filePath), nil)
	if err != nil {
		fmt.Printf("         ❌ Failed to create commit request for %s: %v\n", filePath, err)
		istLocation := time.FixedZone("IST", 5*60*60+30*60)
		return time.Now().In(istLocation), ""
	}

	req.Header.Set("Authorization", "token "+pat)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := optimizedHTTPClient.Do(req)
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

// storeFilesAndExtractK8sBatch uses batch inserts (Optimization #2)
func storeFilesAndExtractK8sBatch(files []File, repoID, orgID string) int {
	if len(files) == 0 {
		return 0
	}

	// Batch insert files
	fileIDs := batchInsertFiles(files, repoID, orgID)

	// Batch process K8s resources
	k8sCount := batchExtractAndStoreK8sResources(files, fileIDs, orgID)

	return k8sCount
}

// batchInsertFiles inserts files in batches (Optimization #2)
func batchInsertFiles(files []File, repoID, orgID string) map[string]int {
	fileIDs := make(map[string]int)

	for _, file := range files {
		// Skip files with zero commit time (unchanged files)
		if file.CommitTime.IsZero() {
			continue
		}

		// Use real commit time from GitHub, fallback to time.Now() only if not set
		commitTime := file.CommitTime
		if commitTime.IsZero() {
			istLocation := time.FixedZone("IST", 5*60*60+30*60)
			commitTime = time.Now().In(istLocation)
		}

		// Use actual commit SHA, fallback to blob SHA if not available
		commitSHA := file.CommitSHA
		if commitSHA == "" {
			commitSHA = file.SHA // Fallback to blob SHA
		}

		// Check if file exists in DB
		var existingFileID int
		var existingCommitSHA, existingContentHash string
		err := database.DB.QueryRow(`
			SELECT id, commit_sha, content_hash 
			FROM github_files 
			WHERE repo_id = $1 AND org_id = $2 AND file_path = $3
		`, repoID, orgID, file.Path).Scan(&existingFileID, &existingCommitSHA, &existingContentHash)

		var fileID int
		if err != nil {
			// File doesn't exist - INSERT
			err = database.DB.QueryRow(`
				INSERT INTO github_files (org_id, repo_id, branch, file_path, commit_sha, content_hash, commit_time, is_deleted)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				RETURNING id
			`, orgID, repoID, "main", file.Path, commitSHA, generateFileHash(file.Path, file.SHA), commitTime, false).Scan(&fileID)

			if err != nil {
				fmt.Printf("      ❌ Failed to insert file: %v\n", err)
				continue
			}
		} else {
			// File exists - UPDATE if changed
			newContentHash := generateFileHash(file.Path, file.SHA)
			if commitSHA != existingCommitSHA || newContentHash != existingContentHash {
				_, err = database.DB.Exec(`
					UPDATE github_files 
					SET commit_sha = $1, content_hash = $2, commit_time = $3
					WHERE id = $4
				`, commitSHA, newContentHash, commitTime, existingFileID)

				if err != nil {
					fmt.Printf("      ❌ Failed to update file: %v\n", err)
					continue
				}
				fmt.Printf("      🔄 Updated file: %s\n", file.Path)
			}
			fileID = existingFileID
		}

		fileIDs[file.Path] = fileID
	}

	fmt.Printf("      ✅ Processed %d files (insert/update)\n", len(fileIDs))
	return fileIDs
}

// batchExtractAndStoreK8sResources removed - K8s parsing now done in Validation endpoint
func batchExtractAndStoreK8sResources(files []File, fileIDs map[string]int, orgID string) int {
	return 0
}

type K8sResourceBatchItem struct {
	ResourceID  string
	OrgID       string
	RepoFileID  int
	ClusterID   string
	Kind        string
	Name        string
	Namespace   string
	YAMLContent string
}

// batchInsertK8sResources performs batch insert of K8s resources with upsert
func batchInsertK8sResources(batch []K8sResourceBatchItem) int {
	if len(batch) == 0 {
		return 0
	}

	valueStrings := make([]string, 0, len(batch))
	valueArgs := make([]interface{}, 0, len(batch)*9)

	for i, res := range batch {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*9+1, i*9+2, i*9+3, i*9+4, i*9+5, i*9+6, i*9+7, i*9+8, i*9+9))
		valueArgs = append(valueArgs, res.ResourceID, res.OrgID, res.RepoFileID,
			res.ClusterID, res.Kind, res.Name, res.Namespace, res.YAMLContent, time.Now())
	}

	query := fmt.Sprintf(`
		INSERT INTO kubernetes_resource (resource_id, org_id, repo_file_id, cluster_id, kind, name, namespace, yaml_content, created_at)
		VALUES %s
		ON CONFLICT (resource_id) DO UPDATE SET
			yaml_content = EXCLUDED.yaml_content,
			created_at = EXCLUDED.created_at
	`, strings.Join(valueStrings, ","))

	result, err := database.DB.Exec(query, valueArgs...)
	if err != nil {
		fmt.Printf("         ❌ Batch insert K8s resources failed: %v\n", err)
		return 0
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("         ✅ Batch inserted/updated %d K8s resources\n", rowsAffected)
	return int(rowsAffected)
}

// getLastSyncInfo retrieves last sync timestamp for incremental updates (Optimization #6)
func getLastSyncInfo(repoName, orgID string) (time.Time, string) {
	var lastSync time.Time
	var repoID string

	err := database.DB.QueryRow(`
		SELECT repo_id, created_at 
		FROM github_repository 
		WHERE repo_name = $1 AND org_id = $2
	`, repoName, orgID).Scan(&repoID, &lastSync)

	if err != nil {
		return time.Time{}, ""
	}

	return lastSync, repoID
}

// updateLastSync updates the last sync timestamp
func updateLastSync(repoID string) {
	database.DB.Exec(`
		UPDATE github_repository 
		SET created_at = $1 
		WHERE repo_id = $2
	`, time.Now(), repoID)
}
