// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	v1 "github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine/olm/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	clog "sigs.k8s.io/controller-runtime/pkg/log"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	ocv1 "github.com/operator-framework/operator-controller/api/v1"
)

func TestEnsureServiceAccount(t *testing.T) {
	testNamespace := "test-namespace"
	saName := "test-sa"

	tests := []struct {
		name          string
		existingSA    *corev1.ServiceAccount
		newSA         *corev1.ServiceAccount
		mch           *operatorsv1.MultiClusterHub
		wantRequeue   bool
		wantCondition bool
		setupClient   func(*testing.T) client.Client
	}{
		{
			name: "ServiceAccount already exists - no action needed",
			existingSA: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: testNamespace,
				},
			},
			newSA: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: testNamespace,
				},
			},
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: testNamespace,
				},
			},
			wantRequeue:   false,
			wantCondition: false,
		},
		{
			name:       "ServiceAccount doesn't exist - create it",
			existingSA: nil,
			newSA: &corev1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: testNamespace,
				},
			},
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: testNamespace,
				},
			},
			wantRequeue:   false,
			wantCondition: true,
		},
		{
			name:       "Get returns unexpected error - requeue",
			existingSA: nil,
			newSA: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: testNamespace,
				},
			},
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: testNamespace,
				},
			},
			setupClient: func(t *testing.T) client.Client {
				// Return client that errors on Get (not IsNotFound)
				s := scheme.Scheme
				_ = corev1.AddToScheme(s)
				_ = operatorsv1.AddToScheme(s)

				// Use interceptor to return error
				return &errorClient{
					Client: fake.NewClientBuilder().WithScheme(s).Build(),
					getErr: apierrors.NewInternalError(errors.New("unexpected error")),
				}
			},
			wantRequeue:   true,
			wantCondition: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			_ = corev1.AddToScheme(s)
			_ = operatorsv1.AddToScheme(s)

			var fakeClient client.Client
			if tt.setupClient != nil {
				fakeClient = tt.setupClient(t)
			} else {
				objects := []client.Object{}
				if tt.existingSA != nil {
					objects = append(objects, tt.existingSA)
				}
				fakeClient = fake.NewClientBuilder().
					WithScheme(s).
					WithObjects(objects...).
					Build()
			}

			reconciler := &MultiClusterHubReconciler{
				Client: fakeClient,
				Scheme: s,
				Log:    clog.Log.WithName("test"),
			}

			result, err := reconciler.ensureServiceAccount(tt.mch, tt.newSA)

			if tt.wantRequeue {
				if result.Requeue == false {
					t.Errorf("ensureServiceAccount() expected requeue but got none")
				}
			} else {
				if result != (ctrl.Result{}) {
					t.Errorf("ensureServiceAccount() unexpected result: %v", result)
				}
				if err != nil {
					t.Errorf("ensureServiceAccount() unexpected error: %v", err)
				}
			}

			// Check if ServiceAccount was created
			if !tt.wantRequeue && tt.existingSA == nil {
				sa := &corev1.ServiceAccount{}
				err := fakeClient.Get(context.Background(), types.NamespacedName{
					Name:      tt.newSA.GetName(),
					Namespace: tt.newSA.GetNamespace(),
				}, sa)
				if err != nil {
					t.Errorf("ensureServiceAccount() ServiceAccount not created: %v", err)
				}
			}

			// Check condition was set
			if tt.wantCondition {
				condition := FindCondition(tt.mch.Status.HubConditions, operatorsv1.Progressing)
				if condition == nil {
					t.Errorf("ensureServiceAccount() expected Progressing condition but not found")
				}
			}
		})
	}
}

