package v1

import (
	"reflect"
	"testing"
)

func TestMultiClusterHub_Prune(t *testing.T) {
	tests := []struct {
		name      string
		have      []ComponentConfig
		component string
		want      bool
		want2     []ComponentConfig
	}{
		{
			name: "solo component pruned",
			have: []ComponentConfig{
				{Name: ManagementIngress, Enabled: true},
			},
			component: ManagementIngress,
			want:      true,
			want2:     []ComponentConfig{},
		},
		{
			name: "component pruned",
			have: []ComponentConfig{
				{Name: ClusterLifecycle, Enabled: true},
				{Name: ManagementIngress, Enabled: true},
				{Name: Search, Enabled: true},
			},
			component: ManagementIngress,
			want:      true,
			want2: []ComponentConfig{
				{Name: ClusterLifecycle, Enabled: true},
				{Name: Search, Enabled: true},
			},
		},
		{
			name: "nothing to prune",
			have: []ComponentConfig{
				{Name: ClusterLifecycle, Enabled: true},
				{Name: Search, Enabled: true},
			},
			component: ManagementIngress,
			want:      false,
			want2: []ComponentConfig{
				{Name: ClusterLifecycle, Enabled: true},
				{Name: Search, Enabled: true},
			},
		},
		{
			name:      "nil list",
			have:      nil,
			component: ManagementIngress,
			want:      false,
			want2:     nil,
		},
	}
	for _, tt := range tests {
		mch := &MultiClusterHub{
			Spec: MultiClusterHubSpec{
				Overrides: &Overrides{
					Components: tt.have,
				},
			},
		}
		t.Run(tt.name, func(t *testing.T) {
			got := mch.Prune(tt.component)
			if got != tt.want {
				t.Errorf("MultiClusterHub.Prune() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(mch.Spec.Overrides.Components, tt.want2) {
				t.Errorf("MultiClusterHub.Prune() = %v, want %v", mch.Spec.Overrides.Components, tt.want2)
			}
		})
	}
}
