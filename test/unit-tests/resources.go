// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package resources

import (
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MulticlusterhubName      = "test-mch"
	MulticlusterhubNamespace = "open-cluster-management"
	JobName                  = "test-job"
)

var (
	OCMNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: MulticlusterhubNamespace,
		},
	}

	EmptyMCH = &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MulticlusterhubName,
			Namespace: MulticlusterhubNamespace,
		},
		Spec: operatorsv1.MultiClusterHubSpec{},
	}
)
