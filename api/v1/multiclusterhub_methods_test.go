package v1

import (
	"reflect"
	"testing"

	"k8s.io/utils/strings/slices"
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

func TestGetDisabledComponents(t *testing.T) {
	tests := []struct {
		name      string
		component string
		want      bool
		want2     int
	}{
		{
			name:      "default disabled components",
			component: ClusterBackup,
			want:      true,
			want2:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disabledComponents, err := GetDefaultDisabledComponents()

			if err != nil {
				t.Errorf("GetDefaultDisabledComponents() = %v, want: %v", err.Error(), nil)
			}

			pass := false
			for _, c := range disabledComponents {
				if c == tt.component {
					pass = true
				}
			}

			if !pass {
				t.Errorf("GetDefaultDisabledComponents() = %v, want: %v", pass, tt.want)
			}

			if len(disabledComponents) != 1 {
				t.Errorf("GetDefaultDisabledComponents() = %v, want: %v", len(disabledComponents), tt.want2)
			}
		})
	}
}

func TestGetDefaultHostedComponents(t *testing.T) {
	tests := []struct {
		name      string
		component string
		want      int
		want2     bool
	}{
		{
			name:      "default hosted components",
			component: MultiClusterEngine,
			want:      1,
			want2:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostedComponents := GetDefaultHostedComponents()

			if len(hostedComponents) != 1 {
				t.Errorf("GetDefaultHostedComponents() = %v, want: %v", len(hostedComponents), tt.want)
			}

			pass := false
			for _, c := range hostedComponents {
				if c == tt.component {
					pass = true
				}
			}

			if !pass {
				t.Errorf("GetDefaultHostedComponents() = %v, want: %v", pass, tt.want2)
			}

		})
	}
}

func TestGetClusterManagementAddonName(t *testing.T) {
	tests := []struct {
		name      string
		component string
		want      string
	}{
		{
			name:      "submariner ClusterManagementAddOn",
			component: SubmarinerAddon,
			want:      "submariner",
		},
		{
			name:      "unknown ClusterManagementAddOn",
			component: "unknown",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetClusterManagementAddonName(tt.component)
			if err != nil && tt.component != "unknown" {
				t.Errorf("GetClusterManagementAddonName(%v) = %v, want: %v", tt.component, err.Error(), tt.want)
			}

			if got != tt.want {
				t.Errorf("GetClusterManagementAddonName(%v) = %v, want: %v", tt.component, got, tt.want)
			}
		})
	}
}

func TestGetLegacyPrometheusKind(t *testing.T) {
	tests := []struct {
		name  string
		kind  string
		want  int
		want2 []string
	}{
		{
			name:  "legacy Prometheus Configuration Kind",
			kind:  "PrometheusRule",
			want:  2,
			want2: LegacyPrometheusKind,
		},
		{
			name:  "legacy Prometheus Configuration Kind",
			kind:  "ServiceMonitor",
			want:  2,
			want2: LegacyPrometheusKind,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLegacyPrometheusKind()
			if len(got) == 0 {
				t.Errorf("GetLegacyPrometheusKind() = %v, want: %v", len(got), tt.want)
			}

			if ok := slices.Contains(got, tt.kind); !ok {
				t.Errorf("GetLegacyPrometheusKind() = %v, want: %v", got, tt.want2)
			}
		})
	}
}

func TestGetPrometheusRulesName(t *testing.T) {
	tests := []struct {
		name      string
		component string
		want      string
	}{
		{
			name:      "console PrometheusRule",
			component: Console,
			want:      MCHPrometheusRules[Console],
		},
		{
			name:      "unknown PrometheusRule",
			component: "unknown",
			want:      MCHPrometheusRules["unknown"],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetPrometheusRulesName(tt.component)
			if err != nil && tt.component != "unknown" {
				t.Errorf("GetPrometheusRulesName(%v) = %v, want: %v", tt.component, err.Error(), tt.want)
			}

			if got != tt.want {
				t.Errorf("GetPrometheusRulesName(%v) = %v, want: %v", tt.component, got, tt.want)
			}
		})
	}
}

func TestGetServiceMonitorName(t *testing.T) {
	tests := []struct {
		name      string
		component string
		want      string
	}{
		{
			name:      "console ServiceMonitor",
			component: Console,
			want:      MCHServiceMonitors[Console],
		},
		{
			name:      "unknown ServiceMonitor",
			component: "unknown",
			want:      MCHServiceMonitors["unknown"],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetServiceMonitorName(tt.component)
			if err != nil && tt.component != "unknown" {
				t.Errorf("GetServiceMonitorName(%v) = %v, want: %v", tt.component, err.Error(), tt.want)
			}

			if got != tt.want {
				t.Errorf("GetServiceMonitorName(%v) = %v, want: %v", tt.component, got, tt.want)
			}
		})
	}
}

func TestGetServiceName(t *testing.T) {
	tests := []struct {
		name      string
		component string
		want      string
	}{
		{
			name:      "multiclusterhub Service",
			component: MCH,
			want:      MCHServices[MCH],
		},
		{
			name:      "unknown Service",
			component: "unknown",
			want:      MCHServices["unknown"],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetServiceName(tt.component)
			if err != nil && tt.component != "unknown" {
				t.Errorf("GetServiceName(%v) = %v, want: %v", tt.component, err.Error(), tt.want)
			}

			if got != tt.want {
				t.Errorf("GetServiceName(%v) = %v, want: %v", tt.component, got, tt.want)
			}
		})
	}
}
