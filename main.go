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

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsapiv2 "github.com/operator-framework/api/pkg/operators/v2"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/operator/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/controllers"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/pkg/webhook"
	searchv2v1alpha1 "github.com/stolostron/search-v2-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmapi "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"

	networking "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/util/workqueue"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
	//+kubebuilder:scaffold:imports
)

const (
	OperatorVersionEnv = "OPERATOR_VERSION"
	NoCacheEnv         = "DISABLE_CLIENT_CACHE"
)

var (
	scheme        = runtime.NewScheme()
	setupLog      = ctrl.Log.WithName("setup")
	mchController controller.Controller
)

func init() {
	if _, exists := os.LookupEnv(OperatorVersionEnv); !exists {
		panic(fmt.Sprintf("%s not defined", OperatorVersionEnv))
	}

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(operatorv1.AddToScheme(scheme))

	utilruntime.Must(searchv2v1alpha1.AddToScheme(scheme))

	utilruntime.Must(apiregistrationv1.AddToScheme(scheme))

	utilruntime.Must(apixv1.AddToScheme(scheme))

	utilruntime.Must(subv1alpha1.AddToScheme(scheme))

	utilruntime.Must(operatorsapiv2.AddToScheme(scheme))

	utilruntime.Must(mcev1.AddToScheme(scheme))

	utilruntime.Must(olmv1.AddToScheme(scheme))

	utilruntime.Must(promv1.AddToScheme(scheme))

	utilruntime.Must(configv1.AddToScheme(scheme))

	utilruntime.Must(consolev1.AddToScheme(scheme))

	utilruntime.Must(olmapi.AddToScheme(scheme))

	utilruntime.Must(networking.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var leaseDuration time.Duration
	var renewDeadline time.Duration
	var retryPeriod time.Duration
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8383", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&leaseDuration, "leader-election-lease-duration", 137*time.Second, ""+
		"The duration that non-leader candidates will wait after observing a leadership "+
		"renewal until attempting to acquire leadership of a led but unrenewed leader "+
		"slot. This is effectively the maximum duration that a leader can be stopped "+
		"before it is replaced by another candidate. This is only applicable if leader "+
		"election is enabled.")
	flag.DurationVar(&renewDeadline, "leader-election-renew-deadline", 107*time.Second, ""+
		"The interval between attempts by the acting master to renew a leadership slot "+
		"before it stops leading. This must be less than or equal to the lease duration. "+
		"This is only applicable if leader election is enabled.")
	flag.DurationVar(&retryPeriod, "leader-election-retry-period", 26*time.Second, ""+
		"The duration the clients should wait between attempting acquisition and renewal "+
		"of a leadership. This is only applicable if leader election is enabled.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	ns, err := getOperatorNamespace()
	if err != nil {
		setupLog.Error(err, "failed to get operator namespace")
		os.Exit(1)
	}

	mgrOptions := ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		Port:                    8443,
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "multicloudhub-operator-lock",
		WebhookServer:           &ctrlwebhook.Server{TLSMinVersion: "1.2"},
		LeaderElectionNamespace: ns,
		LeaseDuration:           &leaseDuration,
		RenewDeadline:           &renewDeadline,
		RetryPeriod:             &retryPeriod,
	}

	mgrOptions.ClientDisableCacheFor = []client.Object{
		&corev1.Secret{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&rbacv1.RoleBinding{},
		&corev1.ConfigMap{},
		&corev1.ServiceAccount{},
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOptions)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	uncachedClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		setupLog.Error(err, "unable to create uncached client")
		os.Exit(1)
	}

	// Force OperatorCondition Upgradeable to False
	//
	// We have to at least default the condition to False or
	// OLM will use the Readiness condition via our readiness probe instead:
	// https://olm.operatorframework.io/docs/advanced-tasks/communicating-operator-conditions-to-olm/#setting-defaults
	//
	// We want to force it to False to ensure that the final decision about whether
	// the operator can be upgraded stays within the hyperconverged controller.
	setupLog.Info("Setting OperatorCondition.")
	upgradeableCondition, err := utils.NewOperatorCondition(uncachedClient, operatorsapiv2.Upgradeable)
	ctx := context.Background()

	if err != nil {
		setupLog.Error(err, "Cannot create the Upgradeable Operator Condition")
		os.Exit(1)
	}
	err = upgradeableCondition.Set(ctx, metav1.ConditionFalse, utils.UpgradeableInitReason, utils.UpgradeableInitMessage)
	if err != nil {
		setupLog.Error(err, "unable to create uncached client")
		os.Exit(1)
	}

	// re-create the condition, this time with the final client
	upgradeableCondition, err = utils.NewOperatorCondition(mgr.GetClient(), operatorsapiv2.Upgradeable)
	if err != nil {
		setupLog.Error(err, "unable to create uncached client")
		os.Exit(1)
	}
	mchReconciler := &controllers.MultiClusterHubReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		UncachedClient:  uncachedClient,
		Log:             ctrl.Log.WithName("Controller").WithName("Multiclusterhub"),
		UpgradeableCond: upgradeableCondition,
	}

	mchController, err = mchReconciler.SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MultiClusterHub")
		os.Exit(1)
	}

	// TODO: Get Webhook Working. Some troubles w/ kubebuilder generation prevented me from
	// creating the same webhook spec. May be able to get past this with Kustomize.
	// if err = (&operatorv1.MultiClusterHub{}).SetupWebhookWithManager(mgr); err != nil {
	// 	setupLog.Error(err, "unable to create webhook", "webhook", "MultiClusterHub")
	// 	os.Exit(1)
	// }

	err = webhook.Setup(mgr)
	if err != nil {
		setupLog.Error(err, "Failed to setup webhooks")
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// go routine to check if mce exist, if it does add watch
	go addMultiClusterEngineWatch(ctx, uncachedClient)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

const (
	ForceRunModeEnv = "OSDK_FORCE_RUN_MODE"
	LocalRunMode    = "local"
)

func addMultiClusterEngineWatch(ctx context.Context, uncachedClient client.Client) {
	for {
		crd := &apixv1.CustomResourceDefinition{}
		mceName := "multiclusterengines.multicluster.openshift.io"
		err := uncachedClient.Get(ctx, types.NamespacedName{Name: mceName}, crd)
		//crdKey := client.ObjectKey{Name: multiclusterengine.Namespace().GetObjectMeta().GetName()}
		//err := uncachedClient.Get(ctx, crdKey, &mcev1.MultiClusterEngine{})
		if err == nil {
			err := mchController.Watch(&source.Kind{Type: &mcev1.MultiClusterEngine{}},
				handler.Funcs{
					UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
						labels := e.ObjectNew.GetLabels()
						name := labels["installer.name"]
						if name == "" {
							name = labels["multiclusterhub.name"]
						}
						namespace := labels["installer.namespace"]
						if namespace == "" {
							namespace = labels["multiclusterhub.namespace"]
						}
						if name == "" || namespace == "" {
							l := log.FromContext(context.Background())
							l.Info(fmt.Sprintf("MCE updated, but did not find required labels: %v", labels))
							return
						}
						q.Add(
							reconcile.Request{
								NamespacedName: types.NamespacedName{
									Name:      name,
									Namespace: namespace,
								},
							},
						)
					},
				})
			if err == nil {
				setupLog.Info("mce watch added")
				return
			}
		}
		time.Sleep(30 * time.Second)
	}
}

