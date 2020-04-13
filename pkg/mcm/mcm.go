package mcm

import (
	"fmt"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
)

// ImageName used by mcm deployments
const ImageName = "multicloud-manager"

// ImageVersion used by mcm deployments
const ImageVersion = "0.0.1"

// ServiceAccount used by mcm deployments
const ServiceAccount = "hub-sa"

func Image(mch *operatorsv1alpha1.MultiClusterHub) string {
	image := fmt.Sprintf("%s/%s:%s", mch.Spec.ImageRepository, ImageName, ImageVersion)
	if mch.Spec.ImageTagSuffix == "" {
		return image
	}
	return image + "-" + mch.Spec.ImageTagSuffix
}

func defaultLabels(app string) map[string]string {
	return map[string]string{
		"app": app,
	}
}
