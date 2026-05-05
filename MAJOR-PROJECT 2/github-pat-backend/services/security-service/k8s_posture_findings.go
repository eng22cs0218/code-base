package securityservice

import (
	"net/http"
	"strings"
	"time"

	"github-pat-backend/pkg/database"
	"github-pat-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type PostureFindingsRequest struct {
	SessionToken string `json:"session_token" binding:"required"`
}

type PostureFinding struct {
	FindingID      string    `json:"finding_id"`
	ResourceID     string    `json:"resource_id,omitempty"`
	Namespace      string    `json:"namespace"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	RepoName       string    `json:"repo_name,omitempty"`
	MissingKind    string    `json:"missing_kind,omitempty"`
	IssueType      string    `json:"issue_type"`
	CheckName      string    `json:"check_name"`
	Severity       string    `json:"severity"`
	Description    string    `json:"description"`
	Recommendation string    `json:"recommendation"`
	DetectedAt     time.Time `json:"detected_at"`
}

type PostureFindingsResponse struct {
	TotalFindings int              `json:"total_findings"`
	Findings      []PostureFinding `json:"findings"`
}

// GetPostureFindings returns findings table entries enriched with resource context.
func GetPostureFindings(c *gin.Context) {
	var req PostureFindingsRequest
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
		response.InternalError(c, "failed to get user organization", err)
		return
	}

	rows, err := database.DB.Query(`
		SELECT
			f.finding_id,
			COALESCE(f.resource_id, krx.resource_id, ''),
			COALESCE(f.namespace, krx.namespace, 'default'),
			COALESCE(krx.name, 'Unknown Resource'),
			COALESCE(NULLIF(f.missing_kind, ''), krx.kind, 'Unknown'),
			COALESCE(krx.repo_name, ''),
			COALESCE(f.missing_kind, ''),
			COALESCE(LOWER(f.severity), 'low'),
			COALESCE(f.description, ''),
			COALESCE(f.recommendations, ''),
			f.detected_at
		FROM findings f
		LEFT JOIN LATERAL (
			SELECT
				kr.resource_id,
				kr.namespace,
				kr.name,
				kr.kind,
				COALESCE(gr.repo_name, '') AS repo_name
			FROM kubernetes_resource kr
			LEFT JOIN github_files gf ON kr.repo_file_id = gf.id
			LEFT JOIN github_repository gr ON gf.repo_id = gr.repo_id
			WHERE kr.org_id = f.org_id
			  AND (
				(f.resource_id IS NOT NULL AND f.resource_id <> '' AND kr.resource_id = f.resource_id)
				OR
				(f.namespace IS NOT NULL AND f.namespace <> '' AND kr.namespace = f.namespace AND f.missing_kind IS NOT NULL AND f.missing_kind <> '' AND LOWER(kr.kind) = LOWER(f.missing_kind))
				OR
				(f.namespace IS NOT NULL AND f.namespace <> '' AND kr.namespace = f.namespace)
			  )
			ORDER BY
				CASE
					WHEN f.resource_id IS NOT NULL AND f.resource_id <> '' AND kr.resource_id = f.resource_id THEN 0
					WHEN f.namespace IS NOT NULL AND f.namespace <> '' AND kr.namespace = f.namespace AND f.missing_kind IS NOT NULL AND f.missing_kind <> '' AND LOWER(kr.kind) = LOWER(f.missing_kind) THEN 1
					WHEN f.namespace IS NOT NULL AND f.namespace <> '' AND kr.namespace = f.namespace THEN 2
					ELSE 3
				END,
				kr.created_at DESC
			LIMIT 1
		) krx ON TRUE
		WHERE f.org_id = $1
		ORDER BY f.detected_at DESC
	`, orgID)
	if err != nil {
		response.InternalError(c, "failed to fetch findings", err)
		return
	}
	defer rows.Close()

	findings := make([]PostureFinding, 0)
	for rows.Next() {
		var item PostureFinding
		if err := rows.Scan(
			&item.FindingID,
			&item.ResourceID,
			&item.Namespace,
			&item.Name,
			&item.Kind,
			&item.RepoName,
			&item.MissingKind,
			&item.Severity,
			&item.Description,
			&item.Recommendation,
			&item.DetectedAt,
		); err != nil {
			continue
		}

		item.IssueType = deriveIssueType(item.Kind, item.MissingKind, item.Description, item.Recommendation)
		item.CheckName = getCheckName(item.IssueType)
		findings = append(findings, item)
	}

	response.Success(c, http.StatusOK, "k8s posture findings fetched successfully", PostureFindingsResponse{
		TotalFindings: len(findings),
		Findings:      findings,
	})
}

func deriveIssueType(kind, missingKind, description, recommendation string) string {
	desc := strings.ToLower(description)
	reco := strings.ToLower(recommendation)
	kindLower := strings.ToLower(kind)
	missingKindLower := strings.ToLower(missingKind)
	blob := strings.ToLower(strings.Join([]string{kindLower, missingKindLower, desc, reco}, " "))

	// Explicitly map missing kinds to their frontend UI categories
	if missingKindLower == "role" || missingKindLower == "clusterrole" || missingKindLower == "rolebinding" || missingKindLower == "clusterrolebinding" || missingKindLower == "serviceaccount" {
		return "rbac"
	}
	if missingKindLower == "service" {
		return "service-port"
	}
	if missingKindLower == "secret" {
		return "secrets"
	}

	// Container security check MUST come before the deployment/resource-limit
	// mapping, because container security findings also store "Deployment" as
	// missing_kind. Without this ordering, they would be mislabeled as
	// resource-limit.
	if strings.Contains(desc, "securitycontext") || strings.Contains(desc, "privileged mode") || strings.Contains(desc, "running as root") || strings.Contains(desc, "allow privilege escalation") || strings.Contains(desc, "allows privilege escalation") || strings.Contains(blob, "runasuser") || strings.Contains(desc, "no securitycontext defined") {
		return "container-security"
	}

	if missingKindLower == "deployment" || missingKindLower == "limitrange" {
		return "resource-limit"
	}

	// 1) RBAC check
	if strings.Contains(blob, "rbac") || strings.Contains(blob, "rolebinding") || strings.Contains(blob, "clusterrole") {
		return "rbac"
	}

	// 2) Network policy check
	if strings.Contains(blob, "networkpolicy") || strings.Contains(blob, "network policy") {
		return "network-policy"
	}

	// 3) Service exposure check
	if strings.Contains(blob, "targetport") || strings.Contains(blob, "containerport") || strings.Contains(blob, "nodeport") || strings.Contains(blob, "loadbalancer") || strings.Contains(blob, "exposed") {
		return "service-port"
	}

	// 4) Secret misconfiguration
	if strings.Contains(desc, "plain text environment variable") || strings.Contains(desc, "secretkeyref") || strings.Contains(desc, "sensitive key") || strings.Contains(desc, "non-base64") || kindLower == "secret" || kindLower == "configmap" {
		return "secrets"
	}

	// 5) Container security checks (now includes the missing securityContext check)
	if strings.Contains(desc, "securitycontext") || strings.Contains(desc, "privileged mode") || strings.Contains(desc, "running as root") || strings.Contains(desc, "allow privilege escalation") || strings.Contains(desc, "allows privilege escalation") || strings.Contains(blob, "runasuser") {
		return "container-security"
	}

	// 6) Resource misconfiguration check (fallback)
	if strings.Contains(desc, "resource configuration") || strings.Contains(desc, "resource requests") || strings.Contains(desc, "resource limits") || strings.Contains(desc, "requests.cpu") || strings.Contains(desc, "limits.cpu") || strings.Contains(desc, "requests.memory") || strings.Contains(desc, "limits.memory") {
		return "resource-limit"
	}

	return "resource-limit"
}

func getCheckName(issueType string) string {
	switch issueType {
	case "rbac":
		return "RBAC Check"
	case "network-policy":
		return "Network Policy Check"
	case "service-port":
		return "Service Exposure Check"
	case "container-security":
		return "Container Security Check"
	case "secrets":
		return "Secret Misconfiguration Check"
	case "resource-limit":
		return "Resource Misconfiguration Check"
	default:
		return "Security Check"
	}
}
