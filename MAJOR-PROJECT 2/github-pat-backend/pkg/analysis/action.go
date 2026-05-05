package analysis

import "strings"

// GenerateActions generates recommended actions based on resource analysis
func GenerateActions(kind, name, yamlContent string) []map[string]interface{} {
	var actions []map[string]interface{}

	if kind == "Pod" || kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" {
		// Check for privileged containers
		if strings.Contains(yamlContent, "privileged: true") {
			actions = append(actions, map[string]interface{}{
				"type":        "remove_privilege",
				"priority":    "critical",
				"description": "Remove privileged mode from container",
				"remediation": "Set privileged: false in securityContext",
			})
		}

		// Check for root user
		if !strings.Contains(yamlContent, "runAsNonRoot: true") {
			actions = append(actions, map[string]interface{}{
				"type":        "enforce_non_root",
				"priority":    "high",
				"description": "Enforce non-root user for container",
				"remediation": "Add runAsNonRoot: true to securityContext",
			})
		}

		// Check for resource limits
		if !strings.Contains(yamlContent, "limits:") {
			actions = append(actions, map[string]interface{}{
				"type":        "add_resource_limits",
				"priority":    "medium",
				"description": "Add resource limits to prevent resource exhaustion",
				"remediation": "Define CPU and memory limits in resources section",
			})
		}

		// Check for readiness probe
		if !strings.Contains(yamlContent, "readinessProbe:") {
			actions = append(actions, map[string]interface{}{
				"type":        "add_readiness_probe",
				"priority":    "medium",
				"description": "Add readiness probe for better health checks",
				"remediation": "Define readinessProbe in container spec",
			})
		}

		// Check for liveness probe
		if !strings.Contains(yamlContent, "livenessProbe:") {
			actions = append(actions, map[string]interface{}{
				"type":        "add_liveness_probe",
				"priority":    "medium",
				"description": "Add liveness probe for automatic recovery",
				"remediation": "Define livenessProbe in container spec",
			})
		}

		// Check for image pull policy
		if !strings.Contains(yamlContent, "imagePullPolicy:") {
			actions = append(actions, map[string]interface{}{
				"type":        "set_image_pull_policy",
				"priority":    "low",
				"description": "Set explicit image pull policy",
				"remediation": "Add imagePullPolicy: Always or IfNotPresent",
			})
		}
	}

	if kind == "Service" {
		// Check for LoadBalancer
		if strings.Contains(yamlContent, "type: LoadBalancer") {
			if !strings.Contains(yamlContent, "loadBalancerSourceRanges:") {
				actions = append(actions, map[string]interface{}{
					"type":        "restrict_loadbalancer",
					"priority":    "high",
					"description": "Restrict LoadBalancer access with source IP ranges",
					"remediation": "Add loadBalancerSourceRanges to limit access",
				})
			}
		}
	}

	if kind == "Ingress" {
		// Check for TLS
		if !strings.Contains(yamlContent, "tls:") {
			actions = append(actions, map[string]interface{}{
				"type":        "enable_tls",
				"priority":    "high",
				"description": "Enable TLS for secure communication",
				"remediation": "Add TLS configuration to Ingress",
			})
		}
	}

	return actions
}
