package controller

import (
	"github.ibm.com/IBMPrivateCloud/multicloudhub-operator/pkg/controller/multicloudhub"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, multicloudhub.Add)
}
