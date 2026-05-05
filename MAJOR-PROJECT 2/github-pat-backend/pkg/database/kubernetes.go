package database

import (
	"time"
)

// KubernetesResource represents a Kubernetes resource
type KubernetesResource struct {
	ID          int
	ResourceID  string
	OrgID       string
	RepoFileID  int
	ClusterID   string
	Kind        string
	Name        string
	Namespace   string
	YAMLContent string
	CreatedAt   time.Time
}

// CreateK8sResource creates a new Kubernetes resource record
func CreateK8sResource(resourceID, orgID string, repoFileID int, clusterID, kind, name, namespace, yamlContent string) error {
	_, err := DB.Exec(
		`INSERT INTO kubernetes_resource
		 (resource_id, org_id, repo_file_id, cluster_id, kind, name, namespace, yaml_content, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())`,
		resourceID, orgID, repoFileID, clusterID, kind, name, namespace, yamlContent,
	)
	return err
}

// GetK8sResourceByID retrieves a Kubernetes resource by resource_id
func GetK8sResourceByID(resourceID string) (*KubernetesResource, error) {
	resource := &KubernetesResource{}
	err := DB.QueryRow(
		`SELECT id, resource_id, org_id, repo_file_id, cluster_id, kind, name, namespace, yaml_content, created_at
		 FROM kubernetes_resource
		 WHERE resource_id = $1`,
		resourceID,
	).Scan(&resource.ID, &resource.ResourceID, &resource.OrgID, &resource.RepoFileID,
		&resource.ClusterID, &resource.Kind, &resource.Name, &resource.Namespace,
		&resource.YAMLContent, &resource.CreatedAt)

	if err != nil {
		return nil, err
	}

	return resource, nil
}

// GetK8sResourcesByOrgID retrieves all Kubernetes resources for an organization
func GetK8sResourcesByOrgID(orgID string) ([]KubernetesResource, error) {
	rows, err := DB.Query(
		`SELECT id, resource_id, org_id, repo_file_id, cluster_id, kind, name, namespace, yaml_content, created_at
		 FROM kubernetes_resource
		 WHERE org_id = $1
		 ORDER BY kind, name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resources []KubernetesResource
	for rows.Next() {
		var resource KubernetesResource
		if err := rows.Scan(&resource.ID, &resource.ResourceID, &resource.OrgID, &resource.RepoFileID,
			&resource.ClusterID, &resource.Kind, &resource.Name, &resource.Namespace,
			&resource.YAMLContent, &resource.CreatedAt); err != nil {
			continue
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

// CheckK8sResourceExists checks if a resource already exists
func CheckK8sResourceExists(orgID string, repoFileID int, kind, name string) (bool, int, error) {
	var exists bool
	var resourceID int
	err := DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM kubernetes_resource WHERE org_id = $1 AND repo_file_id = $2 AND kind = $3 AND name = $4),
		 COALESCE((SELECT id FROM kubernetes_resource WHERE org_id = $1 AND repo_file_id = $2 AND kind = $3 AND name = $4 LIMIT 1), 0)`,
		orgID, repoFileID, kind, name,
	).Scan(&exists, &resourceID)

	return exists, resourceID, err
}

// UpdateK8sResource updates an existing Kubernetes resource
func UpdateK8sResource(id int, clusterID, namespace, yamlContent string) error {
	_, err := DB.Exec(
		`UPDATE kubernetes_resource
		 SET cluster_id = $1, namespace = $2, yaml_content = $3
		 WHERE id = $4`,
		clusterID, namespace, yamlContent, id,
	)
	return err
}

// CheckResourceIDExists checks if resource_id already exists
func CheckResourceIDExists(resourceID string) (bool, error) {
	var exists bool
	err := DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM kubernetes_resource WHERE resource_id = $1)`,
		resourceID,
	).Scan(&exists)

	return exists, err
}

// CountK8sResourcesByOrgID counts Kubernetes resources for an organization
func CountK8sResourcesByOrgID(orgID string) (int, error) {
	var count int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM kubernetes_resource WHERE org_id = $1`,
		orgID,
	).Scan(&count)

	return count, err
}

// DeleteK8sResourcesByOrgID deletes all Kubernetes resources for an organization
func DeleteK8sResourcesByOrgID(orgID string) error {
	_, err := DB.Exec(
		`DELETE FROM kubernetes_resource WHERE org_id = $1`,
		orgID,
	)
	return err
}