func TestEnsureClusterRoleBinding(t *testing.T) {
	crbName := "test-crb"

	tests := []struct {
		name          string
		existingCRB   *rbacv1.ClusterRoleBinding
		newCRB        *rbacv1.ClusterRoleBinding
		mch           *operatorsv1.MultiClusterHub
		wantRequeue   bool
		wantCondition bool
		setupClient   func(*testing.T) client.Client
	}{
		{
			name: "ClusterRoleBinding already exists - no action needed",
			existingCRB: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: crbName,
				},
			},
			newCRB: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: crbName,
				},
			},
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "test-namespace",
				},
			},
			wantRequeue:   false,
			wantCondition: false,
		},
		{
			name:        "ClusterRoleBinding doesn't exist - create it",
			existingCRB: nil,
			newCRB: &rbacv1.ClusterRoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: crbName,
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "test-role",
				},
			},
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "test-namespace",
				},
			},
			wantRequeue:   false,
			wantCondition: true,
		},
		{
			name:        "Get returns unexpected error - requeue",
			existingCRB: nil,
			newCRB: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: crbName,
				},
			},
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "test-namespace",
				},
			},
			setupClient: func(t *testing.T) client.Client {
				s := scheme.Scheme
				_ = rbacv1.AddToScheme(s)
				_ = operatorsv1.AddToScheme(s)

				return &errorClient{
					Client: fake.NewClientBuilder().WithScheme(s).Build(),
					getErr: apierrors.NewInternalError(errors.New("unexpected error")),
				}
			},
			wantRequeue:   true,
			wantCondition: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			_ = rbacv1.AddToScheme(s)
			_ = operatorsv1.AddToScheme(s)

			var fakeClient client.Client
			if tt.setupClient != nil {
				fakeClient = tt.setupClient(t)
			} else {
				objects := []client.Object{}
				if tt.existingCRB != nil {
					objects = append(objects, tt.existingCRB)
				}
				fakeClient = fake.NewClientBuilder().
					WithScheme(s).
					WithObjects(objects...).
					Build()
			}

			reconciler := &MultiClusterHubReconciler{
				Client: fakeClient,
				Scheme: s,
				Log:    clog.Log.WithName("test"),
			}

			result, err := reconciler.ensureClusterRoleBinding(tt.mch, tt.newCRB)

			if tt.wantRequeue {
				if result.Requeue == false {
					t.Errorf("ensureClusterRoleBinding() expected requeue but got none")
				}
			} else {
				if result != (ctrl.Result{}) {
					t.Errorf("ensureClusterRoleBinding() unexpected result: %v", result)
				}
				if err != nil {
					t.Errorf("ensureClusterRoleBinding() unexpected error: %v", err)
				}
			}

			// Check if ClusterRoleBinding was created
			if !tt.wantRequeue && tt.existingCRB == nil {
				crb := &rbacv1.ClusterRoleBinding{}
				err := fakeClient.Get(context.Background(), types.NamespacedName{
					Name: tt.newCRB.GetName(),
				}, crb)
				if err != nil {
					t.Errorf("ensureClusterRoleBinding() ClusterRoleBinding not created: %v", err)
				}
			}

			// Check condition was set
			if tt.wantCondition {
				condition := FindCondition(tt.mch.Status.HubConditions, operatorsv1.Progressing)
				if condition == nil {
					t.Errorf("ensureClusterRoleBinding() expected Progressing condition but not found")
				}
			}
		})
	}
}

func TestEnsureMultiClusterEngineCR(t *testing.T) {
	testNamespace := "test-namespace"
	mceName := "test-mce"

	tests := []struct {
		name        string
		existingMCE *mcev1.MultiClusterEngine
		mch         *operatorsv1.MultiClusterHub
		olmVersion  string
		objects     []client.Object
		wantError   bool
		wantRequeue bool
		wantCreate  bool
	}{
		{
			name: "MCE already exists - update it",
			existingMCE: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: mceName,
				},
				Spec: mcev1.MultiClusterEngineSpec{
					TargetNamespace: testNamespace,
				},
			},
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: testNamespace,
				},
			},
			olmVersion:  "v0",
			wantError:   false,
			wantRequeue: false,
			wantCreate:  false,
		},
		{
			name:        "MCE doesn't exist, OLM v1 - create with ClusterExtension namespace",
			existingMCE: nil,
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: testNamespace,
				},
			},
			olmVersion: "v1",
			objects: []client.Object{
				&ocv1.ClusterExtension{
					ObjectMeta: metav1.ObjectMeta{
						Name: "multicluster-engine",
					},
					Spec: ocv1.ClusterExtensionSpec{
						Namespace: "mce-namespace",
					},
				},
			},
			wantError:   false,
			wantRequeue: false,
			wantCreate:  true,
		},
		{
			name:        "MCE doesn't exist, OLM v0 - create with Subscription namespace",
			existingMCE: nil,
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: testNamespace,
				},
			},
			olmVersion: "v0",
			objects: []client.Object{
				&subv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multicluster-engine",
						Namespace: "mce-sub-namespace",
					},
					Spec: &subv1alpha1.SubscriptionSpec{
						Package: "multicluster-engine",
					},
				},
			},
			wantError:   false,
			wantRequeue: false,
			wantCreate:  true,
		},
		{
			name: "MCE exists but no targetNamespace - error",
			existingMCE: &mcev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: mceName,
				},
				Spec: mcev1.MultiClusterEngineSpec{
					// TargetNamespace not set
				},
			},
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: testNamespace,
				},
			},
			olmVersion:  "v0",
			wantError:   true,
			wantRequeue: false,
			wantCreate:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			_ = mcev1.AddToScheme(s)
			_ = operatorsv1.AddToScheme(s)
			_ = corev1.AddToScheme(s)
			_ = ocv1.AddToScheme(s)
			_ = subv1alpha1.AddToScheme(s)

			objects := []client.Object{}
			if tt.existingMCE != nil {
				objects = append(objects, tt.existingMCE)
			}
			objects = append(objects, tt.objects...)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(objects...).
				Build()

			reconciler := &MultiClusterHubReconciler{
				Client:     fakeClient,
				Scheme:     s,
				Log:        clog.Log.WithName("test"),
				OLMVersion: tt.olmVersion,
			}

			ctx := context.Background()
			result, err := reconciler.ensureMultiClusterEngineCR(ctx, tt.mch)

			if tt.wantError {
				if err == nil {
					t.Errorf("ensureMultiClusterEngineCR() expected error but got none")
				}
				if tt.wantRequeue && result.Requeue == false {
					t.Errorf("ensureMultiClusterEngineCR() expected requeue but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ensureMultiClusterEngineCR() unexpected error: %v", err)
				return
			}

			// Check if MCE was created
			if tt.wantCreate {
				mceList := &mcev1.MultiClusterEngineList{}
				err := fakeClient.List(ctx, mceList)
				if err != nil {
					t.Errorf("ensureMultiClusterEngineCR() failed to list MCE: %v", err)
					return
				}
				if len(mceList.Items) == 0 {
					t.Errorf("ensureMultiClusterEngineCR() MCE not created")
				}
			}
		})
	}
}

