// Copyright (c) 2020 Red Hat, Inc.

package controller

import (
	multiclusterhub "github.com/stolostron/multiclusterhub-operator/pkg/controller/multiclusterhub"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, multiclusterhub.Add)
}
