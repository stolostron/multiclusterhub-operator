// Copyright Contributors to the Open Cluster Management project

// Package v1 manages MCE installation via OLM v1 APIs.
//
// This package handles creating ClusterCatalog and ClusterExtension resources
// to install the MultiClusterEngine operator via OLM v1 (future OpenShift versions).
package v1

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	configv1 "github.com/openshift/api/config/v1"
	ocv1 "github.com/operator-framework/operator-controller/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// OLM v1 ClusterCatalog names (cluster-scoped, no namespace)
	// These are the default Red Hat ClusterCatalogs
	ClusterCatalogName          = "openshift-redhat-operators"
	CommunityClusterCatalogName = "openshift-community-operators"
)

// catalogQueryFunc queries a catalog for package presence. Variable allows mocking in tests.
var catalogQueryFunc = func(ctx context.Context, cl client.Client, catalogName, packageName string) (bool, error) {
	return catalogContainsPackage(ctx, cl, catalogName, packageName)
}

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

	// Filter to available and serving catalogs only
	var availableCatalogs []ocv1.ClusterCatalog
	for _, cc := range ccList.Items {
		// Skip unavailable catalogs
		if cc.Spec.AvailabilityMode == "Unavailable" {
			continue
		}

		// Skip catalogs not actively serving content
		// Check status conditions for Serving=True
		serving := false
		for _, cond := range cc.Status.Conditions {
			if cond.Type == "Serving" && cond.Status == "True" {
				serving = true
				break
			}
		}
		if !serving {
			log.V(2).Info("Skipping ClusterCatalog not in Serving state", "catalog", cc.Name)
			continue
		}

		availableCatalogs = append(availableCatalogs, cc)
	}

	if len(availableCatalogs) == 0 {
		return "", fmt.Errorf("no serving ClusterCatalogs found")
	}

	// Filter catalogs by package presence via catalogd API
	var catalogsWithPackage []ocv1.ClusterCatalog
	for _, cc := range availableCatalogs {
		containsPackage, err := catalogQueryFunc(ctx, k8sClient, cc.Name, desiredPackage)
		if err != nil {
			log.V(1).Info("Failed to query ClusterCatalog for package", "catalog", cc.Name, "error", err)
			continue
		}
		if containsPackage {
			catalogsWithPackage = append(catalogsWithPackage, cc)
		}
	}

	if len(catalogsWithPackage) == 0 {
		return "", fmt.Errorf("no ClusterCatalog found containing package %s", desiredPackage)
	}

	// Find catalog with highest priority among those containing the package
	highestPriority := catalogsWithPackage[0].Spec.Priority
	var highestPriorityCatalogs []ocv1.ClusterCatalog

	for _, cc := range catalogsWithPackage {
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
		return "", fmt.Errorf("found more than one ClusterCatalog with highest priority (%d) containing package %s: %v",
			highestPriority, desiredPackage, catalogNames)
	}

	catalog := highestPriorityCatalogs[0]
	log.Info(fmt.Sprintf("Using ClusterCatalog %s (priority: %d) for package %s",
		catalog.Name, catalog.Spec.Priority, desiredPackage))
	return catalog.Name, nil
}

// catalogContainsPackage queries catalogd API to check if a catalog contains a package.
// Uses the FBC (File-Based Catalog) v1 API: /catalogs/{catalog-name}/api/v1/all
// Response is newline-delimited JSON where each entry is a bundle with a "package" field.
func catalogContainsPackage(ctx context.Context, cl client.Client, catalogName, packageName string) (bool, error) {
	url := fmt.Sprintf("https://catalogd-service.openshift-catalogd.svc/catalogs/%s/api/v1/all", catalogName)

	// Get TLS configuration from cluster APIServer profile
	tlsProfile, err := utils.GetAPIServerTLSProfile(ctx, cl)
	if err != nil {
		// Fallback to TLS 1.2 if profile unavailable
		tlsProfile = &configv1.TLSProfileSpec{MinTLSVersion: configv1.VersionTLS12}
	}

	// Convert configv1.TLSProtocolVersion to tls version constant
	minTLSVersion := tlsVersionToUint16(tlsProfile.MinTLSVersion)

	// Skip TLS verification for in-cluster service communication
	// #nosec G402 -- catalogd service uses self-signed cert for in-cluster communication
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         minTLSVersion,
		},
	}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to query catalogd API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("catalogd API returned status %d", resp.StatusCode)
	}

	// Stream and decode newline-delimited JSON entries
	decoder := json.NewDecoder(resp.Body)
	for {
		var entry struct {
			Package string `json:"package"`
		}

		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			// Skip malformed entries
			continue
		}

		if entry.Package == packageName {
			// Drain remaining response to avoid broken pipe
			_, _ = io.Copy(io.Discard, resp.Body)
			return true, nil
		}
	}

	return false, nil
}

// tlsVersionToUint16 converts configv1.TLSProtocolVersion to crypto/tls version constant
func tlsVersionToUint16(version configv1.TLSProtocolVersion) uint16 {
	switch version {
	case configv1.VersionTLS10:
		return tls.VersionTLS10
	case configv1.VersionTLS11:
		return tls.VersionTLS11
	case configv1.VersionTLS12:
		return tls.VersionTLS12
	case configv1.VersionTLS13:
		return tls.VersionTLS13
	default:
		return tls.VersionTLS12 // Default to TLS 1.2
	}
}