func TestEnsureMultiClusterEngineCR_NoMatchError(t *testing.T) {
	noMatchErr := &apimeta.NoKindMatchError{
		GroupKind: schema.GroupKind{Group: "multicluster.openshift.io", Kind: "MultiClusterEngine"},
	}

	noMatchClient := &listErrorClient{
		Client:  fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
		listErr: noMatchErr,
	}

	reconciler := &MultiClusterHubReconciler{
		Client: noMatchClient,
		Scheme: scheme.Scheme,
		Log:    clog.Log.WithName("test"),
	}

	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mch",
			Namespace: "test-namespace",
		},
	}

	ctx := context.Background()
	result, err := reconciler.ensureMultiClusterEngineCR(ctx, mch)

	if err != nil {
		t.Errorf("ensureMultiClusterEngineCR() expected nil error for NoMatchError, got: %v", err)
	}
	if result.RequeueAfter != resyncPeriod {
		t.Errorf("ensureMultiClusterEngineCR() expected RequeueAfter=%v, got %v", resyncPeriod, result.RequeueAfter)
	}
}

func TestEnsureMultiClusterEngine(t *testing.T) {
	tests := []struct {
		name       string
		olmVersion string
		client     client.Client
		wantError  bool
		wantResult ctrl.Result
	}{
		{
			name:       "v1 get error causes requeue",
			olmVersion: "v1",
			client: &errorClient{
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
				getErr: fmt.Errorf("simulated get error"),
			},
			wantError:  false,
			wantResult: ctrl.Result{Requeue: true},
		},
		{
			name:       "no OLM - MCE CR NoMatchError propagates as requeue",
			olmVersion: "",
			client: &listErrorClient{
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
				listErr: &apimeta.NoKindMatchError{
					GroupKind: schema.GroupKind{Group: "multicluster.openshift.io", Kind: "MultiClusterEngine"},
				},
			},
			wantError:  false,
			wantResult: ctrl.Result{RequeueAfter: resyncPeriod},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &MultiClusterHubReconciler{
				Client:     tt.client,
				Scheme:     scheme.Scheme,
				Log:        clog.Log.WithName("test"),
				OLMVersion: tt.olmVersion,
			}

			mch := &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "test-namespace",
				},
			}

			ctx := context.Background()
			result, err := reconciler.ensureMultiClusterEngine(ctx, mch)

			if tt.wantError {
				if err == nil {
					t.Errorf("ensureMultiClusterEngine() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("ensureMultiClusterEngine() unexpected error: %v", err)
				}
			}
			if result != tt.wantResult {
				t.Errorf("ensureMultiClusterEngine() result = %v, want %v", result, tt.wantResult)
			}
		})
	}
}

// errorClient wraps a fake client and returns errors for Get operations
type errorClient struct {
	client.Client
	getErr error
}

func (e *errorClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	if e.getErr != nil {
		return e.getErr
	}
	return e.Client.Get(ctx, key, obj, opts...)
}

// listErrorClient wraps a fake client and returns errors for List operations
type listErrorClient struct {
	client.Client
	listErr error
}

func (e *listErrorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if e.listErr != nil {
		return e.listErr
	}
	return e.Client.List(ctx, list, opts...)
}

// Helper to find condition in status
func FindCondition(conditions []operatorsv1.HubCondition, condType operatorsv1.HubConditionType) *operatorsv1.HubCondition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

