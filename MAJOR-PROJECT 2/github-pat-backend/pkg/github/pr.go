package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiBase = "https://api.github.com"

// Client wraps GitHub REST API calls authenticated with a PAT.
type Client struct {
	PAT        string
	Owner      string
	Repo       string
	httpClient *http.Client
}

// NewClient creates a new GitHub API client.
func NewClient(pat, owner, repo string) *Client {
	return &Client{
		PAT:   pat,
		Owner: owner,
		Repo:  repo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// -------------------------------------------------------------------
// List Branches
// -------------------------------------------------------------------

type Branch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// ListBranches returns all branches for the repository.
func (c *Client) ListBranches() ([]Branch, error) {
	var allBranches []Branch
	page := 1

	for {
		url := fmt.Sprintf("%s/repos/%s/%s/branches?per_page=100&page=%d", apiBase, c.Owner, c.Repo, page)
		body, err := c.doGet(url)
		if err != nil {
			return nil, fmt.Errorf("list branches: %w", err)
		}

		var branches []Branch
		if err := json.Unmarshal(body, &branches); err != nil {
			return nil, fmt.Errorf("parse branches: %w", err)
		}

		if len(branches) == 0 {
			break
		}

		allBranches = append(allBranches, branches...)
		if len(branches) < 100 {
			break
		}
		page++
	}

	return allBranches, nil
}

// -------------------------------------------------------------------
// Get File Content (returns content + SHA needed for update)
// -------------------------------------------------------------------

type FileContent struct {
	Content  string `json:"content"`  // base64
	SHA      string `json:"sha"`
	Path     string `json:"path"`
	Encoding string `json:"encoding"`
}

// GetFileContent retrieves a file's metadata + SHA from a specific branch.
func (c *Client) GetFileContent(path, branch string) (*FileContent, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", apiBase, c.Owner, c.Repo, path, branch)
	body, err := c.doGet(url)
	if err != nil {
		return nil, fmt.Errorf("get file content: %w", err)
	}

	var fc FileContent
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, fmt.Errorf("parse file content: %w", err)
	}

	return &fc, nil
}

// -------------------------------------------------------------------
// Get Branch SHA
// -------------------------------------------------------------------

// GetBranchSHA returns the latest commit SHA for a branch.
func (c *Client) GetBranchSHA(branch string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/ref/heads/%s", apiBase, c.Owner, c.Repo, branch)
	body, err := c.doGet(url)
	if err != nil {
		return "", fmt.Errorf("get branch SHA: %w", err)
	}

	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.Unmarshal(body, &ref); err != nil {
		return "", fmt.Errorf("parse branch SHA: %w", err)
	}

	return ref.Object.SHA, nil
}

// -------------------------------------------------------------------
// Create Branch
// -------------------------------------------------------------------

// CreateBranch creates a new branch from a source SHA.
func (c *Client) CreateBranch(branchName, sourceSHA string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs", apiBase, c.Owner, c.Repo)
	payload := map[string]string{
		"ref": fmt.Sprintf("refs/heads/%s", branchName),
		"sha": sourceSHA,
	}

	_, err := c.doPost(url, payload)
	if err != nil {
		return fmt.Errorf("create branch: %w", err)
	}

	return nil
}

// -------------------------------------------------------------------
// Update / Create File (commit to a branch)
// -------------------------------------------------------------------

type UpdateFileRequest struct {
	Message string `json:"message"`
	Content string `json:"content"` // base64-encoded
	SHA     string `json:"sha,omitempty"` // required for updates, omit for new files
	Branch  string `json:"branch"`
}

// UpdateFile creates or updates a file on a branch.
func (c *Client) UpdateFile(path string, req UpdateFileRequest) error {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", apiBase, c.Owner, c.Repo, path)

	_, err := c.doPut(url, req)
	if err != nil {
		return fmt.Errorf("update file: %w", err)
	}

	return nil
}

// -------------------------------------------------------------------
// Create Pull Request
// -------------------------------------------------------------------

type CreatePRRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"` // source branch
	Base  string `json:"base"` // target branch
}

type PRResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Title   string `json:"title"`
}

// CreatePR opens a new pull request.
func (c *Client) CreatePR(req CreatePRRequest) (*PRResponse, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls", apiBase, c.Owner, c.Repo)

	body, err := c.doPost(url, req)
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}

	var pr PRResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("parse PR response: %w", err)
	}

	return &pr, nil
}

// -------------------------------------------------------------------
// HTTP helpers
// -------------------------------------------------------------------

func (c *Client) doGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.PAT)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (c *Client) doPost(url string, payload interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.PAT)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (c *Client) doPut(url string, payload interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.PAT)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
