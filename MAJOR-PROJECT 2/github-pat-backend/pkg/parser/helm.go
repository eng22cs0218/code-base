package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// IsHelmTemplate checks if content contains Helm templating syntax
func IsHelmTemplate(content string) bool {
	// Check for common Helm template patterns
	helmPatterns := []string{
		`\{\{.*\}\}`,       // {{ anything }}
		`\{\{-.*-\}\}`,     // {{- anything -}}
		`\.Values\.`,       // .Values.
		`\.Release\.`,      // .Release.
		`\.Chart\.`,        // .Chart.
		`\.Capabilities\.`, // .Capabilities.
		`\.Template\.`,     // .Template.
		`include\s+`,       // include function
		`toYaml\s+`,        // toYaml function
		`\|\s*indent`,      // | indent
		`\|\s*nindent`,     // | nindent
		`\|\s*quote`,       // | quote
		`\|\s*default`,     // | default
	}

	for _, pattern := range helmPatterns {
		matched, _ := regexp.MatchString(pattern, content)
		if matched {
			return true
		}
	}

	return false
}

// RenderHelmTemplate attempts to render a Helm template with default values
func RenderHelmTemplate(content string) string {
	// Add common defaults
	values := map[string]interface{}{
		"name":            "app-name",
		"fullname":        "app-fullname",
		"app":             "app-name",
		"replicas":        "3",
		"replicaCount":    "3",
		"image":           "nginx:latest",
		"repository":      "nginx",
		"tag":             "latest",
		"pullPolicy":      "IfNotPresent",
		"imagePullPolicy": "IfNotPresent",
		"port":            "80",
		"containerPort":   "8080",
		"targetPort":      "8080",
		"servicePort":     "80",
		"serviceType":     "ClusterIP",
		"type":            "ClusterIP",
		"tier":            "backend",
		"version":         "v1",
	}

	// Apply basic template rendering
	rendered := renderBasicTemplates(content, values)
	return rendered
}

