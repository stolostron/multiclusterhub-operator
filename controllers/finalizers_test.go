package controllers

import (
	"context"
	"testing"

	backplanev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
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
		mch  operatorv1.MultiClusterHub
		mce  backplanev1.MultiClusterEngine
		want bool
	}{
		{
			name: "should cleanup MultiClusterEngine",
			mce:  resources.EmptyMCE(),
			mch:  resources.EmptyMCH(),
			want: true,
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := recon.Client.Create(context.TODO(), &tt.mch); err != nil {
				t.Errorf("failed to create MultiClusterHub: %v", err)
			}

			tt.mce.Labels = map[string]string{
				"installer.name":        tt.mch.GetName(),
				"installer.namespace":   tt.mch.GetNamespace(),
				utils.MCEManagedByLabel: "true",
			}
			if err := recon.Client.Create(context.TODO(), &tt.mce); err != nil {
				t.Errorf("failed to create MultiClusterEngine: %v", err)
			}

			if err := recon.cleanupMultiClusterEngine(log, &tt.mch); err != nil {
				t.Errorf("failed to cleanup MultiClusterEngine: %v", err)
			}
		})
	}
}
