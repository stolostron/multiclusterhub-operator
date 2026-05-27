package controllers

import (
	"context"
	"testing"

	backplanev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
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
				"installer.name":                          tt.mch.GetName(),
				"installer.namespace":                     tt.mch.GetNamespace(),
				multiclusterengineutils.MCEManagedByLabel: "true",
			}
			if err := recon.Client.Create(context.TODO(), &tt.mce); err != nil {
				t.Errorf("failed to create MultiClusterEngine: %v", err)
			}

			// If MCE exists the first time it will return an error.
			if err := recon.cleanupMultiClusterEngine(log, &tt.mch); err == nil {
				t.Errorf("failed to cleanup MultiClusterEngine: %v", err)
			}

			if err := recon.cleanupMultiClusterEngine(log, &tt.mch); err != nil {
				t.Errorf("failed to cleanup MultiClusterEngine: %v", err)
			}
		})
	}
}

func Test_cleanupMultiClusterEngine_OLMv1(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-olmv1"
	mce := resources.EmptyMCE()
	mce.Name = "test-mce-olmv1"
	mce.Labels = map[string]string{
		"installer.name":                          mch.GetName(),
		"installer.namespace":                     mch.GetNamespace(),
		multiclusterengineutils.MCEManagedByLabel: "true",
	}

	// Setup client with MCE
	if err := recon.Client.Create(context.TODO(), &mch); err != nil {
		t.Fatalf("failed to create MultiClusterHub: %v", err)
	}
	if err := recon.Client.Create(context.TODO(), &mce); err != nil {
		t.Fatalf("failed to create MultiClusterEngine: %v", err)
	}

	// Set OLM v1 mode
	recon.OLMVersion = "v1"

	// First call should return error (MCE still exists)
	err := recon.cleanupMultiClusterEngine(log, &mch)
	if err == nil {
		t.Error("expected error on first cleanup call, got nil")
	}

	// Second call should succeed (MCE deleted)
	err = recon.cleanupMultiClusterEngine(log, &mch)
	if err != nil {
		t.Errorf("expected no error on second cleanup call, got: %v", err)
	}

	// Reset OLM version
	recon.OLMVersion = ""
}

func Test_cleanupMultiClusterEngine_OLMv0(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-olmv0"
	mce := resources.EmptyMCE()
	mce.Name = "test-mce-olmv0"
	mce.Labels = map[string]string{
		"installer.name":                          mch.GetName(),
		"installer.namespace":                     mch.GetNamespace(),
		multiclusterengineutils.MCEManagedByLabel: "true",
	}

	// Setup client with MCE
	if err := recon.Client.Create(context.TODO(), &mch); err != nil {
		t.Fatalf("failed to create MultiClusterHub: %v", err)
	}
	if err := recon.Client.Create(context.TODO(), &mce); err != nil {
		t.Fatalf("failed to create MultiClusterEngine: %v", err)
	}

	// Set OLM v0 mode
	recon.OLMVersion = "v0"

	// First call should return error (MCE still exists)
	err := recon.cleanupMultiClusterEngine(log, &mch)
	if err == nil {
		t.Error("expected error on first cleanup call, got nil")
	}

	// Second call should succeed (MCE deleted)
	err = recon.cleanupMultiClusterEngine(log, &mch)
	if err != nil {
		t.Errorf("expected no error on second cleanup call, got: %v", err)
	}

	// Reset OLM version
	recon.OLMVersion = ""
}