// renderBasicTemplates replaces common Helm template patterns
func renderBasicTemplates(content string, values map[string]interface{}) string {
	rendered := content

	// Handle if/else/end blocks - remove them but keep content
	ifRe := regexp.MustCompile(`\{\{-?\s*if\s+[^}]+\s*-?\}\}`)
	rendered = ifRe.ReplaceAllString(rendered, "")
	elseRe := regexp.MustCompile(`\{\{-?\s*else\s*-?\}\}`)
	rendered = elseRe.ReplaceAllString(rendered, "")
	endRe := regexp.MustCompile(`\{\{-?\s*end\s*-?\}\}`)
	rendered = endRe.ReplaceAllString(rendered, "")

	// Handle range loops - remove them but keep content
	rangeRe := regexp.MustCompile(`\{\{-?\s*range\s+[^}]+\s*-?\}\}`)
	rendered = rangeRe.ReplaceAllString(rendered, "")

	// Handle with blocks
	withRe := regexp.MustCompile(`\{\{-?\s*with\s+[^}]+\s*-?\}\}`)
	rendered = withRe.ReplaceAllString(rendered, "")

	// Replace .Release.Name and .Release.Namespace (all variations)
	rendered = strings.ReplaceAll(rendered, "{{ .Release.Name }}", "release-name")
	rendered = strings.ReplaceAll(rendered, "{{.Release.Name}}", "release-name")
	rendered = strings.ReplaceAll(rendered, "{{- .Release.Name }}", "release-name")
	rendered = strings.ReplaceAll(rendered, "{{- .Release.Name -}}", "release-name")
	rendered = strings.ReplaceAll(rendered, "{{ .Release.Namespace }}", "default")
	rendered = strings.ReplaceAll(rendered, "{{.Release.Namespace}}", "default")
	rendered = strings.ReplaceAll(rendered, "{{- .Release.Namespace }}", "default")
	rendered = strings.ReplaceAll(rendered, "{{- .Release.Namespace -}}", "default")

	// Replace .Chart.Name and .Chart.Version
	rendered = strings.ReplaceAll(rendered, "{{ .Chart.Name }}", "chart-name")
	rendered = strings.ReplaceAll(rendered, "{{.Chart.Name}}", "chart-name")
	rendered = strings.ReplaceAll(rendered, "{{- .Chart.Name }}", "chart-name")
	rendered = strings.ReplaceAll(rendered, "{{- .Chart.Name -}}", "chart-name")
	rendered = strings.ReplaceAll(rendered, "{{ .Chart.Version }}", "1.0.0")
	rendered = strings.ReplaceAll(rendered, "{{.Chart.Version}}", "1.0.0")
	rendered = strings.ReplaceAll(rendered, "{{- .Chart.Version }}", "1.0.0")
	rendered = strings.ReplaceAll(rendered, "{{- .Chart.Version -}}", "1.0.0")

	// Handle include statements - replace with placeholder
	includeRe := regexp.MustCompile(`\{\{-?\s*include\s+"[^"]+"\s+\.\s*-?\}\}`)
	rendered = includeRe.ReplaceAllString(rendered, "app-name")

	// Handle toYaml with nindent - remove completely
	toYamlRe := regexp.MustCompile(`\{\{-?\s*\.Values\.\w+(\.\w+)*\s*\|\s*toYaml\s*\|\s*nindent\s+\d+\s*-?\}\}`)
	rendered = toYamlRe.ReplaceAllString(rendered, "")

	// Handle .Values.xxx.yyy.zzz (3-level nested)
	deepNestedRe := regexp.MustCompile(`\{\{-?\s*\.Values\.(\w+)\.(\w+)\.(\w+)\s*-?\}\}`)
	rendered = deepNestedRe.ReplaceAllStringFunc(rendered, func(match string) string {
		matches := deepNestedRe.FindStringSubmatch(match)
		if len(matches) >= 4 {
			// Use the deepest key
			return getDefaultValue(matches[3])
		}
		return "default-value"
	})

	// Handle .Values.xxx.yyy (2-level nested)
	nestedRe := regexp.MustCompile(`\{\{-?\s*\.Values\.(\w+)\.(\w+)\s*-?\}\}`)
	rendered = nestedRe.ReplaceAllStringFunc(rendered, func(match string) string {
		matches := nestedRe.FindStringSubmatch(match)
		if len(matches) >= 3 {
			parent := matches[1]
			child := matches[2]

			// Special handling for image fields
			if parent == "image" {
				if child == "repository" {
					return "nginx"
				}
				if child == "tag" {
					return "latest"
				}
				if child == "pullPolicy" {
					return "IfNotPresent"
				}
			}
			return getDefaultValue(child)
		}
		return "default-value"
	})

	// Handle .Values.xxx (simple)
	simpleRe := regexp.MustCompile(`\{\{-?\s*\.Values\.(\w+)\s*-?\}\}`)
	rendered = simpleRe.ReplaceAllStringFunc(rendered, func(match string) string {
		matches := simpleRe.FindStringSubmatch(match)
		if len(matches) >= 2 {
			key := matches[1]
			if val, ok := values[key]; ok {
				return fmt.Sprintf("%v", val)
			}
			return getDefaultValue(key)
		}
		return "default-value"
	})

	// Handle quote filter
	quoteRe := regexp.MustCompile(`\{\{-?\s*\.Values\.(\w+)\s*\|\s*quote\s*-?\}\}`)
	rendered = quoteRe.ReplaceAllStringFunc(rendered, func(match string) string {
		matches := quoteRe.FindStringSubmatch(match)
		if len(matches) >= 2 {
			key := matches[1]
			if val, ok := values[key]; ok {
				return fmt.Sprintf("\"%v\"", val)
			}
			return fmt.Sprintf("\"%s\"", getDefaultValue(key))
		}
		return "\"default-value\""
	})

	// Handle default filter with various quote styles
	defaultRe := regexp.MustCompile(`\{\{-?\s*\.Values\.(\w+)\s*\|\s*default\s+["']([^"']+)["']\s*-?\}\}`)
	rendered = defaultRe.ReplaceAllStringFunc(rendered, func(match string) string {
		matches := defaultRe.FindStringSubmatch(match)
		if len(matches) >= 3 {
			key := matches[1]
			defaultVal := matches[2]

			if val, ok := values[key]; ok {
				return fmt.Sprintf("%v", val)
			}
			return defaultVal
		}
		return "default-value"
	})

	// Handle ternary operator: {{ if .Values.x }}a{{ else }}b{{ end }}
	// Already handled by removing if/else/end blocks above

	// Clean up any remaining template syntax
	remainingRe := regexp.MustCompile(`\{\{-?[^}]*-?\}\}`)
	rendered = remainingRe.ReplaceAllString(rendered, "")

	// Fix empty image fields
	imageRe := regexp.MustCompile(`image:\s*"?:?"?\s*$`)
	rendered = imageRe.ReplaceAllString(rendered, "image: nginx:latest")

	// Fix lines that became empty after template removal
	lines := strings.Split(rendered, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Keep the line if it's not empty or if it's just whitespace (for indentation)
		if trimmed != "" || (len(line) > 0 && line[0] == ' ') {
			cleanedLines = append(cleanedLines, line)
		}
	}
	rendered = strings.Join(cleanedLines, "\n")

	return rendered
}

