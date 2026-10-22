// Copyright Contributors to the Open Cluster Management project

// Package v0 manages MCE installation via OLM v0 APIs.
//
// This package handles creating CatalogSource and Subscription resources
// to install the MultiClusterEngine operator via OLM on OpenShift 4.x.
package v0

import (
	"context"
	"fmt"
	"math"

	"github.com/Masterminds/semver/v3"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmapi "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// OLM v0 catalog source names
	CatalogSourceName          = "redhat-operators"
	CatalogSourceNamespace     = "openshift-marketplace"
	CommunityCatalogSourceName = "community-operators"
)

// GetCatalogSource returns the name and namespace of an MCE catalogSource with the required channel.
// Returns error if two or more catalogsources satisfy criteria.
func GetCatalogSource(k8sClient client.Client, desiredChannel, desiredPackage string) (types.NamespacedName, error) {
	nn := types.NamespacedName{}

	pkgs, err := GetMCEPackageManifests(k8sClient, desiredPackage)
	if err != nil {
		return nn, err
	}

	// Return an error if there are no package manifests found with the desired MCE package name.
	if len(pkgs) == 0 {
		return nn, fmt.Errorf("no %s packageManifests found", desiredPackage)
	}

	filtered := filterPackageManifests(pkgs, desiredChannel)
	// Return an error if there are no package manifests found with the desired MCE channel name.
	if len(filtered) == 0 {
		return nn, fmt.Errorf("no %s packageManifests found with desired channel %s", desiredPackage, desiredChannel)
	}

	catalogSource, err := findHighestPriorityCatalogSource(k8sClient, filtered)
	if err != nil {
		return nn, err
	}

	nn.Name = catalogSource.Name
	nn.Namespace = catalogSource.Namespace
	return nn, nil
}

// extractCatalogSource extracts namespaced name from the given PackageManifest.
func extractCatalogSource(pm olmapi.PackageManifest) types.NamespacedName {
	return types.NamespacedName{
		Name:      pm.Status.CatalogSource,
		Namespace: pm.Status.CatalogSourceNamespace,
	}
}

// findHighestPriorityCatalogSource finds the catalog source with the highest priority among the given list.
func findHighestPriorityCatalogSource(k8sClient client.Client, pkgs []olmapi.PackageManifest) (*subv1alpha1.CatalogSource, error) {
	var (
		highestPriorityCatalogSources []*subv1alpha1.CatalogSource
		maxPriority                   = math.MinInt64
		log                           = log.Log.WithName("reconcile")
	)

	for _, pm := range pkgs {
		cs := &subv1alpha1.CatalogSource{}
		nn := extractCatalogSource(pm)

		if err := k8sClient.Get(context.TODO(), nn, cs); err != nil {
			// Log the error and continue to the next iteration
			log.Error(err, fmt.Sprintf("failed to retrieve catalog source %s/%s", nn.Namespace, nn.Name))
			continue
		}

		if cs.Spec.Priority > maxPriority {
			// Found a new highest priority, reset the slice and update the maxPriority
			maxPriority = cs.Spec.Priority
			highestPriorityCatalogSources = []*subv1alpha1.CatalogSource{cs}

		} else if cs.Spec.Priority == maxPriority {
			// Found another catalog source with the same highest priority, append it to the slice
			highestPriorityCatalogSources = append(highestPriorityCatalogSources, cs)
		}
	}

	switch len(highestPriorityCatalogSources) {
	case 0:
		return nil, fmt.Errorf("no catalog sources could be retrieved for MCE package")

	case 1:
		catalogSource := highestPriorityCatalogSources[0]
		log.V(2).Info(fmt.Sprintf("Using catalog source %v/%v with the highest priority: %v",
			catalogSource.Namespace, catalogSource.Name, catalogSource.Spec.Priority))
		return catalogSource, nil

	default:
		// Multiple catalog sources found with the same highest priority
		var catalogNames []string
		for _, cs := range highestPriorityCatalogSources {
			catalogNames = append(catalogNames, fmt.Sprintf("%s/%s", cs.Namespace, cs.Name))
		}

		return nil, fmt.Errorf(
			"found more than one catalogSource with expected channel with the highest priority:%v",
			catalogNames)
	}
}

// filterPackageManifests returns a list of packagemanifests containing the desired channel
// at the latest available version. Returns an empty list if no packagemanifests include the
// channel. If more than one packagemanifest have the same latest version available it will
// return them all.
func filterPackageManifests(pkgManifests []olmapi.PackageManifest, desiredChannel string) []olmapi.PackageManifest {
	filtered := []olmapi.PackageManifest{}
	latestVersion := &semver.Version{}
	for _, p := range pkgManifests {
		for _, c := range p.Status.Channels {
			if c.Name == desiredChannel {
				versionString := c.CurrentCSVDesc.Version.String()
				v, err := semver.NewVersion(versionString)
				if err != nil {
					log.Log.WithName("reconcile").Info("failed to parse version from packagemanifest", "catalogsource", p.Status.CatalogSource)
					continue
				}
				if len(filtered) == 0 {
					filtered = append(filtered, p)
					latestVersion = v
					continue
				}
				if v.Equal(latestVersion) {
					filtered = append(filtered, p)
				} else if v.GreaterThan(latestVersion) {
					filtered = []olmapi.PackageManifest{p}
					latestVersion = v
				}
			}
		}
	}
	return filtered
}

// GetMCEPackageManifests returns packagemanifests with the name multicluster-engine
func GetMCEPackageManifests(k8sClient client.Client, packageName string) ([]olmapi.PackageManifest, error) {
	ctx := context.Background()
	log := log.Log.WithName("reconcile")
	packageManifests := &olmapi.PackageManifestList{}
	var err error
	if utils.IsUnitTest() {
		// Return mock for unit tests
		return []olmapi.PackageManifest{}, nil
	} else {
		err = k8sClient.List(ctx, packageManifests)
	}
	if err != nil {
		log.Error(err, "failed to list package manifests")
		return nil, err
	}

	pkgList := []olmapi.PackageManifest{}
	for _, p := range packageManifests.Items {
		if p.Name == packageName {
			pkgList = append(pkgList, p)
		}
	}
	return pkgList, nil
}
