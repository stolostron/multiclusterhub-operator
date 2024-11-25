package controllers

import (
	"context"
	"os"
	"testing"

	v1 "github.com/operator-framework/api/pkg/operators/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	backplanev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBackupNamespace(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		want2 string
		want3 string
	}{
		{
			name:  "basic return values test",
			want:  "v1",
			want2: "Namespace",
			want3: "open-cluster-management-backup",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BackupNamespace()
			if got.APIVersion != tt.want {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
			if got.Kind != tt.want2 {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
			if got.Name != tt.want3 {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBackupNamespaceUnstructured(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		want2 string
		want3 string
	}{
		{
			name:  "basic return values test",
			want:  "v1",
			want2: "Namespace",
			want3: "open-cluster-management-backup",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BackupNamespaceUnstructured()
			if got.GetAPIVersion() != tt.want {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
			if got.GetKind() != tt.want2 {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
			if got.GetName() != tt.want3 {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_cleanupMultiClusterEngine(t *testing.T) {
	tests := []struct {
		name string
		csv  subv1alpha1.ClusterServiceVersion
		ns   corev1.Namespace
		mch  operatorv1.MultiClusterHub
		mce  backplanev1.MultiClusterEngine
		og   v1.OperatorGroup
		sub  subv1alpha1.Subscription
		want bool
	}{
		{
			name: "should cleanup MultiClusterEngine",
			csv: subv1alpha1.ClusterServiceVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multicluster-engine.v2.8.0",
					Namespace: "multicluster-engine",
				},
			},
			ns:  corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "multicluster-engine"}},
			mce: resources.EmptyMCE(),
			mch: resources.EmptyMCH(),
			og:  *multiclusterengine.OperatorGroup(),
			sub: subv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mce-sub",
					Namespace: "multicluster-engine",
					Labels: map[string]string{
						utils.MCEManagedByLabel: "true",
					},
				},
				Status: subv1alpha1.SubscriptionStatus{
					CurrentCSV: "multicluster-engine.v2.8.0",
				},
			},
			want: true,
		},
	}

	registerScheme()
	for _, tt := range tests {
		os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
		defer func() {
			os.Unsetenv("OPERATOR_PACKAGE")
		}()

		t.Run(tt.name, func(t *testing.T) {
			if err := recon.Client.Create(context.TODO(), &tt.mch); err != nil {
				t.Errorf("failed to create MultiClusterHub: %v", err)
			}

			tt.mce.Labels = map[string]string{
				"installer.name":        tt.mch.GetName(),
				"installer.namespace":   tt.mch.GetNamespace(),
				utils.MCEManagedByLabel: "true",
			}

			tt.sub.Labels = map[string]string{
				"installer.name":        tt.mch.GetName(),
				"installer.namespace":   tt.mch.GetNamespace(),
				utils.MCEManagedByLabel: "true",
			}

			if err := recon.Client.Create(context.TODO(), &tt.ns); err != nil {
				t.Errorf("failed to create Namespace: %v", err)
			}

			if err := recon.Client.Create(context.TODO(), &tt.sub); err != nil {
				t.Errorf("failed to create Subscription: %v", err)
			}

			if err := recon.Client.Create(context.TODO(), &tt.csv); err != nil {
				t.Errorf("failed to create CSV: %v", err)
			}

			if err := recon.Client.Create(context.TODO(), &tt.mce); err != nil {
				t.Errorf("failed to create MultiClusterEngine: %v", err)
			}

			if err := recon.Client.Create(context.TODO(), &tt.og); err != nil {
				t.Errorf("failed to create OperatorGroup: %v", err)
			}

			// If MCE exists the first time it will return an error.
			if err := recon.cleanupMultiClusterEngine(log, &tt.mch); err == nil {
				t.Errorf("failed to cleanup MultiClusterEngine: %v", err)
			}

			// If the CSV exists, it will return an error
			if err := recon.cleanupMultiClusterEngine(log, &tt.mch); err == nil {
				t.Errorf("failed to cleanup MultiClusterEngine: %v", err)
			}

			// If the Subscription exists, it will return an error
			if err := recon.cleanupMultiClusterEngine(log, &tt.mch); err == nil {
				t.Errorf("failed to cleanup MultiClusterEngine: %v", err)
			}

			// If the OperatorGroup exists, it will return an error
			if err := recon.cleanupMultiClusterEngine(log, &tt.mch); err != nil {
				t.Errorf("failed to cleanup MultiClusterEngine: %v", err)
			}
		})
	}
}