// getDefaultValue returns sensible defaults for common Helm value keys
func getDefaultValue(key string) string {
	defaults := map[string]string{
		// Basic app info
		"name":      "app-name",
		"fullname":  "app-fullname",
		"namespace": "default",
		"app":       "app-name",
		"chart":     "chart-name",

		// Deployment settings
		"replicas":             "3",
		"replicaCount":         "3",
		"revisionHistoryLimit": "10",

		// Image settings
		"image":           "nginx:latest",
		"repository":      "nginx",
		"tag":             "latest",
		"pullPolicy":      "IfNotPresent",
		"imagePullPolicy": "IfNotPresent",
		"pullSecrets":     "",

		// Port settings
		"port":          "80",
		"targetPort":    "8080",
		"containerPort": "8080",
		"protocol":      "TCP",
		"servicePort":   "80",

		// Service settings
		"serviceType":  "ClusterIP",
		"type":         "ClusterIP",
		"clusterIP":    "",
		"externalPort": "80",
		"internalPort": "8080",

		// Resource limits
		"cpu":           "100m",
		"memory":        "128Mi",
		"cpuLimit":      "200m",
		"memoryLimit":   "256Mi",
		"cpuRequest":    "100m",
		"memoryRequest": "128Mi",

		// Storage
		"storageClass": "standard",
		"size":         "1Gi",
		"accessMode":   "ReadWriteOnce",

		// Ingress
		"path":           "/",
		"host":           "example.com",
		"ingressEnabled": "false",
		"tls":            "false",

		// Health checks
		"livenessPath":        "/healthz",
		"readinessPath":       "/ready",
		"healthPath":          "/health",
		"initialDelaySeconds": "30",
		"periodSeconds":       "10",
		"timeoutSeconds":      "5",
		"successThreshold":    "1",
		"failureThreshold":    "3",

		// Strategy
		"maxSurge":       "1",
		"maxUnavailable": "0",
		"strategyType":   "RollingUpdate",

		// Labels and selectors
		"tier":        "backend",
		"version":     "v1",
		"component":   "api",
		"environment": "production",
		"release":     "release-name",

		// Security
		"runAsUser":                "1000",
		"runAsNonRoot":             "true",
		"readOnlyRootFilesystem":   "false",
		"allowPrivilegeEscalation": "false",

		// Boolean flags
		"enabled":     "true",
		"autoscaling": "false",
		"monitoring":  "false",

		// Autoscaling
		"minReplicas":                    "1",
		"maxReplicas":                    "10",
		"targetCPUUtilizationPercentage": "80",

		// Service Account
		"serviceAccount":       "default",
		"serviceAccountName":   "default",
		"createServiceAccount": "false",

		// Config and Secrets
		"configMap": "",
		"secret":    "",

		// Empty defaults for complex objects
		"annotations":       "",
		"labels":            "",
		"nodeSelector":      "",
		"tolerations":       "",
		"affinity":          "",
		"resources":         "",
		"env":               "",
		"envFrom":           "",
		"volumeMounts":      "",
		"volumes":           "",
		"extraEnv":          "",
		"extraVolumes":      "",
		"extraVolumeMounts": "",
	}

	if val, ok := defaults[key]; ok {
		return val
	}

	// Return a generic default
	return "default-value"
}