func TestListCustomResources(t *testing.T) {
	tests := []struct {
		name       string
		olmVersion string
		objects    []client.Object
		wantKeys   []string
	}{
		{
			name:       "OLM v1 - ClusterExtension present",
			olmVersion: "v1",
			objects: []client.Object{
				&ocv1.ClusterExtension{
					ObjectMeta: metav1.ObjectMeta{
						Name: "multicluster-engine",
						Labels: map[string]string{
							"multiclusterhubs.operator.open-cluster-management.io/managed-by": "true",
						},
					},
				},
				&mcev1.MultiClusterEngine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "multiclusterengine",
					},
				},
			},
			wantKeys: []string{"mce-clusterextension", "mce"},
		},
		{
			name:       "OLM v0 - Subscription and CSV present",
			olmVersion: "v0",
			objects: []client.Object{
				&subv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multicluster-engine",
						Namespace: "multicluster-engine",
						Labels: map[string]string{
							"multiclusterhubs.operator.open-cluster-management.io/managed-by": "true",
						},
					},
					Spec: &subv1alpha1.SubscriptionSpec{
						Package: "multicluster-engine",
					},
					Status: subv1alpha1.SubscriptionStatus{
						CurrentCSV:   "multicluster-engine.v2.0.0",
						InstalledCSV: "multicluster-engine.v2.0.0",
					},
				},
				&subv1alpha1.ClusterServiceVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multicluster-engine.v2.0.0",
						Namespace: "multicluster-engine",
					},
				},
				&mcev1.MultiClusterEngine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "multiclusterengine",
					},
				},
			},
			wantKeys: []string{"mce-sub", "mce-csv", "mce"},
		},
		{
			name:       "No OLM - only MCE CR",
			olmVersion: "",
			objects: []client.Object{
				&mcev1.MultiClusterEngine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "multiclusterengine",
					},
				},
			},
			wantKeys: []string{"mce"},
		},
		{
			name:       "OLM v1 - no resources present",
			olmVersion: "v1",
			objects:    []client.Object{},
			wantKeys:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			_ = mcev1.AddToScheme(s)
			_ = operatorsv1.AddToScheme(s)
			_ = ocv1.AddToScheme(s)
			_ = subv1alpha1.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(tt.objects...).
				Build()

			reconciler := &MultiClusterHubReconciler{
				Client:     fakeClient,
				Scheme:     s,
				Log:        clog.Log.WithName("test"),
				OLMVersion: tt.olmVersion,
			}

			result, err := reconciler.listCustomResources()
			if err != nil {
				t.Errorf("listCustomResources() unexpected error: %v", err)
				return
			}

			// Check expected keys present
			for _, key := range tt.wantKeys {
				if _, exists := result[key]; !exists {
					t.Errorf("listCustomResources() missing expected key: %s", key)
				}
			}

			// For OLM scenarios, verify correct number of keys
			if tt.olmVersion != "" && len(tt.objects) > 0 {
				// Account for mce key which is always present
				if len(result) != len(tt.wantKeys) {
					t.Errorf("listCustomResources() got %d keys, want %d", len(result), len(tt.wantKeys))
				}
			}
		})
	}
}

func TestAddInstallerLabelSecret(t *testing.T) {
	tests := []struct {
		name          string
		secret        *corev1.Secret
		installerName string
		installerNS   string
		wantUpdated   bool
		wantLabels    map[string]string
	}{
		{
			name: "No labels exist - add both",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secret",
				},
			},
			installerName: "test-installer",
			installerNS:   "test-ns",
			wantUpdated:   true,
			wantLabels: map[string]string{
				"installer.name":      "test-installer",
				"installer.namespace": "test-ns",
			},
		},
		{
			name: "Labels already correct - no update",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secret",
					Labels: map[string]string{
						"installer.name":      "test-installer",
						"installer.namespace": "test-ns",
					},
				},
			},
			installerName: "test-installer",
			installerNS:   "test-ns",
			wantUpdated:   false,
			wantLabels: map[string]string{
				"installer.name":      "test-installer",
				"installer.namespace": "test-ns",
			},
		},
		{
			name: "Name label wrong - update",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secret",
					Labels: map[string]string{
						"installer.name":      "wrong-name",
						"installer.namespace": "test-ns",
					},
				},
			},
			installerName: "test-installer",
			installerNS:   "test-ns",
			wantUpdated:   true,
			wantLabels: map[string]string{
				"installer.name":      "test-installer",
				"installer.namespace": "test-ns",
			},
		},
		{
			name: "Namespace label wrong - update",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secret",
					Labels: map[string]string{
						"installer.name":      "test-installer",
						"installer.namespace": "wrong-ns",
					},
				},
			},
			installerName: "test-installer",
			installerNS:   "test-ns",
			wantUpdated:   true,
			wantLabels: map[string]string{
				"installer.name":      "test-installer",
				"installer.namespace": "test-ns",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := addInstallerLabelSecret(tt.secret, tt.installerName, tt.installerNS)

			if updated != tt.wantUpdated {
				t.Errorf("addInstallerLabelSecret() updated = %v, want %v", updated, tt.wantUpdated)
			}

			for key, want := range tt.wantLabels {
				if got := tt.secret.Labels[key]; got != want {
					t.Errorf("addInstallerLabelSecret() label[%s] = %v, want %v", key, got, want)
				}
			}
		})
	}
}

