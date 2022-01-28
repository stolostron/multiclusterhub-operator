// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package resources

import (
	mcev1 "github.com/stolostron/backplane-operator/api/v1alpha1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	MulticlusterhubName      = "test-mch"
	MulticlusterhubNamespace = "open-cluster-management"
	JobName                  = "test-job"
	MultiClusterEngineName   = "multiclusterengine-sample"
)

var (
	MCHLookupKey = types.NamespacedName{Name: MulticlusterhubName, Namespace: MulticlusterhubNamespace}
	MCELookupKey = types.NamespacedName{Name: MultiClusterEngineName}

	OCMNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: MulticlusterhubNamespace,
		},
	}

	EmptyMCH = operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MulticlusterhubName,
			Namespace: MulticlusterhubNamespace,
		},
		Spec: operatorsv1.MultiClusterHubSpec{},
	}

	EmptyMCE = mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name: MultiClusterEngineName,
		},
		Spec: mcev1.MultiClusterEngineSpec{},
	}
)
