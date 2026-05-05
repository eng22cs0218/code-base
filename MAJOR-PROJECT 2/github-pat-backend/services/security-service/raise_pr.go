package securityservice

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github-pat-backend/pkg/database"
	ghclient "github-pat-backend/pkg/github"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// --------------------------------------------------------------------------
// List Branches — so frontend can show a branch picker
// --------------------------------------------------------------------------

type ListBranchesRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
}

type BranchItem struct {
	Name string `json:"name"`
	SHA  string `json:"sha"`
}

func ListBranches(c *gin.Context) {
	var req ListBranchesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "session_token is required", err)
		return
	}

	client, _, err := buildGitHubClient(req.SessionToken)
	if err != nil {
		handleClientError(c, err)
		return
	}

	branches, err := client.ListBranches()
	if err != nil {
		response.InternalError(c, "failed to list branches from GitHub", err)
		return
	}

	items := make([]BranchItem, len(branches))
	for i, b := range branches {
		items[i] = BranchItem{Name: b.Name, SHA: b.Commit.SHA}
	}

	response.Success(c, http.StatusOK, "branches fetched", gin.H{
		"branches": items,
	})
}

// --------------------------------------------------------------------------
// Raise PR — create branch, commit fixed YAML, open PR
// --------------------------------------------------------------------------

type RaisePRRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
	ResourceID   string `json:"resource_id" binding:"required"`
	FixedYAML    string `json:"fixed_yaml" binding:"required"`
	TargetBranch string `json:"target_branch" binding:"required"` // user-selected base branch
	Title        string `json:"title"`
	Description  string `json:"description"`
}

type RaisePRResponse struct {
	PRURL      string `json:"pr_url"`
	PRNumber   int    `json:"pr_number"`
	BranchName string `json:"branch_name"`
	Status     string `json:"status"`
}

