package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// K8sResource represents a parsed Kubernetes resource
type K8sResource struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   map[string]interface{} `yaml:"metadata"`
	Spec       map[string]interface{} `yaml:"spec,omitempty"`
	Data       map[string]interface{} `yaml:"data,omitempty"`
}

// ParsedResource contains extracted K8s resource information
type ParsedResource struct {
	Kind        string
	Name        string
	Namespace   string
	ClusterID   string
	YAMLContent string
	IsK8sFile   bool
}

// IsKubernetesFile checks if a file path suggests it's a K8s manifest
func IsKubernetesFile(filePath string) bool {
	lowerPath := strings.ToLower(filePath)

	// Skip only helper files
	if strings.Contains(lowerPath, "_helpers.tpl") {
		return false
	}

	// Check for common K8s file patterns
	k8sPatterns := []string{
		".yaml", ".yml",
		"deployment", "service", "configmap", "secret",
		"ingress", "statefulset", "daemonset", "job",
		"cronjob", "pod", "namespace", "pvc", "pv",
		"k8s", "kubernetes", "helm", "chart",
	}

	for _, pattern := range k8sPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// ParseYAMLContent parses YAML content and extracts K8s resources
func ParseYAMLContent(content string) ([]ParsedResource, error) {
	var resources []ParsedResource

	// Check if it's a Helm template and render it first
	if IsHelmTemplate(content) {
		content = RenderHelmTemplate(content)
	}

	// Split by YAML document separator (---)
	documents := strings.Split(content, "\n---")

	for _, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Try to parse as K8s resource
		var resource K8sResource
		err := yaml.Unmarshal([]byte(doc), &resource)
		if err != nil {
			continue // Skip invalid YAML
		}

		// Check if it's a valid K8s resource (has kind and apiVersion)
		if resource.Kind == "" || resource.APIVersion == "" {
			continue
		}

		// Extract metadata
		name := extractString(resource.Metadata, "name")
		namespace := extractString(resource.Metadata, "namespace")

		// If name is empty, use a default
		if name == "" {
			name = "resource"
		}

		// Default namespace for namespaced resources
		if namespace == "" && isNamespacedResource(resource.Kind) {
			namespace = "default"
		}

		// Extract cluster ID if present in labels/annotations
		clusterID := extractClusterID(resource.Metadata)

		resources = append(resources, ParsedResource{
			Kind:        resource.Kind,
			Name:        name,
			Namespace:   namespace,
			ClusterID:   clusterID,
			YAMLContent: doc,
			IsK8sFile:   true,
		})
	}

	return resources, nil
}

// extractFromHelmTemplate extracts K8s resource info from Helm template
func extractFromHelmTemplate(content string) ParsedResource {
	lines := strings.Split(content, "\n")
	var kind, name, namespace string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "kind:") {
			kind = strings.TrimSpace(strings.TrimPrefix(line, "kind:"))
		} else if strings.Contains(line, "name:") && name == "" {
			// Extract name even if it has template syntax
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				nameValue := strings.TrimSpace(parts[1])
				// Remove template syntax for a cleaner name
				nameValue = strings.ReplaceAll(nameValue, "{{", "")
				nameValue = strings.ReplaceAll(nameValue, "}}", "")
				nameValue = strings.TrimSpace(nameValue)
				// Use the template variable name or a default
				if strings.Contains(nameValue, ".Values.") {
					// Extract the variable name
					parts := strings.Split(nameValue, ".")
					if len(parts) > 0 {
						name = parts[len(parts)-1]
					}
				} else if nameValue != "" {
					name = nameValue
				}
			}
		} else if strings.Contains(line, "namespace:") && namespace == "" {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				nsValue := strings.TrimSpace(parts[1])
				nsValue = strings.ReplaceAll(nsValue, "{{", "")
				nsValue = strings.ReplaceAll(nsValue, "}}", "")
				nsValue = strings.TrimSpace(nsValue)
				if nsValue != "" && !strings.Contains(nsValue, ".Values") {
					namespace = nsValue
				}
			}
		}
	}

	// Set defaults
	if name == "" {
		name = "helm-resource"
	}
	if namespace == "" && isNamespacedResource(kind) {
		namespace = "default"
	}

	return ParsedResource{
		Kind:        kind,
		Name:        name,
		Namespace:   namespace,
		ClusterID:   "",
		YAMLContent: content,
		IsK8sFile:   true,
	}
}

