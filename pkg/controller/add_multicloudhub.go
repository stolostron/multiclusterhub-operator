package controller

import (
	"github.com/open-cluster-management/multicloudhub-operator/pkg/controller/multicloudhub"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, multicloudhub.Add)
}
