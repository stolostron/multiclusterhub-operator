// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"errors"
	"testing"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			wantRequeue: true,
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

// Helper to find condition in status
func FindCondition(conditions []operatorsv1.HubCondition, condType operatorsv1.HubConditionType) *operatorsv1.HubCondition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