func TestEnsureMCESubscription(t *testing.T) {
	// Set POD_NAMESPACE for tests
	t.Setenv("POD_NAMESPACE", "test-ns")

	tests := []struct {
		name        string
		olmVersion  string
		objects     []client.Object
		mch         *operatorsv1.MultiClusterHub
		wantError   bool
		wantRequeue bool
	}{
		{
			name:       "No OLM - skip subscription management",
			olmVersion: "",
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "test-ns",
				},
			},
			wantError:   false,
			wantRequeue: false,
		},
		{
			name:       "OLM v1 - delegates to ensureMCEClusterExtension",
			olmVersion: "v1",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "multicluster-engine",
					},
				},
			},
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "test-ns",
				},
			},
			wantError:   false,
			wantRequeue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			_ = mcev1.AddToScheme(s)
			_ = operatorsv1.AddToScheme(s)
			_ = subv1alpha1.AddToScheme(s)
			_ = corev1.AddToScheme(s)
			_ = ocv1.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(tt.objects...).
				Build()

			reconciler := &MultiClusterHubReconciler{
				Client:     fakeClient,
				Scheme:     s,
				Log:        clog.Log.WithName("test"),
				OLMVersion: tt.olmVersion,
			}

			ctx := context.Background()
			result, err := reconciler.ensureMCEInstallation(ctx, tt.mch)

			if tt.wantError {
				if err == nil {
					t.Errorf("ensureMCEInstallation() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ensureMCEInstallation() unexpected error: %v", err)
				return
			}

			if tt.wantRequeue && result.Requeue == false {
				t.Errorf("ensureMCEInstallation() expected requeue but got none")
			}
		})
	}
}

func TestEnsureMCEClusterExtension(t *testing.T) {
	tests := []struct {
		name        string
		objects     []client.Object
		mch         *operatorsv1.MultiClusterHub
		wantError   bool
		wantRequeue bool
	}{
		{
			name: "No ClusterCatalog found - proceeds with CE creation",
			objects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "multicluster-engine",
					},
				},
			},
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "test-ns",
				},
			},
			wantError:   false,
			wantRequeue: false,
		},
		{
			name: "No serving ClusterCatalog - proceeds with CE creation",
			objects: []client.Object{
				&ocv1.ClusterCatalog{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-catalog",
					},
					Status: ocv1.ClusterCatalogStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Serving",
								Status: metav1.ConditionFalse,
							},
						},
					},
				},
			},
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "test-ns",
				},
			},
			wantError:   false,
			wantRequeue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			_ = mcev1.AddToScheme(s)
			_ = operatorsv1.AddToScheme(s)
			_ = ocv1.AddToScheme(s)
			_ = corev1.AddToScheme(s)
			_ = rbacv1.AddToScheme(s)

			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(tt.objects...).
				Build()

			reconciler := &MultiClusterHubReconciler{
				Client:     fakeClient,
				Scheme:     s,
				Log:        clog.Log.WithName("test"),
				OLMVersion: "v1",
			}

			ctx := context.Background()
			result, err := reconciler.ensureMCEClusterExtension(ctx, tt.mch)

			if tt.wantError {
				if err == nil {
					t.Errorf("ensureMCEClusterExtension() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ensureMCEClusterExtension() unexpected error: %v", err)
				return
			}

			if tt.wantRequeue && result.Requeue == false {
				t.Errorf("ensureMCEClusterExtension() expected requeue but got none")
			}
		})
	}
}

func TestCheckSubscriptionAnnotationConflicts(t *testing.T) {
	// Get actual desired channel from multiclusterengine package
	desiredChannel := multiclusterengine.DesiredChannel()
	differentChannel := "different-channel"

	tests := []struct {
		name      string
		overrides *subv1alpha1.SubscriptionSpec
		wantLogs  int // number of warning logs expected
	}{
		{
			name:      "No overrides - no warnings",
			overrides: nil,
			wantLogs:  0,
		},
		{
			name:      "Empty overrides - no warnings",
			overrides: &subv1alpha1.SubscriptionSpec{},
			wantLogs:  0,
		},
		{
			name: "Channel matches desired - no channel warning",
			overrides: &subv1alpha1.SubscriptionSpec{
				Channel: desiredChannel,
			},
			wantLogs: 0,
		},
		{
			name: "Channel differs from desired - warning",
			overrides: &subv1alpha1.SubscriptionSpec{
				Channel: differentChannel,
			},
			wantLogs: 1,
		},
		{
			name: "StartingCSV set - warning",
			overrides: &subv1alpha1.SubscriptionSpec{
				StartingCSV: "multicluster-engine.v2.6.0",
			},
			wantLogs: 1,
		},
		{
			name: "Both channel conflict and startingCSV - two warnings",
			overrides: &subv1alpha1.SubscriptionSpec{
				Channel:     differentChannel,
				StartingCSV: "multicluster-engine.v2.5.0",
			},
			wantLogs: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use test logger that counts log calls
			logCount := 0
			testSink := &testLogger{
				onInfo: func(msg string, keysAndValues ...interface{}) {
					if len(msg) > 0 && msg[0:7] == "WARNING" {
						logCount++
					}
				},
			}
			testLog := logr.New(testSink)

			checkSubscriptionAnnotationConflicts(testLog, tt.overrides)

			if logCount != tt.wantLogs {
				t.Errorf("checkSubscriptionAnnotationConflicts() logged %d warnings, want %d", logCount, tt.wantLogs)
			}
		})
	}
}

