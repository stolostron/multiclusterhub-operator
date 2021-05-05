// Copyright (c) 2020 Red Hat, Inc.

// Package predicate defines custom predicates used to filter event triggers
package predicate

import (
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var log = logf.Log.WithName("predicate")

// GenerationChangedPredicate will skip update events that have no change in the object's metadata.generation field.
// The metadata.generation field of an object is incremented by the API server when writes are made to the spec field of an object.
// This allows a controller to ignore update events where the spec is unchanged, and only the metadata and/or status fields are changed.
// This predicate is customized to not ignore certain annotations significant to the multiclusterhub reconciler.
type GenerationChangedPredicate struct {
	predicate.Funcs
}

// Update implements default UpdateEvent filter for validating generation change
func (GenerationChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.MetaOld == nil {
		log.Error(nil, "Update event has no old metadata", "event", e)
		return false
	}
	if e.ObjectOld == nil {
		log.Error(nil, "Update event has no old runtime object to update", "event", e)
		return false
	}
	if e.ObjectNew == nil {
		log.Error(nil, "Update event has no new runtime object for update", "event", e)
		return false
	}
	if e.MetaNew == nil {
		log.Error(nil, "Update event has no new metadata", "event", e)
		return false
	}

	if !utils.AnnotationsMatch(e.MetaOld.GetAnnotations(), e.MetaNew.GetAnnotations()) {
		log.Info("Metadata annotations have changed")
		return true
	}

	return e.MetaNew.GetGeneration() != e.MetaOld.GetGeneration()
}

// DeletePredicate will only respond to delete events where the object has installer labels
type DeletePredicate struct {
	predicate.Funcs
}

func (DeletePredicate) Create(e event.CreateEvent) bool   { return false }
func (DeletePredicate) Update(e event.UpdateEvent) bool   { return false }
func (DeletePredicate) Generic(e event.GenericEvent) bool { return false }
func (DeletePredicate) Delete(e event.DeleteEvent) bool {
	labels := e.Meta.GetLabels()
	return hasInstallerLabels(labels)
}

// InstallerLabelPredicate will only respond to events where the object has installer labels
type InstallerLabelPredicate struct {
	predicate.Funcs
}

// TODO: Use controller-runtime's 'NewPredicateFuncs' to simplify once available
func (InstallerLabelPredicate) Create(e event.CreateEvent) bool {
	labels := e.Meta.GetLabels()
	return hasInstallerLabels(labels)
}
func (InstallerLabelPredicate) Update(e event.UpdateEvent) bool {
	labels := e.MetaNew.GetLabels()
	return hasInstallerLabels(labels)
}
func (InstallerLabelPredicate) Generic(e event.GenericEvent) bool {
	labels := e.Meta.GetLabels()
	return hasInstallerLabels(labels)
}
func (InstallerLabelPredicate) Delete(e event.DeleteEvent) bool {
	labels := e.Meta.GetLabels()
	return hasInstallerLabels(labels)
}

// hasInstallerLabels checks if the map has installer label keys
func hasInstallerLabels(labels map[string]string) bool {
	_, nameExists := labels["installer.name"]
	_, namespaceExists := labels["installer.namespace"]
	return nameExists && namespaceExists
}
