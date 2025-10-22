// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"os"
	"reflect"
	"testing"

	mchv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("utility functions", func() {
	Context("CertManagerNS function", func() {
		It("returns the cert manager namespace when provided in the spec", func() {
			mch := resources.EmptyMCH()
			mch.Spec.SeparateCertificateManagement = true
			Expect(CertManagerNS(&mch)).To(Equal(CertManagerNamespace))
		})

		It("returns the mch namespace when certmanager ns is not provided", func() {
			mch := resources.EmptyMCH()
			Expect(CertManagerNS(&mch)).To(Equal(resources.MulticlusterhubNamespace))
		})
	})

	Context("using label function", func() {
		It("adds the installer labels to the object", func() {
			By("creating an unstructured object with a label")
			u := &unstructured.Unstructured{}
			u.SetLabels(map[string]string{"mylabel": "myvalue"})

			By("adding the installer labels")
			name := "installer-name"
			ns := "installer-ns"
			AddInstallerLabel(u, name, ns)
			l := u.GetLabels()
			s, ok := l["installer.name"]
			Expect(ok).To(BeTrue())
			Expect(s).To(Equal(name))
			s, ok = l["installer.namespace"]
			Expect(ok).To(BeTrue())
			Expect(s).To(Equal(ns))

			By("ensuring existing labels are still present")
			s, ok = l["mylabel"]
			Expect(ok).To(BeTrue())
			Expect(s).To(Equal("myvalue"))
		})

		It("adds labels to a deployment", func() {
			By("creating a deployment with no labels")
			d := &appsv1.Deployment{}

			By("adding a label to the deployment")
			l := map[string]string{"mylabel-1": "myvalue-1"}
			Expect(AddDeploymentLabels(d, l)).To(BeTrue())
			s, ok := d.Labels["mylabel-1"]
			Expect(ok).To(BeTrue())
			Expect(s).To(Equal("myvalue-1"))

			By("adding the same label to the deployment")
			Expect(AddDeploymentLabels(d, l)).To(BeFalse())
			s, ok = d.Labels["mylabel-1"]
			Expect(ok).To(BeTrue())
			Expect(s).To(Equal("myvalue-1"))

			By("adding a second label to the deployment")
			l = map[string]string{"mylabel-2": "myvalue-2"}
			Expect(AddDeploymentLabels(d, l)).To(BeTrue())
			s, ok = d.Labels["mylabel-2"]
			Expect(ok).To(BeTrue())
			Expect(s).To(Equal("myvalue-2"))

			By("updating the second label on the deployment")
			l = map[string]string{"mylabel-2": "myvalue-2a"}
			Expect(AddDeploymentLabels(d, l)).To(BeTrue())
			s, ok = d.Labels["mylabel-2"]
			Expect(ok).To(BeTrue())
			Expect(s).To(Equal("myvalue-2a"))
		})

		It("adds labels to the pods in a deployment", func() {
			By("creating a deployment with no pod labels")
			d := &appsv1.Deployment{}

			By("adding a label to the deployment pods")
			l := map[string]string{"mylabel-1": "myvalue-1"}
			Expect(AddPodLabels(d, l)).To(BeTrue())
			s, ok := d.Spec.Template.Labels["mylabel-1"]
			Expect(ok).To(BeTrue())
			Expect(s).To(Equal("myvalue-1"))

			By("adding the same label to the deployment pods")
			Expect(AddPodLabels(d, l)).To(BeFalse())
			s, ok = d.Spec.Template.Labels["mylabel-1"]
			Expect(ok).To(BeTrue())
			Expect(s).To(Equal("myvalue-1"))

			By("adding a second label to the deployment pods")
			l = map[string]string{"mylabel-2": "myvalue-2"}
			Expect(AddPodLabels(d, l)).To(BeTrue())
			s, ok = d.Spec.Template.Labels["mylabel-2"]
			Expect(ok).To(BeTrue())
			Expect(s).To(Equal("myvalue-2"))

			By("updating the second label on the deployment pods")
			l = map[string]string{"mylabel-2": "myvalue-2a"}
			Expect(AddPodLabels(d, l)).To(BeTrue())
			s, ok = d.Spec.Template.Labels["mylabel-2"]
			Expect(ok).To(BeTrue())
			Expect(s).To(Equal("myvalue-2a"))
		})
	})

	Context("CoreToUnstructured function", func() {
		It("converts a valid object to unstructured", func() {
			By("creating a valid object")
			d := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			}

			By("converting the object to unstructured")
			u, err := CoreToUnstructured(d)
			Expect(err).To(BeNil())
			Expect(u).ToNot(BeNil())
		})
	})

	Context("AvailabilityConfigIsValid function", func() {
		It("returns true for a valid config", func() {
			c := mchv1.HAHigh
			Expect(mchv1.AvailabilityConfigIsValid(c)).To(BeTrue())
		})
		It("returns false for an invalid config", func() {
			c := mchv1.AvailabilityType("invalid")
			Expect(mchv1.AvailabilityConfigIsValid(c)).To(BeFalse())
		})
	})

	Context("functions to get values based on MCH", func() {
		It("gets test images", func() {
			images := GetTestImages()
			Expect(len(images)).To(BeNumerically(">", 0))
		})
		It("gets deployments", func() {
			os.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
			mch := resources.EmptyMCH()
			mch.Enable(mchv1.ClusterBackup)
			d := GetDeployments(&mch)
			Expect(len(d)).To(Equal(2))
		})
		It("gets deployments in Community Mode", func() {
			os.Setenv("OPERATOR_PACKAGE", "stolostron")
			mch := resources.EmptyMCH()
			d := GetDeployments(&mch)
			Expect(len(d)).To(Equal(0))
		})
		It("gets custom resources", func() {
			mch := resources.EmptyMCH()
			cr := GetCustomResources(&mch)
			Expect(len(cr)).To(Equal(3))
		})
		It("gets deployments for status with mcho-repo disabled", func() {
			mch := resources.EmptyMCH()
			d := GetDeploymentsForStatus(&mch, true, false)
			Expect(len(d)).To(Equal(0))
		})
		It("gets deployments for status with insights enabled", func() {
			mch := resources.EmptyMCH()
			mch.Enable("insights")
			d := GetDeploymentsForStatus(&mch, true, false)
			Expect(len(d)).To(Equal(2))
		})
		It("gets deployments for status with cluster-lifecycle enabled", func() {
			mch := resources.EmptyMCH()
			mch.Enable(mchv1.ClusterLifecycle)
			d := GetDeploymentsForStatus(&mch, true, false)
			Expect(len(d)).To(Equal(1))
		})
		It("gets deployments for status with cluster-backkup enabled", func() {
			mch := resources.EmptyMCH()
			mch.Enable(mchv1.ClusterBackup)
		})
		It("gets deployments for status with grc enabled", func() {
			mch := resources.EmptyMCH()
			mch.Enable(mchv1.GRC)
			d := GetDeploymentsForStatus(&mch, true, false)
			Expect(len(d)).To(Equal(2))
		})
		It("gets deployments for status with app-lifecycle enabled", func() {
			mch := resources.EmptyMCH()
			mch.Enable(mchv1.Appsub)
			d := GetDeploymentsForStatus(&mch, true, false)
			Expect(len(d)).To(Equal(5))
		})
		It("gets deployments for status with console enabled", func() {
			mch := resources.EmptyMCH()
			mch.Enable(mchv1.Console)
			d := GetDeploymentsForStatus(&mch, true, false)
			Expect(len(d)).To(Equal(1))
		})
		It("gets deployments for status with observability enabled", func() {
			mch := resources.EmptyMCH()
			mch.Enable(mchv1.MultiClusterObservability)
			d := GetDeploymentsForStatus(&mch, true, false)
			Expect(len(d)).To(Equal(1))
		})
		It("gets deployments for status with volsync enabled", func() {
			mch := resources.EmptyMCH()
			mch.Enable(mchv1.Volsync)
			d := GetDeploymentsForStatus(&mch, true, false)
			Expect(len(d)).To(Equal(1))
		})
		It("gets deployments for status with cluster-permission enabled", func() {
			mch := resources.EmptyMCH()
			mch.Enable(mchv1.ClusterPermission)
			d := GetDeploymentsForStatus(&mch, true, false)
			Expect(len(d)).To(Equal(1))
		})
		It("Sets Default Component values", func() {
			mch := resources.EmptyMCH()
			updated, err := SetDefaultComponents(&mch)
			Expect(updated).To(Equal(true))
			Expect(err).To(BeNil())
		})
		It("Sets Default Component values", func() {
			mch := resources.EmptyMCH()
			updated, err := SetDefaultComponents(&mch)
			Expect(updated).To(Equal(true))
			Expect(err).To(BeNil())
		})
		It("gets custom resources for status with MCE disabled", func() {
			mch := resources.EmptyMCH()
			cr := GetCustomResourcesForStatus(&mch)
			Expect(len(cr)).To(Equal(0))
		})
		It("gets custom resources for status with MCE enabled", func() {
			mch := resources.EmptyMCH()
			mch.Enable(mchv1.MultiClusterEngine)
			cr := GetCustomResourcesForStatus(&mch)
			Expect(len(cr)).To(Equal(3))
		})
		It("gets the default toleration", func() {
			mch := resources.EmptyMCH()
			t := GetTolerations(&mch)
			Expect(len(t)).To(Equal(1))
			Expect(string(t[0].Effect)).To(Equal("NoSchedule"))
			Expect(string(t[0].Key)).To(Equal("node-role.kubernetes.io/infra"))
			Expect(string(t[0].Operator)).To(Equal("Exists"))
		})
		It("checks if a string is in a slice of strings", func() {
			s := []string{"alpha", "beta", "gamma"}
			Expect(Contains(s, "beta")).To(BeTrue())
			Expect(Contains(s, "delta")).To(BeFalse())
		})
		It("removes a string from a slice of strings", func() {
			s := []string{"alpha", "beta", "gamma"}
			s = RemoveString(s, "beta")
			Expect(Contains(s, "beta")).To(BeFalse())
			Expect(len(s)).To(Equal(2))
			s = RemoveString(s, "delta")
			Expect(len(s)).To(Equal(2))
		})
	})
})