func TestCheckClusterExtensionAnnotationConflicts(t *testing.T) {
	// Get actual desired channel from multiclusterengine package
	desiredChannel := multiclusterengine.DesiredChannel()
	differentChannel := "different-channel"

	tests := []struct {
		name      string
		overrides *v1.ClusterExtensionOverrides
		wantLogs  int
	}{
		{
			name:      "No overrides - no warnings",
			overrides: nil,
			wantLogs:  0,
		},
		{
			name:      "Empty overrides - no warnings",
			overrides: &v1.ClusterExtensionOverrides{},
			wantLogs:  0,
		},
		{
			name: "Channels include desired - no channel warning",
			overrides: &v1.ClusterExtensionOverrides{
				Channels: []string{desiredChannel},
			},
			wantLogs: 0,
		},
		{
			name: "Channels exclude desired - warning",
			overrides: &v1.ClusterExtensionOverrides{
				Channels: []string{differentChannel},
			},
			wantLogs: 1,
		},
		{
			name: "Version set - warning",
			overrides: &v1.ClusterExtensionOverrides{
				Version: ">=2.6.0 <2.7.0",
			},
			wantLogs: 1,
		},
		{
			name: "Both channel conflict and version - two warnings",
			overrides: &v1.ClusterExtensionOverrides{
				Channels: []string{differentChannel},
				Version:  ">=2.5.0 <2.6.0",
			},
			wantLogs: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logCount := 0
			testSink := &testLogger{
				onInfo: func(msg string, keysAndValues ...interface{}) {
					if len(msg) > 0 && msg[0:7] == "WARNING" {
						logCount++
					}
				},
			}
			testLog := logr.New(testSink)

			checkClusterExtensionAnnotationConflicts(testLog, tt.overrides)

			if logCount != tt.wantLogs {
				t.Errorf("checkClusterExtensionAnnotationConflicts() logged %d warnings, want %d", logCount, tt.wantLogs)
			}
		})
	}
}

// testLogger is a simple logger for testing that counts log calls
type testLogger struct {
	onInfo func(msg string, keysAndValues ...interface{})
}

func (l *testLogger) Info(level int, msg string, keysAndValues ...interface{}) {
	if l.onInfo != nil {
		l.onInfo(msg, keysAndValues...)
	}
}

func (l *testLogger) Enabled(level int) bool { return true }
func (l *testLogger) Error(err error, msg string, keysAndValues ...interface{}) {
}
func (l *testLogger) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return l
}
func (l *testLogger) WithName(name string) logr.LogSink {
	return l
}
func (l *testLogger) Init(info logr.RuntimeInfo) {
}

// --- HubCondition tests ---
//
// The following tests cover the status condition updates added so that
// `kubectl get mch` surfaces what is blocking OperatorGroup reconciliation
// and MultiClusterEngine readiness/creation.

// Test_ensureOperatorGroup_SetsRequirementsNotMetCondition_MultipleGroups
// verifies that finding more than one OperatorGroup in a namespace records a
// Progressing/RequirementsNotMetReason condition naming the namespace,
// instead of only logging the conflict. This is a terminal misconfiguration
// requiring manual intervention, so it should not requeue on a timer.
func Test_ensureOperatorGroup_SetsRequirementsNotMetCondition_MultipleGroups(t *testing.T) {
	s := scheme.Scheme
	_ = olmv1.AddToScheme(s)
	_ = operatorsv1.AddToScheme(s)

	ogNamespace := "test-namespace-multiple-og"
	og1 := &olmv1.OperatorGroup{ObjectMeta: metav1.ObjectMeta{Name: "og-one", Namespace: ogNamespace}}
	og2 := &olmv1.OperatorGroup{ObjectMeta: metav1.ObjectMeta{Name: "og-two", Namespace: ogNamespace}}

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(og1, og2).Build()
	reconciler := &MultiClusterHubReconciler{
		Client: fakeClient,
		Scheme: s,
		Log:    clog.Log.WithName("test"),
	}

	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Name: "test-mch", Namespace: ogNamespace},
	}
	desiredOG := &olmv1.OperatorGroup{ObjectMeta: metav1.ObjectMeta{Name: "desired-og", Namespace: ogNamespace}}

	result, err := reconciler.ensureOperatorGroup(mch, desiredOG)
	if err != nil {
		t.Fatalf("ensureOperatorGroup() unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Errorf("ensureOperatorGroup() expected empty result (no requeue for a terminal misconfiguration), got %+v", result)
	}

	condition := GetHubCondition(mch.Status, operatorsv1.Progressing)
	if condition == nil {
		t.Fatal("expected Progressing condition to be set")
	}
	if condition.Reason != RequirementsNotMetReason {
		t.Errorf("expected reason %q, got %q", RequirementsNotMetReason, condition.Reason)
	}
	if condition.Status != metav1.ConditionFalse {
		t.Errorf("expected status %q, got %q", metav1.ConditionFalse, condition.Status)
	}
	if !strings.Contains(condition.Message, ogNamespace) {
		t.Errorf("expected message to mention namespace %q, got %q", ogNamespace, condition.Message)
	}
}

