// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controller

import (
	multiclusterhub "github.com/open-cluster-management/multiclusterhub-operator/pkg/controller/multiclusterhub"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, multiclusterhub.Add)
}