func TestContainsPullSecret(t *testing.T) {
	superset := []corev1.LocalObjectReference{{Name: "foo"}, {Name: "bar"}}
	type args struct {
		pullSecrets []corev1.LocalObjectReference
		ps          corev1.LocalObjectReference
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Contains pull secret",
			args{
				pullSecrets: superset,
				ps:          corev1.LocalObjectReference{Name: "foo"},
			},
			true,
		},
		{
			"Does not contain pull secret",
			args{
				pullSecrets: superset,
				ps:          corev1.LocalObjectReference{Name: "baz"},
			},
			false,
		},
		{
			"Empty list",
			args{
				pullSecrets: []corev1.LocalObjectReference{},
				ps:          corev1.LocalObjectReference{Name: "baz"},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsPullSecret(tt.args.pullSecrets, tt.args.ps); got != tt.want {
				t.Errorf("ContainsPullSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsMap(t *testing.T) {
	superset := map[string]string{
		"hello":     "world",
		"goodnight": "moon",
		"yip":       "yip",
	}
	type args struct {
		all      map[string]string
		expected map[string]string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Superset",
			args{
				all:      superset,
				expected: map[string]string{"hello": "world", "yip": "yip"},
			},
			true,
		},
		{
			"Partial overlap",
			args{
				all:      superset,
				expected: map[string]string{"hello": "world", "greetings": "traveler"},
			},
			false,
		},
		{
			"Empty superset",
			args{
				all:      map[string]string{},
				expected: map[string]string{"yip": "yip"},
			},
			false,
		},
		{
			"Empty subset",
			args{
				all:      superset,
				expected: map[string]string{},
			},
			true,
		},
		{
			"Same keys, different values",
			args{
				all:      superset,
				expected: map[string]string{"hello": "moon", "yip": "yip"},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsMap(tt.args.all, tt.args.expected); got != tt.want {
				t.Errorf("ContainsMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMchIsValid(t *testing.T) {
	validMCH := &mchv1.MultiClusterHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiClusterHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: mchv1.MultiClusterHubSpec{
			ImagePullSecret: "test",
			Ingress: &mchv1.IngressSpec{
				SSLCiphers: []string{"foo", "bar", "baz"},
			},
			AvailabilityConfig: mchv1.HAHigh,
		},
	}

	type args struct {
		m *mchv1.MultiClusterHub
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Valid MCH",
			args{validMCH},
			true,
		},
		{
			"Empty object",
			args{&mchv1.MultiClusterHub{}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MchIsValid(tt.args.m); got != tt.want {
				t.Errorf("MchIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDistributePods(t *testing.T) {
	t.Run("Returns pod affinity", func(t *testing.T) {
		if got := DistributePods("app", "testapp"); reflect.TypeOf(got) != reflect.TypeOf((*corev1.Affinity)(nil)) {
			t.Errorf("DistributePods() did not return an affinity type")
		}
	})
}

func TestGetImagePullPolicy(t *testing.T) {
	noPullPolicyMCH := &mchv1.MultiClusterHub{}
	pullPolicyMCH := &mchv1.MultiClusterHub{
		Spec: mchv1.MultiClusterHubSpec{
			Overrides: &mchv1.Overrides{ImagePullPolicy: corev1.PullIfNotPresent},
		},
	}

	t.Run("No pull policy set", func(t *testing.T) {
		want := corev1.PullIfNotPresent
		if got := GetImagePullPolicy(noPullPolicyMCH); got != want {
			t.Errorf("GetImagePullPolicy() = %v, want %v", got, want)
		}
	})
	t.Run("Pull policy set", func(t *testing.T) {
		want := corev1.PullIfNotPresent
		if got := GetImagePullPolicy(pullPolicyMCH); got != want {
			t.Errorf("GetImagePullPolicy() = %v, want %v", got, want)
		}
	})
}

func TestDefaultReplicaCount(t *testing.T) {
	mchDefault := &mchv1.MultiClusterHub{}
	mchNonHA := &mchv1.MultiClusterHub{
		Spec: mchv1.MultiClusterHubSpec{
			AvailabilityConfig: mchv1.HABasic,
		},
	}
	mchHA := &mchv1.MultiClusterHub{
		Spec: mchv1.MultiClusterHubSpec{
			AvailabilityConfig: mchv1.HAHigh,
		},
	}

	t.Run("HA (by default)", func(t *testing.T) {
		if got := DefaultReplicaCount(mchDefault); got != 2 {
			t.Errorf("DefaultReplicaCount() = %v, want %v", got, 2)
		}
	})
	t.Run("Non-HA", func(t *testing.T) {
		if got := DefaultReplicaCount(mchNonHA); got != 1 {
			t.Errorf("DefaultReplicaCount() = %v, want %v", got, 1)
		}
	})
	t.Run("HA-mode replicas", func(t *testing.T) {
		if got := DefaultReplicaCount(mchHA); got <= 1 {
			t.Errorf("DefaultReplicaCount() = %v, but should return multiple replicas", got)
		}
	})
}

func TestFormatSSLCiphers(t *testing.T) {
	type args struct {
		ciphers []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"Default cipher list",
			args{[]string{"ECDHE-ECDSA-AES256-GCM-SHA384", "ECDHE-RSA-AES256-GCM-SHA384"}},
			"ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384",
		},
		{"Empty slice", args{[]string{}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatSSLCiphers(tt.args.ciphers); got != tt.want {
				t.Errorf("FormatSSLCiphers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrackedNamespaces(t *testing.T) {
	tests := []struct {
		name string
		mch  *mchv1.MultiClusterHub
		want []string
	}{
		{
			name: "Watching only in same namespace",
			mch:  &mchv1.MultiClusterHub{ObjectMeta: metav1.ObjectMeta{Namespace: "test"}},
			want: []string{"test"},
		},
		{
			name: "Watching current and cert-manager namespace",
			mch: &mchv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
				Spec: mchv1.MultiClusterHubSpec{
					SeparateCertificateManagement: true,
				},
			},
			want: []string{"test", CertManagerNamespace},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TrackedNamespaces(tt.mch); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TrackedNamespaces() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_deduplicate(t *testing.T) {
	tests := []struct {
		name string
		have []mchv1.ComponentConfig
		want []mchv1.ComponentConfig
	}{
		{
			name: "unique components",
			have: []mchv1.ComponentConfig{
				{Name: "component1", Enabled: true},
				{Name: "component2", Enabled: true},
			},
			want: []mchv1.ComponentConfig{
				{Name: "component1", Enabled: true},
				{Name: "component2", Enabled: true},
			},
		},
		{
			name: "duplicate components",
			have: []mchv1.ComponentConfig{
				{Name: "component1", Enabled: false},
				{Name: "component2", Enabled: true},
				{Name: "component1", Enabled: true},
			},
			want: []mchv1.ComponentConfig{
				{Name: "component1", Enabled: true},
				{Name: "component2", Enabled: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deduplicate(tt.have); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("deduplicate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddInstallerLabels(t *testing.T) {
	labels := map[string]string{
		"testlabel": "testvalue",
	}
	name := "testname"
	ns := "testnamespace"

	labels = AddInstallerLabels(labels, name, ns)

	tests := map[string]string{
		"testlabel":           "testvalue",
		"installer.name":      name,
		"installer.namespace": ns,
	}

	for key, value := range tests {
		if v, ok := labels[key]; !ok {
			t.Errorf("AddInstallerLabels() missing label %q", key)
		} else if v != value {
			t.Errorf("AddInstallerLabels() label %q not %q, found %q", key, value, v)
		}
	}
}

func TestGetMCEComponents(t *testing.T) {
	mch := &mchv1.MultiClusterHub{}
	mch.Spec.DisableHubSelfManagement = false
	config := GetMCEComponents(mch)

	found := false
	for _, c := range config {
		if c.Name != mchv1.MCELocalCluster {
			continue
		}
		found = true
		if !c.Enabled {
			t.Errorf("GetMCEComponents() with DisableHubSelfManagement=false, expected 'local-cluster' to be enabled")
		}
	}
	if !found {
		t.Errorf("GetMCEComponents() with DisableHubSelfManagement=false, expected 'local-cluster' to be present")
	}

	mch.Spec.DisableHubSelfManagement = true
	config = GetMCEComponents(mch)

	found = false
	for _, c := range config {
		if c.Name != mchv1.MCELocalCluster {
			continue
		}
		found = true
		if c.Enabled {
			t.Errorf("GetMCEComponents() with DisableHubSelfManagement=true, expected 'local-cluster' to be disabled")
		}
	}
	if !found {
		t.Errorf("GetMCEComponents() with DisableHubSelfManagement=true, expected 'local-cluster' to be present")
	}
}

func TestUpdateMCEOverrides(t *testing.T) {
	mch := &mchv1.MultiClusterHub{}
	mch.Spec.DisableHubSelfManagement = false
	mce := &mcev1.MultiClusterEngine{}

	UpdateMCEOverrides(mce, mch)

	found := false
	for _, c := range mce.Spec.Overrides.Components {
		if c.Name != mchv1.MCELocalCluster {
			continue
		}
		found = true
		if !c.Enabled {
			t.Errorf("UpdateMCEOverrides() with DisableHubSelfManagement=false, expected 'local-cluster' to be enabled")
		}
	}
	if !found {
		t.Errorf("UpdateMCEOverrides() with DisableHubSelfManagement=false, expected 'local-cluster' to be present")
	}

	mch = &mchv1.MultiClusterHub{}
	mch.Spec.DisableHubSelfManagement = true
	mce = &mcev1.MultiClusterEngine{}

	if mce.Spec.Overrides == nil {
		// Overrides.Components is empty, so local-cluster is disabled
		return
	}
	for _, c := range mce.Spec.Overrides.Components {
		if c.Name != mchv1.MCELocalCluster {
			continue
		}
		if c.Enabled {
			t.Errorf("UpdateMCEOverrides() with DisableHubSelfManagement=true, expected 'local-cluster' to be disabled")
		}
	}
	// Ok if local-cluster not found
}

func Test_GetDeploymentsForStatus(t *testing.T) {
	tests := []struct {
		name       string
		mch        mchv1.MultiClusterHub
		stsEnabled bool
		want       int
	}{
		{
			name:       "should get deployment status for MCH components",
			mch:        resources.EmptyMCH(),
			stsEnabled: false,
			want:       19,
		},
		{
			name: "should get deployment status for MCH components with STS enabled",
			mch: mchv1.MultiClusterHub{
				Spec: mchv1.MultiClusterHubSpec{
					Overrides: &mchv1.Overrides{
						Components: []mchv1.ComponentConfig{
							{
								Name:    mchv1.ClusterBackup,
								Enabled: true,
							},
						},
					},
				},
			},
			stsEnabled: true,
			want:       20,
		},
		{
			name: "should get deployment status for MCH components with STS disabled",
			mch: mchv1.MultiClusterHub{
				Spec: mchv1.MultiClusterHubSpec{
					Overrides: &mchv1.Overrides{
						Components: []mchv1.ComponentConfig{
							{
								Name:    mchv1.ClusterBackup,
								Enabled: true,
							},
						},
					},
				},
			},
			stsEnabled: false,
			want:       21,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := SetDefaultComponents(&tt.mch); err != nil {
				t.Errorf("failed to set default components: %v", err)
			}

			if deployments := GetDeploymentsForStatus(&tt.mch, true, tt.stsEnabled); len(deployments) != tt.want {
				t.Errorf("expected %v, got %v", len(deployments), tt.want)
			}
		})
	}
}
