package analysis

import "strings"

// CalculateResourceScore calculates security score for a resource
func CalculateResourceScore(kind, yamlContent string) (int, int, map[string]interface{}) {
	score := 0
	maxScore := 100
	breakdown := make(map[string]interface{})

	if kind == "Pod" || kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" {
		// Security Context (20 points)
		if strings.Contains(yamlContent, "securityContext:") {
			score += 20
			breakdown["security_context"] = "✓ Configured"
		} else {
			breakdown["security_context"] = "✗ Missing"
		}

		// Non-root user (20 points)
		if strings.Contains(yamlContent, "runAsNonRoot: true") {
			score += 20
			breakdown["non_root_user"] = "✓ Enforced"
		} else {
			breakdown["non_root_user"] = "✗ Not enforced"
		}

		// No privileged mode (20 points)
		if !strings.Contains(yamlContent, "privileged: true") {
			score += 20
			breakdown["privileged_mode"] = "✓ Disabled"
		} else {
			breakdown["privileged_mode"] = "✗ Enabled (critical)"
		}

		// Resource limits (15 points)
		if strings.Contains(yamlContent, "limits:") {
			score += 15
			breakdown["resource_limits"] = "✓ Defined"
		} else {
			breakdown["resource_limits"] = "✗ Missing"
		}

		// Readiness probe (10 points)
		if strings.Contains(yamlContent, "readinessProbe:") {
			score += 10
			breakdown["readiness_probe"] = "✓ Configured"
		} else {
			breakdown["readiness_probe"] = "✗ Missing"
		}

		// Liveness probe (10 points)
		if strings.Contains(yamlContent, "livenessProbe:") {
			score += 10
			breakdown["liveness_probe"] = "✓ Configured"
		} else {
			breakdown["liveness_probe"] = "✗ Missing"
		}

		// No host network (5 points)
		if !strings.Contains(yamlContent, "hostNetwork: true") {
			score += 5
			breakdown["host_network"] = "✓ Not used"
		} else {
			breakdown["host_network"] = "✗ Used (risky)"
		}
	} else if kind == "Service" {
		// Service type (40 points)
		if strings.Contains(yamlContent, "type: ClusterIP") {
			score += 40
			breakdown["service_type"] = "✓ ClusterIP (secure)"
		} else if strings.Contains(yamlContent, "type: NodePort") {
			score += 20
			breakdown["service_type"] = "⚠ NodePort (moderate risk)"
		} else if strings.Contains(yamlContent, "type: LoadBalancer") {
			if strings.Contains(yamlContent, "loadBalancerSourceRanges:") {
				score += 30
				breakdown["service_type"] = "⚠ LoadBalancer with IP restrictions"
			} else {
				score += 10
				breakdown["service_type"] = "✗ LoadBalancer without restrictions"
			}
		} else {
			score += 40
			breakdown["service_type"] = "✓ Default type"
		}

		// Session affinity (30 points)
		if strings.Contains(yamlContent, "sessionAffinity:") {
			score += 30
			breakdown["session_affinity"] = "✓ Configured"
		} else {
			score += 15
			breakdown["session_affinity"] = "⚠ Not configured"
		}

		// Selector defined (30 points)
		if strings.Contains(yamlContent, "selector:") {
			score += 30
			breakdown["selector"] = "✓ Defined"
		} else {
			breakdown["selector"] = "✗ Missing"
		}
	} else if kind == "Ingress" {
		// TLS enabled (50 points)
		if strings.Contains(yamlContent, "tls:") {
			score += 50
			breakdown["tls"] = "✓ Enabled"
		} else {
			breakdown["tls"] = "✗ Disabled"
		}

		// Annotations (25 points)
		if strings.Contains(yamlContent, "annotations:") {
			score += 25
			breakdown["annotations"] = "✓ Configured"
		} else {
			breakdown["annotations"] = "⚠ Missing"
		}

		// Rules defined (25 points)
		if strings.Contains(yamlContent, "rules:") {
			score += 25
			breakdown["rules"] = "✓ Defined"
		} else {
			breakdown["rules"] = "✗ Missing"
		}
	} else if kind == "NetworkPolicy" {
		// Network policy exists (bonus)
		score = 100
		breakdown["network_policy"] = "✓ Configured (excellent)"
	} else {
		// Default scoring for other resources
		score = 70
		breakdown["default"] = "⚠ Basic resource (default score)"
	}

	return score, maxScore, breakdown
}

// CalculatePercentage calculates percentage score
func CalculatePercentage(score, maxScore int) float64 {
	if maxScore == 0 {
		return 0
	}
	return float64(score) / float64(maxScore) * 100
}

// CalculateGrade assigns a letter grade based on score
func CalculateGrade(score, maxScore int) string {
	percentage := CalculatePercentage(score, maxScore)

	if percentage >= 90 {
		return "A"
	} else if percentage >= 80 {
		return "B"
	} else if percentage >= 70 {
		return "C"
	} else if percentage >= 60 {
		return "D"
	}
	return "F"
}

// GetComplianceStatus returns compliance status based on score
func GetComplianceStatus(percentage float64) string {
	if percentage >= 90 {
		return "Excellent - Highly compliant"
	} else if percentage >= 80 {
		return "Good - Mostly compliant"
	} else if percentage >= 70 {
		return "Fair - Needs improvement"
	} else if percentage >= 60 {
		return "Poor - Significant issues"
	}
	return "Critical - Immediate action required"
}

// GetRecommendations provides recommendations based on score
func GetRecommendations(percentage float64) []string {
	var recommendations []string

	if percentage < 90 {
		recommendations = append(recommendations, "Review and implement security contexts for all workloads")
	}
	if percentage < 80 {
		recommendations = append(recommendations, "Add resource limits to prevent resource exhaustion")
	}
	if percentage < 70 {
		recommendations = append(recommendations, "Implement health checks (readiness and liveness probes)")
	}
	if percentage < 60 {
		recommendations = append(recommendations, "Remove privileged containers and enforce non-root users")
		recommendations = append(recommendations, "Restrict LoadBalancer services with IP whitelisting")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Maintain current security posture and monitor for changes")
	}

	return recommendations
}