// Test_ensureOperatorGroup_NoCondition_WhenSingleGroupExists verifies that no
// condition is added when exactly one OperatorGroup already exists (the
// no-op path).
func Test_ensureOperatorGroup_NoCondition_WhenSingleGroupExists(t *testing.T) {
	s := scheme.Scheme
	_ = olmv1.AddToScheme(s)
	_ = operatorsv1.AddToScheme(s)

	ogNamespace := "test-namespace-single-og"
	existingOG := &olmv1.OperatorGroup{ObjectMeta: metav1.ObjectMeta{Name: "og-existing", Namespace: ogNamespace}}

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(existingOG).Build()
	reconciler := &MultiClusterHubReconciler{
		Client: fakeClient,
		Scheme: s,
		Log:    clog.Log.WithName("test"),
	}

	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Name: "test-mch", Namespace: ogNamespace},
	}
	desiredOG := &olmv1.OperatorGroup{ObjectMeta: metav1.ObjectMeta{Name: "desired-og", Namespace: ogNamespace}}

	result, err := reconciler.ensureOperatorGroup(mch, desiredOG)
	if err != nil {
		t.Fatalf("ensureOperatorGroup() unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("ensureOperatorGroup() expected empty result, got: %+v", result)
	}

	if condition := GetHubCondition(mch.Status, operatorsv1.Progressing); condition != nil {
		t.Errorf("expected no Progressing condition, got: %+v", condition)
	}
}

// Test_ensureMultiClusterEngineCR_NoMatchError_SetsWaitingCondition verifies
// that when the MultiClusterEngine CRD isn't yet registered (NoKindMatchError),
// a Progressing/WaitingForMCEReason condition is recorded instead of only
// being logged as a WARNING.
func Test_ensureMultiClusterEngineCR_NoMatchError_SetsWaitingCondition(t *testing.T) {
	noMatchErr := &apimeta.NoKindMatchError{
		GroupKind: schema.GroupKind{Group: "multicluster.openshift.io", Kind: "MultiClusterEngine"},
	}

	noMatchClient := &listErrorClient{
		Client:  fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
		listErr: noMatchErr,
	}

	reconciler := &MultiClusterHubReconciler{
		Client: noMatchClient,
		Scheme: scheme.Scheme,
		Log:    clog.Log.WithName("test"),
	}

	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mch-nomatch-condition",
			Namespace: "test-namespace",
		},
	}

	_, err := reconciler.ensureMultiClusterEngineCR(context.Background(), mch)
	if err != nil {
		t.Fatalf("ensureMultiClusterEngineCR() expected nil error for NoMatchError, got: %v", err)
	}

	condition := GetHubCondition(mch.Status, operatorsv1.Progressing)
	if condition == nil {
		t.Fatal("expected Progressing condition to be set")
	}
	if condition.Reason != WaitingForMCEReason {
		t.Errorf("expected reason %q, got %q", WaitingForMCEReason, condition.Reason)
	}
	if condition.Message != "Waiting for MultiClusterEngine CRD to become available" {
		t.Errorf("unexpected condition message: %q", condition.Message)
	}
}

// Test_waitForMCEReady_SetsWaitingCondition_NoMCE verifies that when no
// managed MultiClusterEngine exists yet, a Progressing/WaitingForMCEReason
// condition is recorded and the request is requeued.
func Test_waitForMCEReady_SetsWaitingCondition_NoMCE(t *testing.T) {
	registerScheme()
	r := newTestReconciler()
	mch := resources.EmptyMCH()
	mch.Name = "test-mch-mce-not-present"

	result, err := r.waitForMCEReady(context.Background(), &mch)
	if err != nil {
		t.Fatalf("waitForMCEReady() unexpected error: %v", err)
	}
	if !result.Requeue {
		t.Errorf("expected Requeue=true, got %+v", result)
	}

	condition := GetHubCondition(mch.Status, operatorsv1.Progressing)
	if condition == nil {
		t.Fatal("expected Progressing condition to be set")
	}
	if condition.Reason != WaitingForMCEReason {
		t.Errorf("expected reason %q, got %q", WaitingForMCEReason, condition.Reason)
	}
	if condition.Message != "Waiting for MultiClusterEngine to be created" {
		t.Errorf("unexpected condition message: %q", condition.Message)
	}
}

