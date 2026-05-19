// Copyright Contributors to the Open Cluster Management project

package v1

import (
	"context"
	"fmt"

	ocv1 "github.com/operator-framework/operator-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// OLM v1 ClusterCatalog names (cluster-scoped, no namespace)
	// These are the default Red Hat ClusterCatalogs
	ClusterCatalogName          = "openshift-redhat-operators"
	CommunityClusterCatalogName = "openshift-community-operators"
)

// GetClusterCatalog returns the name of a ClusterCatalog containing the desired package.
// Unlike v0 CatalogSource (namespaced), ClusterCatalog is cluster-scoped so returns only name.
// Selects catalog based on priority (highest priority wins).
// Returns error if multiple catalogs with same highest priority exist.
func GetClusterCatalog(ctx context.Context, k8sClient client.Client, desiredPackage string) (string, error) {
	log := log.Log.WithName("reconcile")

	// List all ClusterCatalogs
	ccList := &ocv1.ClusterCatalogList{}
	if err := k8sClient.List(ctx, ccList); err != nil {
		return "", fmt.Errorf("failed to list ClusterCatalogs: %w", err)
	}

	if len(ccList.Items) == 0 {
		return "", fmt.Errorf("no ClusterCatalogs found")
	}

	// Filter to available catalogs only
	var availableCatalogs []ocv1.ClusterCatalog
	for _, cc := range ccList.Items {
		// Skip unavailable catalogs
		if cc.Spec.AvailabilityMode == "Unavailable" {
			continue
		}
		availableCatalogs = append(availableCatalogs, cc)
	}

	if len(availableCatalogs) == 0 {
		return "", fmt.Errorf("no available ClusterCatalogs found")
	}

	// Find catalog with highest priority
	// TODO: In the future, query catalog content to verify package exists
	highestPriority := availableCatalogs[0].Spec.Priority
	var highestPriorityCatalogs []ocv1.ClusterCatalog

	for _, cc := range availableCatalogs {
		if cc.Spec.Priority > highestPriority {
			highestPriority = cc.Spec.Priority
			highestPriorityCatalogs = []ocv1.ClusterCatalog{cc}
		} else if cc.Spec.Priority == highestPriority {
			highestPriorityCatalogs = append(highestPriorityCatalogs, cc)
		}
	}

	if len(highestPriorityCatalogs) == 0 {
		return "", fmt.Errorf("no suitable ClusterCatalog found for package %s", desiredPackage)
	}

	if len(highestPriorityCatalogs) > 1 {
		var catalogNames []string
		for _, cc := range highestPriorityCatalogs {
			catalogNames = append(catalogNames, cc.Name)
		}
		return "", fmt.Errorf("found more than one ClusterCatalog with highest priority (%d): %v", highestPriority, catalogNames)
	}

	catalog := highestPriorityCatalogs[0]
	log.Info(fmt.Sprintf("Using ClusterCatalog %s (priority: %d) for package %s", catalog.Name, catalog.Spec.Priority, desiredPackage))
	return catalog.Name, nil
}
