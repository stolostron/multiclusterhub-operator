package controllers

import (
	"testing"
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
