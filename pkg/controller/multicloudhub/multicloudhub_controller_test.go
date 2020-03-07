package multicloudhub

import (
	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"testing"
)

func Test_generatePass(t *testing.T) {
	t.Run("Test length", func(t *testing.T) {
		length := 16
		if got := generatePass(length); len(got) != length {
			t.Errorf("length of generatePass(%d) = %d, want %d", length, len(got), length)
		}
	})

	t.Run("Test randomness", func(t *testing.T) {
		t1 := generatePass(32)
		t2 := generatePass(32)
		if t1 == t2 {
			t.Errorf("generatePass() did not generate a unique password")
		}
	})
}

func Test_checkMultiCloudHubConfig(t *testing.T) {
	mch := &operatorsv1alpha1.MultiCloudHub{
		Spec: operatorsv1alpha1.MultiCloudHubSpec{
			Mongo: operatorsv1alpha1.Mongo{
				Endpoints: "",
			},
		},
	}
	checkMultiCloudHubConfig(mch)

	if mch.Spec.Mongo.Endpoints != "multicloud-mongodb" ||
		mch.Spec.Mongo.ReplicaSet != "rs0" ||
		mch.Spec.Mongo.UserSecret != "mongodb-admin" ||
		mch.Spec.Mongo.CASecret != "multicloud-ca-cert" ||
		mch.Spec.Mongo.TLSSecret != "multicloud-mongodb-client-cert" {
		t.Errorf("checkMultiCloudHubConfig test fail")
	}
}
