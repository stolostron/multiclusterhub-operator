// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// platformCloudResources maps platform names to the types of cloud resources at risk.
var platformCloudResources = map[string]map[string]string{
	"AWS": {
		"hypershift.openshift.io/finalizer":                            "EC2 instances, Elastic Load Balancers, security groups, Route53 DNS records, NAT gateways, VPC resources",
		"hypershift.io/aws-oidc-discovery":                             "S3 OIDC discovery documents (stale identity provider configuration)",
		"hypershift.openshift.io/control-plane-operator-finalizer":     "AWS PrivateLink VPC endpoint services",
		"hive.openshift.io/deprovision":                                "EC2 instances, EBS volumes, Elastic Load Balancers, security groups, Route53 records, VPC, subnets, NAT gateways, S3 buckets",
		"agentserviceconfig.agent-install.openshift.io/ai-deprovision": "Assisted-installer provisioned bare-metal or cloud resources",
	},
	"AZURE": {
		"hypershift.openshift.io/finalizer":                            "Azure VMs, Load Balancers, Network Security Groups, DNS zones, VNets",
		"hypershift.openshift.io/control-plane-operator-finalizer":     "Azure Private Link services",
		"hive.openshift.io/deprovision":                                "Azure VMs, managed disks, Load Balancers, NSGs, DNS zones, VNets, subnets, storage accounts",
		"agentserviceconfig.agent-install.openshift.io/ai-deprovision": "Assisted-installer provisioned Azure resources",
	},
	"GCP": {
		"hypershift.openshift.io/finalizer":                            "GCE instances, forwarding rules, firewall rules, Cloud DNS records, VPC networks",
		"hypershift.openshift.io/control-plane-operator-finalizer":     "GCP Private Service Connect forwarding rules",
		"hive.openshift.io/deprovision":                                "GCE instances, persistent disks, forwarding rules, firewall rules, Cloud DNS, VPC, subnets, Cloud Storage buckets",
		"agentserviceconfig.agent-install.openshift.io/ai-deprovision": "Assisted-installer provisioned GCP resources",
	},
}

// genericCloudResources is the fallback when platform is unknown.
var genericCloudResources = map[string]string{
	"hypershift.openshift.io/finalizer":                            "cloud VMs, load balancers, security groups, DNS records, and networking resources",
	"hypershift.io/aws-oidc-discovery":                             "S3 OIDC discovery documents",
	"hypershift.openshift.io/control-plane-operator-finalizer":     "cloud private link / endpoint services",
	"hive.openshift.io/deprovision":                                "full cluster cloud infrastructure (VMs, networking, storage, DNS)",
	"agentserviceconfig.agent-install.openshift.io/ai-deprovision": "assisted-installer provisioned infrastructure resources",
}

// buildCloudWarnings generates CloudResourceWarning entries for all Tier 1 finalizers in the graph.
func (r *HubTeardownReconciler) buildCloudWarnings(ctx context.Context, log logr.Logger, graph *DependencyGraph) []operatorv1.CloudResourceWarning {
	var warnings []operatorv1.CloudResourceWarning

	for _, dr := range graph.CloudRiskResources {
		for _, finalizer := range dr.Finalizers {
			if !IsCloudProtectingFinalizer(finalizer) {
				continue
			}

			riskSummary := buildRiskSummary(dr, finalizer)

			warnings = append(warnings, operatorv1.CloudResourceWarning{
				Resource:     dr.Ref,
				Finalizer:    finalizer,
				RiskSummary:  riskSummary,
				Platform:     dr.Platform,
				Acknowledged: false,
			})
		}
	}

	return warnings
}

// buildRiskSummary generates a human-readable warning for a specific cloud-protecting finalizer.
func buildRiskSummary(dr DiscoveredResource, finalizer string) string {
	platform := dr.Platform

	var cloudResourceDesc string
	if platform != "" {
		if platformMap, ok := platformCloudResources[platform]; ok {
			if desc, ok := platformMap[finalizer]; ok {
				cloudResourceDesc = desc
			}
		}
	}
	if cloudResourceDesc == "" {
		if desc, ok := genericCloudResources[finalizer]; ok {
			cloudResourceDesc = desc
		} else {
			cloudResourceDesc = "cloud infrastructure resources"
		}
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Removing finalizer %q from %s/%s",
		finalizer, dr.Ref.Kind, dr.Ref.Name))

	if dr.Ref.Namespace != "" {
		parts[0] = fmt.Sprintf("Removing finalizer %q from %s/%s/%s",
			finalizer, dr.Ref.Kind, dr.Ref.Namespace, dr.Ref.Name)
	}

	parts = append(parts, fmt.Sprintf("will orphan %s.", cloudResourceDesc))

	if dr.Phase != "" {
		parts = append(parts, fmt.Sprintf("The resource is currently in %q state.", dr.Phase))
	}

	if dr.IsDeleting {
		parts = append(parts, "The resource is already in terminating state (deletion timestamp set).")
	}

	platformHint := "your cloud provider console"
	switch platform {
	case "AWS":
		platformHint = "the AWS Management Console or CLI"
	case "AZURE":
		platformHint = "the Azure Portal or CLI"
	case "GCP":
		platformHint = "the Google Cloud Console or gcloud CLI"
	}
	parts = append(parts, fmt.Sprintf("Verify these resources manually in %s before acknowledging.", platformHint))

	return strings.Join(parts, " ")
}

// emitCloudWarningEvents emits Kubernetes Warning events for each cloud resource warning.
func (r *HubTeardownReconciler) emitCloudWarningEvents(td *operatorv1.HubTeardown) {
	for _, w := range td.Status.CloudResourceWarnings {
		r.Recorder.Eventf(td, corev1.EventTypeWarning, "CloudResourceLeakRisk",
			"[%s] %s/%s/%s: finalizer %q protects cloud resources. %s",
			w.Platform, w.Resource.Kind, w.Resource.Namespace, w.Resource.Name,
			w.Finalizer, w.RiskSummary)
	}
}

// emitOrphanSummaryEvent emits a final summary event listing all cloud resources that were orphaned.
func (r *HubTeardownReconciler) emitOrphanSummaryEvent(td *operatorv1.HubTeardown) {
	var orphaned []string
	for _, w := range td.Status.CloudResourceWarnings {
		if td.Spec.AcknowledgeCloudResourceRisk {
			orphaned = append(orphaned, fmt.Sprintf("%s/%s/%s (%s: %s)",
				w.Resource.Kind, w.Resource.Namespace, w.Resource.Name,
				w.Platform, truncateMessage(w.RiskSummary, 200)))
		}
	}

	if len(orphaned) == 0 {
		r.Recorder.Event(td, corev1.EventTypeNormal, "TeardownComplete",
			"Teardown completed with no cloud resources orphaned.")
		return
	}

	summary := fmt.Sprintf("Teardown completed. The following cloud resources were orphaned and require manual cleanup:\n%s",
		strings.Join(orphaned, "\n"))
	r.Recorder.Event(td, corev1.EventTypeWarning, "CloudResourcesOrphaned", truncateMessage(summary, 1024))
}
