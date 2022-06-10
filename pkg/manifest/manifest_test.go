// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package manifest

import (
	"testing"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
)

func Test_buildFullImageReference(t *testing.T) {
	mi := ManifestImage{
		ImageKey:     "test_app",
		ImageName:    "test-app",
		ImageVersion: "9.9.9",
		ImageRemote:  "quay.io/stolostron",
		ImageDigest:  "sha256:abc123",
	}
	mch := &operatorsv1.MultiClusterHub{}

	mch1 := mch.DeepCopy()

	mch2 := mch.DeepCopy()
	mch2.SetAnnotations(map[string]string{utils.AnnotationImageRepo: "foo.io/bar"})

	type args struct {
		mch *operatorsv1.MultiClusterHub
		mi  ManifestImage
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Default (sha format)",
			args: args{mch1, mi},
			want: "quay.io/stolostron/test-app@sha256:abc123",
		},
		{
			name: "Custom registry",
			args: args{mch2, mi},
			want: "foo.io/bar/test-app@sha256:abc123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildFullImageReference(tt.args.mch, tt.args.mi); got != tt.want {
				t.Errorf("buildFullImageReference() = %v, want %v", got, tt.want)
			}
		})
	}
}