// Test_waitForMCEReady_SetsWaitingCondition_NoVersion verifies that once MCE
// exists but hasn't reported a CurrentVersion yet, a
// Progressing/WaitingForMCEReason condition naming the MCE is recorded.
func Test_waitForMCEReady_SetsWaitingCondition_NoVersion(t *testing.T) {
	registerScheme()

	mce := resources.EmptyMCE()
	mce.Name = "test-mce-no-version"
	mce.Labels = map[string]string{multiclusterengineutils.MCEManagedByLabel: "true"}
	// Status.CurrentVersion intentionally left empty.

	r := newTestReconciler(&mce)
	// waitForMCEReady short-circuits past this point under UNIT_TEST=true;
	// pin it explicitly so this path is exercised deterministically.
	t.Setenv(utils.UnitTestEnvVar, "false")

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-mce-no-version"

	result, err := r.waitForMCEReady(context.Background(), &mch)
	if err != nil {
		t.Fatalf("waitForMCEReady() unexpected error: %v", err)
	}
	if result.RequeueAfter != resyncPeriod {
		t.Errorf("expected RequeueAfter=%v, got %+v", resyncPeriod, result)
	}

	condition := GetHubCondition(mch.Status, operatorsv1.Progressing)
	if condition == nil {
		t.Fatal("expected Progressing condition to be set")
	}
	if condition.Reason != WaitingForMCEReason {
		t.Errorf("expected reason %q, got %q", WaitingForMCEReason, condition.Reason)
	}
	if !strings.Contains(condition.Message, mce.GetName()) {
		t.Errorf("expected message to mention MCE name %q, got %q", mce.GetName(), condition.Message)
	}
}

// Test_waitForMCEReady_SetsWaitingCondition_VersionTooLow verifies that when
// MCE reports a version that doesn't satisfy the minimum requirement, a
// Progressing/WaitingForMCEReason condition mentioning the upgrade wait is
// recorded.
func Test_waitForMCEReady_SetsWaitingCondition_VersionTooLow(t *testing.T) {
	registerScheme()

	mce := resources.EmptyMCE()
	mce.Name = "test-mce-low-version"
	mce.Labels = map[string]string{multiclusterengineutils.MCEManagedByLabel: "true"}
	mce.Status.CurrentVersion = "1.0.0"

	r := newTestReconciler(&mce)
	t.Setenv(utils.UnitTestEnvVar, "false")
	// Pin the non-community version requirement (5.0.0) explicitly so this
	// test doesn't depend on utils.IsCommunityMode()'s default.
	t.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-mce-low-version"

	result, err := r.waitForMCEReady(context.Background(), &mch)
	if err != nil {
		t.Fatalf("waitForMCEReady() unexpected error: %v", err)
	}
	if result.RequeueAfter != resyncPeriod {
		t.Errorf("expected RequeueAfter=%v, got %+v", resyncPeriod, result)
	}

	condition := GetHubCondition(mch.Status, operatorsv1.Progressing)
	if condition == nil {
		t.Fatal("expected Progressing condition to be set")
	}
	if condition.Reason != WaitingForMCEReason {
		t.Errorf("expected reason %q, got %q", WaitingForMCEReason, condition.Reason)
	}
	if !strings.Contains(condition.Message, "Waiting for MultiClusterEngine upgrade") {
		t.Errorf("expected message to mention upgrade wait, got %q", condition.Message)
	}
}

// Test_waitForMCEReady_NoCondition_WhenReady verifies that no condition is
// added once MCE reports a version satisfying the minimum requirement.
func Test_waitForMCEReady_NoCondition_WhenReady(t *testing.T) {
	registerScheme()

	mce := resources.EmptyMCE()
	mce.Name = "test-mce-ready"
	mce.Labels = map[string]string{multiclusterengineutils.MCEManagedByLabel: "true"}
	mce.Status.CurrentVersion = "5.0.0"

	r := newTestReconciler(&mce)
	t.Setenv(utils.UnitTestEnvVar, "false")
	// Pin the non-community version requirement (5.0.0) explicitly so this
	// test doesn't depend on utils.IsCommunityMode()'s default.
	t.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-mce-ready"

	result, err := r.waitForMCEReady(context.Background(), &mch)
	if err != nil {
		t.Fatalf("waitForMCEReady() unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got: %+v", result)
	}

	if condition := GetHubCondition(mch.Status, operatorsv1.Progressing); condition != nil {
		t.Errorf("expected no Progressing condition, got: %+v", condition)
	}
}