func RaisePR(c *gin.Context) {
	var req RaisePRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "session_token, resource_id, fixed_yaml, and target_branch are required", err)
		return
	}

	client, orgID, err := buildGitHubClient(req.SessionToken)
	if err != nil {
		handleClientError(c, err)
		return
	}

	// ------------------------------------------------------------------
	// 1. Resolve the resource → find its source file path in the repo
	// ------------------------------------------------------------------
	var resourceID, kind, name, namespace string
	var repoFileID *int
	err = database.DB.QueryRow(`
		SELECT resource_id, kind, name, namespace, repo_file_id
		FROM kubernetes_resource
		WHERE org_id = $1 AND resource_id = $2
	`, orgID, req.ResourceID).Scan(&resourceID, &kind, &name, &namespace, &repoFileID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "resource not found", err)
		return
	}

	// Find the file path in the repo
	filePath := ""
	if repoFileID != nil {
		_ = database.DB.QueryRow(`
			SELECT file_path FROM github_files WHERE id = $1
		`, *repoFileID).Scan(&filePath)
	}

	// Fallback: construct a path from kind/name
	if filePath == "" {
		filePath = fmt.Sprintf("k8s/%s/%s-%s.yaml", namespace, strings.ToLower(kind), name)
	}

	// ------------------------------------------------------------------
	// 2. Create a fix branch from the target branch
	// ------------------------------------------------------------------
	fixBranch := fmt.Sprintf("aegios/fix-%s-%s-%d", strings.ToLower(kind), name, time.Now().Unix())

	baseSHA, err := client.GetBranchSHA(req.TargetBranch)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to get SHA for branch '%s'", req.TargetBranch), err)
		return
	}

	if err := client.CreateBranch(fixBranch, baseSHA); err != nil {
		response.InternalError(c, "failed to create fix branch", err)
		return
	}

	// ------------------------------------------------------------------
	// 3. Get existing file SHA (if it exists) — needed by GitHub's update API
	// ------------------------------------------------------------------
	var existingSHA string
	fc, err := client.GetFileContent(filePath, req.TargetBranch)
	if err == nil && fc != nil {
		existingSHA = fc.SHA
	}
	// If file doesn't exist yet, SHA is empty → GitHub will create a new file

	// ------------------------------------------------------------------
	// 4. Commit the fixed YAML to the fix branch
	// ------------------------------------------------------------------
	commitMsg := req.Title
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("fix(%s): remediate security finding for %s/%s", namespace, kind, name)
	}

	encodedContent := base64.StdEncoding.EncodeToString([]byte(req.FixedYAML))

	updateReq := ghclient.UpdateFileRequest{
		Message: commitMsg,
		Content: encodedContent,
		Branch:  fixBranch,
	}
	if existingSHA != "" {
		updateReq.SHA = existingSHA
	}

	if err := client.UpdateFile(filePath, updateReq); err != nil {
		response.InternalError(c, "failed to commit fixed YAML", err)
		return
	}

	// ------------------------------------------------------------------
	// 5. Open a Pull Request: fix branch → target branch
	// ------------------------------------------------------------------
	prTitle := req.Title
	if prTitle == "" {
		prTitle = fmt.Sprintf("[Aegios] Fix %s/%s in %s", kind, name, namespace)
	}

	prBody := req.Description
	if prBody == "" {
		prBody = fmt.Sprintf(
			"## 🛡️ Aegios Security Remediation\n\n"+
				"**Resource:** `%s/%s`\n"+
				"**Namespace:** `%s`\n"+
				"**Kind:** `%s`\n\n"+
				"This PR was generated by [Aegios](https://github.com/aegios) to remediate a Kubernetes security finding.\n\n"+
				"### Changes\n"+
				"- Applied security fix to `%s`\n\n"+
				"---\n"+
				"*Auto-generated by Aegios K8s Security Platform*",
			kind, name, namespace, kind, filePath,
		)
	}

	pr, err := client.CreatePR(ghclient.CreatePRRequest{
		Title: prTitle,
		Body:  prBody,
		Head:  fixBranch,
		Base:  req.TargetBranch,
	})
	if err != nil {
		response.InternalError(c, "failed to create pull request", err)
		return
	}

	// ------------------------------------------------------------------
	// 6. Return the PR URL to frontend
	// ------------------------------------------------------------------
	response.Success(c, http.StatusOK, "pull request created successfully", RaisePRResponse{
		PRURL:      pr.HTMLURL,
		PRNumber:   pr.Number,
		BranchName: fixBranch,
		Status:     "created",
	})
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// buildGitHubClient validates the session, resolves credentials, and returns
// a ready-to-use GitHub client.
func buildGitHubClient(sessionToken string) (*ghclient.Client, string, error) {
	// Validate session
	githubUsername, err := database.ValidateSession(sessionToken)
	if err != nil {
		return nil, "", fmt.Errorf("auth:%w", err)
	}

	// Get org ID
	orgID, err := database.GetUserOrgID(githubUsername)
	if err != nil {
		return nil, "", fmt.Errorf("org:%w", err)
	}

	// Get PAT
	var encryptedPAT string
	err = database.DB.QueryRow(`
		SELECT encrypted_pat FROM github_credentials
		WHERE org_id = $1 AND github_username = $2
	`, orgID, githubUsername).Scan(&encryptedPAT)
	if err != nil {
		return nil, "", fmt.Errorf("pat:%w", err)
	}

	patBytes, _ := hex.DecodeString(encryptedPAT)
	pat := string(patBytes)

	// Get repo info
	var repoName, repoURL string
	err = database.DB.QueryRow(`
		SELECT repo_name, repo_url FROM github_repository
		WHERE org_id = $1 LIMIT 1
	`, orgID).Scan(&repoName, &repoURL)
	if err != nil {
		return nil, "", fmt.Errorf("repo:%w", err)
	}

	// Parse owner/repo
	owner, repo := parseRepoFullName(repoURL, githubUsername, repoName)

	return ghclient.NewClient(pat, owner, repo), orgID, nil
}

// parseRepoFullName extracts owner and repo from a GitHub URL.
func parseRepoFullName(repoURL, fallbackOwner, fallbackRepo string) (string, string) {
	// Try to parse URL like https://github.com/owner/repo
	if strings.Contains(repoURL, "github.com/") {
		parts := strings.Split(repoURL, "github.com/")
		if len(parts) > 1 {
			fullName := strings.TrimSuffix(parts[1], ".git")
			ownerRepo := strings.SplitN(fullName, "/", 2)
			if len(ownerRepo) == 2 {
				return ownerRepo[0], ownerRepo[1]
			}
		}
	}
	return fallbackOwner, fallbackRepo
}

// handleClientError maps internal error prefixes to proper HTTP responses.
func handleClientError(c *gin.Context, err error) {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "auth:"):
		response.Unauthorized(c, "invalid or expired session", err)
	case strings.HasPrefix(msg, "org:"):
		response.InternalError(c, "failed to get user organization", err)
	case strings.HasPrefix(msg, "pat:"):
		response.InternalError(c, "failed to get GitHub credentials", err)
	case strings.HasPrefix(msg, "repo:"):
		response.InternalError(c, "no repository found for this organization", err)
	default:
		response.InternalError(c, "internal error", err)
	}
}