func isRunModeLocal() bool {
	return os.Getenv(ForceRunModeEnv) == LocalRunMode
}

func getOperatorNamespace() (string, error) {
	if isRunModeLocal() {
		return "", fmt.Errorf("operator run mode forced to local")
	}

	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("namespace not found for current environment")
		}
		return "", err
	}
	ns := strings.TrimSpace(string(nsBytes))
	return ns, nil
}

func ensureCRD(mgr ctrl.Manager, crd *unstructured.Unstructured) error {
	ctx := context.Background()
	maxAttempts := 5
	go func() {
		for i := 0; i < maxAttempts; i++ {
			setupLog.Info(fmt.Sprintf("Ensuring '%s' CRD exists", crd.GetName()))
			existingCRD := &unstructured.Unstructured{}
			existingCRD.SetGroupVersionKind(crd.GroupVersionKind())
			err := mgr.GetClient().Get(ctx, types.NamespacedName{Name: crd.GetName()}, existingCRD)
			if err != nil && errors.IsNotFound(err) {
				// CRD not found. Create and return
				err = mgr.GetClient().Create(ctx, crd)
				if err != nil {
					setupLog.Error(err, fmt.Sprintf("Error creating '%s' CRD", crd.GetName()))
					time.Sleep(5 * time.Second)
					continue
				}
				return
			} else if err != nil {
				setupLog.Error(err, fmt.Sprintf("Error getting '%s' CRD", crd.GetName()))
			} else if err == nil {
				// CRD already exists. Update and return
				setupLog.Info(fmt.Sprintf("'%s' CRD already exists. Updating.", crd.GetName()))
				crd.SetResourceVersion(existingCRD.GetResourceVersion())
				err = mgr.GetClient().Update(ctx, crd)
				if err != nil {
					setupLog.Error(err, fmt.Sprintf("Error updating '%s' CRD", crd.GetName()))
					time.Sleep(5 * time.Second)
					continue
				}
				return
			}
			time.Sleep(5 * time.Second)
		}

		setupLog.Info(fmt.Sprintf("Unable to ensure '%s' CRD exists in allotted time. Failing.", crd.GetName()))
		os.Exit(1)
	}()
	return nil
}
