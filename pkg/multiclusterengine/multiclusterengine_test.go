package multiclusterengine

import (
	"fmt"
	"reflect"
	"testing"

	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/api/v1"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSubscription(t *testing.T) {
	// 1. No MCE Annotation
	emptyMCH := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec:       operatorsv1.MultiClusterHubSpec{},
	}

	// 2. All MCE Annotations
	mch1 := emptyMCH.DeepCopy()
	mch1.ObjectMeta.Annotations = map[string]string{
		"installer.open-cluster-management.io/mce-subscription-spec": `{"channel": "channel-1.0","installPlanApproval": "Manual","name":
      	"package","source": "catalogsource","sourceNamespace": "catalogsourcenamespace","startingCSV":
      	"csv-1.0"}`,
	}
	// 3. Override Only Channel
	mch2 := emptyMCH.DeepCopy()
	mch2.ObjectMeta.Annotations = map[string]string{
		"installer.open-cluster-management.io/mce-subscription-spec": `{
			"channel": "channel-1.0"
		}`,
	}
	// 3. Override startingCSV and installPlanApproval
	mch3 := emptyMCH.DeepCopy()
	mch3.ObjectMeta.Annotations = map[string]string{
		"installer.open-cluster-management.io/mce-subscription-spec": `{
			"startingCSV": "csv-1.0",
			"installPlanApproval": "Manual"
		}`,
	}
	tests := []struct {
		name string
		MCH  *operatorsv1.MultiClusterHub
		want *subv1alpha1.SubscriptionSpec
	}{
		{
			name: "Empty MCH (No MCE annotations)",
			MCH:  emptyMCH,
			want: &subv1alpha1.SubscriptionSpec{
				Channel:                "stable-2.0",
				InstallPlanApproval:    subv1alpha1.ApprovalAutomatic,
				Package:                "multicluster-engine",
				CatalogSource:          "redhat-operators",
				CatalogSourceNamespace: "openshift-marketplace",
			},
		},
		{
			name: "MCE Annotations set (All fields)",
			MCH:  mch1,
			want: &subv1alpha1.SubscriptionSpec{
				Channel:                "channel-1.0",
				InstallPlanApproval:    subv1alpha1.ApprovalManual,
				Package:                "package",
				CatalogSource:          "catalogsource",
				CatalogSourceNamespace: "catalogsourcenamespace",
				StartingCSV:            "csv-1.0",
			},
		},
		{
			name: "MCE Annotations set (Channel Only)",
			MCH:  mch2,
			want: &subv1alpha1.SubscriptionSpec{
				Channel:                "channel-1.0",
				InstallPlanApproval:    subv1alpha1.ApprovalAutomatic,
				Package:                "multicluster-engine",
				CatalogSource:          "redhat-operators",
				CatalogSourceNamespace: "openshift-marketplace",
			},
		},
		{
			name: "MCE Annotations set (StartingCSV and InstallPlanApproval)",
			MCH:  mch3,
			want: &subv1alpha1.SubscriptionSpec{
				Channel:                "stable-2.0",
				InstallPlanApproval:    subv1alpha1.ApprovalManual,
				Package:                "multicluster-engine",
				CatalogSource:          "redhat-operators",
				CatalogSourceNamespace: "openshift-marketplace",
				StartingCSV:            "csv-1.0",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := Subscription(tt.MCH)
			if !reflect.DeepEqual(sub.Spec, tt.want) {
				fmt.Printf("%+v\n", sub.Spec)
				fmt.Printf("%+v\n", tt.want)
				t.Errorf("Subscription() got = %v, want %v", sub.Spec, tt.want)
			}
		})
	}
}
