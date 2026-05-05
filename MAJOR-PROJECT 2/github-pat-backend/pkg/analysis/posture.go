package analysis

import "strings"

// AnalyzeResourcePosture performs security posture analysis
func AnalyzeResourcePosture(kind, yamlContent string) []string {
	var issues []string

	// Check for common security issues
	if kind == "Pod" || kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" {
		// Check for privileged containers
		if strings.Contains(yamlContent, "privileged: true") {
			issues = append(issues, "Container running in privileged mode")
		}

		// Check for root user
		if !strings.Contains(yamlContent, "runAsNonRoot: true") {
			issues = append(issues, "Container may run as root user")
		}

		// Check for resource limits
		if !strings.Contains(yamlContent, "limits:") {
			issues = append(issues, "No resource limits defined")
		}

		// Check for security context
		if !strings.Contains(yamlContent, "securityContext:") {
			issues = append(issues, "No security context defined")
		}

		// Check for host network
		if strings.Contains(yamlContent, "hostNetwork: true") {
			issues = append(issues, "Using host network")
		}

		// Check for host PID
		if strings.Contains(yamlContent, "hostPID: true") {
			issues = append(issues, "Using host PID namespace")
		}

		// Check for capabilities
		if strings.Contains(yamlContent, "capabilities:") && strings.Contains(yamlContent, "add:") {
			issues = append(issues, "Additional capabilities granted")
		}
	}

	if kind == "Service" {
		// Check for LoadBalancer without restrictions
		if strings.Contains(yamlContent, "type: LoadBalancer") {
			if !strings.Contains(yamlContent, "loadBalancerSourceRanges:") {
				issues = append(issues, "LoadBalancer service exposed without IP restrictions")
			}
		}
	}

	if kind == "Secret" {
		// Check for hardcoded secrets
		if strings.Contains(yamlContent, "stringData:") {
			issues = append(issues, "Secret may contain plaintext data")
		}
	}

	return issues
}

// CalculateSeverity determines severity based on issues
func CalculateSeverity(issues []string) string {
	if len(issues) == 0 {
		return "none"
	}

	for _, issue := range issues {
		if strings.Contains(issue, "privileged") || strings.Contains(issue, "hostNetwork") || strings.Contains(issue, "hostPID") {
			return "critical"
		}
	}

	for _, issue := range issues {
		if strings.Contains(issue, "root") || strings.Contains(issue, "LoadBalancer") || strings.Contains(issue, "capabilities") {
			return "high"
		}
	}

	if len(issues) >= 3 {
		return "medium"
	}

	return "low"
}