// extractNameFromTemplate tries to extract name from template syntax
func extractNameFromTemplate(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, "name:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				// Clean up template syntax
				name = strings.ReplaceAll(name, "{{", "")
				name = strings.ReplaceAll(name, "}}", "")
				name = strings.TrimSpace(name)
				if name != "" {
					return name
				}
			}
		}
	}
	return "resource"
}

// extractString safely extracts a string value from a map
func extractString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}

	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}

	return ""
}

// extractClusterID tries to find cluster ID in labels or annotations
func extractClusterID(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}

	// Check labels
	if labels, ok := metadata["labels"].(map[string]interface{}); ok {
		if clusterID := extractString(labels, "cluster"); clusterID != "" {
			return clusterID
		}
		if clusterID := extractString(labels, "cluster-id"); clusterID != "" {
			return clusterID
		}
	}

	// Check annotations
	if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
		if clusterID := extractString(annotations, "cluster"); clusterID != "" {
			return clusterID
		}
		if clusterID := extractString(annotations, "cluster-id"); clusterID != "" {
			return clusterID
		}
	}

	return ""
}

// isNamespacedResource checks if a K8s resource type is namespaced
func isNamespacedResource(kind string) bool {
	namespacedKinds := map[string]bool{
		"Pod":                   true,
		"Service":               true,
		"Deployment":            true,
		"StatefulSet":           true,
		"DaemonSet":             true,
		"ReplicaSet":            true,
		"Job":                   true,
		"CronJob":               true,
		"ConfigMap":             true,
		"Secret":                true,
		"Ingress":               true,
		"PersistentVolumeClaim": true,
		"ServiceAccount":        true,
		"Role":                  true,
		"RoleBinding":           true,
		"NetworkPolicy":         true,
		"ResourceQuota":         true,
		"LimitRange":            true,
	}

	return namespacedKinds[kind]
}

// ValidateK8sResource performs basic validation on a K8s resource
func ValidateK8sResource(resource ParsedResource) error {
	if resource.Kind == "" {
		return fmt.Errorf("resource kind is required")
	}

	if resource.Name == "" {
		return fmt.Errorf("resource name is required")
	}

	if resource.YAMLContent == "" {
		return fmt.Errorf("YAML content is required")
	}

	return nil
}

// YAMLToJSON converts YAML content to JSON string for JSONB storage
func YAMLToJSON(yamlContent string) (string, error) {
	var data interface{}

	// Parse YAML
	err := yaml.Unmarshal([]byte(yamlContent), &data)
	if err != nil {
		return "", fmt.Errorf("failed to parse YAML: %v", err)
	}

	// Clean up null values recursively
	data = removeNullValues(data)

	// Convert to JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to convert to JSON: %v", err)
	}

	return string(jsonBytes), nil
}

// removeNullValues recursively removes null/nil values and empty objects from maps and slices
func removeNullValues(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			if val == nil {
				continue // Skip null values
			}
			cleaned := removeNullValues(val)
			// Only add non-nil, non-empty values
			if cleaned != nil {
				switch c := cleaned.(type) {
				case map[string]interface{}:
					if len(c) > 0 {
						result[key] = cleaned
					}
				case []interface{}:
					if len(c) > 0 {
						result[key] = cleaned
					}
				default:
					result[key] = cleaned
				}
			}
		}
		// Return nil if map is empty
		if len(result) == 0 {
			return nil
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0)
		for _, val := range v {
			if val == nil {
				continue // Skip null values
			}
			cleaned := removeNullValues(val)
			// Skip empty maps and nil values
			if cleaned != nil {
				if m, ok := cleaned.(map[string]interface{}); ok {
					if len(m) > 0 {
						result = append(result, cleaned)
					}
				} else {
					result = append(result, cleaned)
				}
			}
		}
		return result
	default:
		return v
	}
}
