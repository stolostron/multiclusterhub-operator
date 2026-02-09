// Copyright Contributors to the Open Cluster Management project

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"time"

	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// MultiClusterHubReconciler reconciles a MultiClusterHub object
type MultiClusterHubReconciler struct {
	Client          client.Client
	UncachedClient  client.Client
	CacheSpec       CacheSpec
	Scheme          *runtime.Scheme
	Log             logr.Logger
	UpgradeableCond utils.Condition
}

const (
	resyncPeriod = time.Second * 20

	crdPathEnvVar       = "CRDS_PATH"
	templatesPathEnvVar = "TEMPLATES_PATH"
	templatesKind       = "multiclusterhub"
	hubFinalizer        = "finalizer.operator.open-cluster-management.io"

	trustBundleNameEnvVar  = "TRUSTED_CA_BUNDLE"
	defaultTrustBundleName = "trusted-ca-bundle"
)

var (
	log              = ctrllog.Log.WithName("reconcile")
	STSEnabledStatus = false
)

//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apiextensions.k8s.io";"apiregistration.k8s.io";"apps";"apps.open-cluster-management.io";"authorization.k8s.io";"hive.openshift.io";"mcm.ibm.com";"proxy.open-cluster-management.io";"rbac.authorization.k8s.io";"security.openshift.io";"clusterview.open-cluster-management.io";"discovery.open-cluster-management.io";"wgpolicyk8s.io",resources=apiservices;channels;clusterjoinrequests;clusterrolebindings;clusterstatuses/log;configmaps;customresourcedefinitions;deployments;discoveryconfigs;hiveconfigs;mutatingwebhookconfigurations;validatingwebhookconfigurations;namespaces;pods;policyreports;replicasets;rolebindings;secrets;serviceaccounts;services;subjectaccessreviews;subscriptions;helmreleases;managedclusters;managedclustersets,verbs=get
//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apiextensions.k8s.io";"apiregistration.k8s.io";"apps";"apps.open-cluster-management.io";"authorization.k8s.io";"hive.openshift.io";"monitoring.coreos.com";"rbac.authorization.k8s.io";"mcm.ibm.com";"security.openshift.io",resources=apiservices;channels;clusterjoinrequests;clusterrolebindings;clusterroles;configmaps;customresourcedefinitions;deployments;hiveconfigs;mutatingwebhookconfigurations;validatingwebhookconfigurations;namespaces;rolebindings;secrets;serviceaccounts;services;servicemonitors;subjectaccessreviews;subscriptions;validatingwebhookconfigurations,verbs=create;update
//+kubebuilder:rbac:groups="";"apps";"apps.open-cluster-management.io";"admissionregistration.k8s.io";"apiregistration.k8s.io";"authorization.k8s.io";"config.openshift.io";"inventory.open-cluster-management.io";"mcm.ibm.com";"observability.open-cluster-management.io";"operator.open-cluster-management.io";"rbac.authorization.k8s.io";"hive.openshift.io";"clusterview.open-cluster-management.io";"discovery.open-cluster-management.io";"wgpolicyk8s.io",resources=apiservices;clusterjoinrequests;configmaps;deployments;discoveryconfigs;helmreleases;ingresses;multiclusterhubs;multiclusterobservabilities;namespaces;hiveconfigs;rolebindings;servicemonitors;secrets;services;subjectaccessreviews;subscriptions;validatingwebhookconfigurations;pods;policyreports;managedclusters;managedclustersets,verbs=list
//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apiregistration.k8s.io";"apps";"authorization.k8s.io";"config.openshift.io";"mcm.ibm.com";"operator.open-cluster-management.io";"rbac.authorization.k8s.io";"storage.k8s.io";"apps.open-cluster-management.io";"hive.openshift.io";"clusterview.open-cluster-management.io";"wgpolicyk8s.io",resources=apiservices;helmreleases;hiveconfigs;configmaps;clusterjoinrequests;deployments;ingresses;multiclusterhubs;namespaces;rolebindings;secrets;services;subjectaccessreviews;validatingwebhookconfigurations;pods;policyreports;managedclusters;managedclustersets,verbs=watch;list
//+kubebuilder:rbac:groups="";"admissionregistration.k8s.io";"apps";"apps.open-cluster-management.io";"mcm.ibm.com";"monitoring.coreos.com";"operator.open-cluster-management.io";,resources=deployments;deployments/finalizers;helmreleases;services;services/finalizers;servicemonitors;servicemonitors/finalizers;validatingwebhookconfigurations;multiclusterhubs;multiclusterhubs/finalizers;multiclusterhubs/status,verbs=update
//+kubebuilder:rbac:groups="admissionregistration.k8s.io";"apiextensions.k8s.io";"apiregistration.k8s.io";"hive.openshift.io";"mcm.ibm.com";"rbac.authorization.k8s.io";,resources=apiservices;clusterroles;clusterrolebindings;customresourcedefinitions;hiveconfigs;mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=delete;deletecollection;list;watch;patch
//+kubebuilder:rbac:groups="";"apps";"apiregistration.k8s.io";"apps.open-cluster-management.io";"apiextensions.k8s.io";,resources=deployments;services;channels;customresourcedefinitions;apiservices,verbs=delete
//+kubebuilder:rbac:groups="";"action.open-cluster-management.io";"addon.open-cluster-management.io";"agent.open-cluster-management.io";"argoproj.io";"cluster.open-cluster-management.io";"work.open-cluster-management.io";"app.k8s.io";"apps.open-cluster-management.io";"authorization.k8s.io";"certificates.k8s.io";"clusterregistry.k8s.io";"config.openshift.io";"compliance.mcm.ibm.com";"hive.openshift.io";"hiveinternal.openshift.io";"internal.open-cluster-management.io";"inventory.open-cluster-management.io";"mcm.ibm.com";"multicloud.ibm.com";"policy.open-cluster-management.io";"proxy.open-cluster-management.io";"rbac.authorization.k8s.io";"view.open-cluster-management.io";"operator.open-cluster-management.io";"register.open-cluster-management.io";"coordination.k8s.io";"search.open-cluster-management.io";"submarineraddon.open-cluster-management.io";"discovery.open-cluster-management.io";"imageregistry.open-cluster-management.io",resources=applications;applications/status;applicationrelationships;applicationrelationships/status;certificatesigningrequests;certificatesigningrequests/approval;channels;channels/status;clustermanagementaddons;managedclusteractions;managedclusteractions/status;clusterdeployments;clusterpools;clusterclaims;discoveryconfigs;discoveredclusters;managedclusteraddons;managedclusteraddons/status;managedclusterinfos;managedclusterinfos/status;managedclustersets;managedclustersets/bind;managedclustersets/join;managedclustersets/status;managedclustersetbindings;managedclusters;managedclusters/accept;managedclusters/status;managedclusterviews;managedclusterviews/status;manifestworks;manifestworks/status;clustercurators;clustermanagers;clusterroles;clusterrolebindings;clusterstatuses/aggregator;clusterversions;compliances;configmaps;deployables;deployables/status;deployableoverrides;deployableoverrides/status;endpoints;endpointconfigs;events;helmrepos;helmrepos/status;klusterletaddonconfigs;machinepools;namespaces;placements;placementrules/status;placementdecisions;placementdecisions/status;placementrules;placementrules/status;pods;pods/log;policies;policies/status;placementbindings;policyautomations;policysets;policysets/status;roles;rolebindings;secrets;signers;subscriptions;subscriptions/status;subjectaccessreviews;submarinerconfigs;submarinerconfigs/status;syncsets;clustersyncs;leases;searchcustomizations;managedclusterimageregistries;managedclusterimageregistries/status,verbs=create;get;list;watch;update;delete;deletecollection;patch;approve;escalate;bind
//+kubebuilder:rbac:groups="operators.coreos.com",resources=catalogsources;subscriptions;clusterserviceversions;operatorgroups;operatorconditions,verbs=create;get;list;patch;update;delete;watch
//+kubebuilder:rbac:groups="multicluster.openshift.io",resources=multiclusterengines,verbs=create;get;list;patch;update;delete;watch
//+kubebuilder:rbac:groups=console.openshift.io;search.open-cluster-management.io,resources=consoleplugins;consolelinks;searches,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.openshift.io,resources=cloudcredentials;consoles,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=config.openshift.io,resources=authentications;infrastructures,verbs=get;list;watch
//+kubebuilder:rbac:groups="";"apps",resources=deployments;services;serviceaccounts,verbs=patch;delete;get;deletecollection
//+kubebuilder:rbac:groups=packages.operators.coreos.com,resources=packagemanifests,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules;servicemonitors,verbs=create;delete;get;list;watch;update;patch;deletecollection

//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=create;delete;get;list;watch;update;patch

// AgentServiceConfig webhook delete check
//+kubebuilder:rbac:groups=agent-install.openshift.io,resources=agentserviceconfigs,verbs=get;list;watch

// InternalHubComponent
// +kubebuilder:rbac:groups="operator.open-cluster-management.io",resources="internalhubcomponents",verbs=create;get;delete;patch;list;watch

// Note: The Reconcile function has been moved to reconcile.go
// Note: STS-related functions have been moved to sts.go
// Note: Component management functions have been moved to components.go
// Note: Template management functions have been moved to templates.go
// Note: InternalHubComponent functions have been moved to internal_hub_components.go
// Note: Infrastructure functions have been moved to infrastructure.go
// Note: Lifecycle functions have been moved to lifecycle.go
// Note: Configuration functions have been moved to config.go
// Note: Storage functions have been moved to storage.go
// Note: Setup functions have been moved to setup.go
