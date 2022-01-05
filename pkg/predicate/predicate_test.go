// Copyright (c) 2020 Red Hat, Inc.

// Package predicate defines custom predicates used to filter event triggers
package predicate

import (
	"testing"

	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var (
	pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "biz", Name: "baz"},
	}
	labeledPod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "biz",
			Name:      "baz",
			Labels: map[string]string{
				"installer.name":      "foo",
				"installer.namespace": "bar",
			},
		},
	}
	createEvent = func(p *corev1.Pod) event.CreateEvent {
		return event.CreateEvent{
			Object: p,
			Meta:   p.GetObjectMeta(),
		}
	}
	updateEvent = func(p *corev1.Pod) event.UpdateEvent {
		return event.UpdateEvent{
			ObjectOld: p,
			MetaOld:   p.GetObjectMeta(),
			ObjectNew: p,
			MetaNew:   p.GetObjectMeta(),
		}
	}
	deleteEvent = func(p *corev1.Pod) event.DeleteEvent {
		return event.DeleteEvent{
			Object: p,
			Meta:   p.GetObjectMeta(),
		}
	}
	genericEvent = func(p *corev1.Pod) event.GenericEvent {
		return event.GenericEvent{
			Object: p,
			Meta:   p.GetObjectMeta(),
		}
	}
)

func TestDeletePredicate(t *testing.T) {
	pred := DeletePredicate{}

	t.Run("Create event", func(t *testing.T) {
		want := false
		if got := pred.Create(createEvent(labeledPod)); got != want {
			t.Errorf("DeletePredicate.Create() = %v, want %v", got, want)
		}
	})

	t.Run("Update event", func(t *testing.T) {
		want := false
		if got := pred.Update(updateEvent(labeledPod)); got != want {
			t.Errorf("DeletePredicate.Update() = %v, want %v", got, want)
		}
	})

	t.Run("Generic event", func(t *testing.T) {
		want := false
		if got := pred.Generic(genericEvent(labeledPod)); got != want {
			t.Errorf("DeletePredicate.Generic() = %v, want %v", got, want)
		}
	})

	t.Run("Delete without labels", func(t *testing.T) {
		want := false
		if got := pred.Delete(deleteEvent(pod)); got != want {
			t.Errorf("DeletePredicate.Delete() = %v, want %v", got, want)
		}
	})

	t.Run("Delete with labels", func(t *testing.T) {
		want := true
		if got := pred.Delete(deleteEvent(labeledPod)); got != want {
			t.Errorf("DeletePredicate.Delete() = %v, want %v", got, want)
		}
	})
}

func TestGenerationChangedPredicate(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "biz", Name: "baz"},
	}
	oldAnnotatedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "biz",
			Name:        "baz",
			Annotations: map[string]string{utils.AnnotationImageRepo: "foo"},
		},
	}
	newAnnotatedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "biz",
			Name:        "baz",
			Annotations: map[string]string{utils.AnnotationImageRepo: "bar"},
		},
	}
	pred := GenerationChangedPredicate{}

	t.Run("Update event - no annotations", func(t *testing.T) {
		e := event.UpdateEvent{
			ObjectOld: pod,
			MetaOld:   pod.GetObjectMeta(),
			ObjectNew: pod,
			MetaNew:   pod.GetObjectMeta(),
		}
		want := false
		if got := pred.Update(e); got != want {
			t.Errorf("GenerationChangedPredicate.Update() = %v, want %v", got, want)
		}
	})

	t.Run("Update event - annotations changed", func(t *testing.T) {
		e := event.UpdateEvent{
			ObjectOld: oldAnnotatedPod,
			MetaOld:   oldAnnotatedPod.GetObjectMeta(),
			ObjectNew: newAnnotatedPod,
			MetaNew:   newAnnotatedPod.GetObjectMeta(),
		}
		want := true
		if got := pred.Update(e); got != want {
			t.Errorf("GenerationChangedPredicate.Update() = %v, want %v", got, want)
		}
	})
}

func TestInstallerLabelPredicate(t *testing.T) {
	pred := InstallerLabelPredicate{}

	t.Run("Create event", func(t *testing.T) {
		want := false
		if got := pred.Create(createEvent(pod)); got != want {
			t.Errorf("TestInstallerLabelPredicate.Create() = %v, want %v", got, want)
		}
	})
	t.Run("Create event with labels", func(t *testing.T) {
		want := true
		if got := pred.Create(createEvent(labeledPod)); got != want {
			t.Errorf("TestInstallerLabelPredicate.Create() = %v, want %v", got, want)
		}
	})

	t.Run("Update event", func(t *testing.T) {
		want := false
		if got := pred.Update(updateEvent(pod)); got != want {
			t.Errorf("TestInstallerLabelPredicate.Update() = %v, want %v", got, want)
		}
	})
	t.Run("Update event with labels", func(t *testing.T) {
		want := true
		if got := pred.Update(updateEvent(labeledPod)); got != want {
			t.Errorf("TestInstallerLabelPredicate.Update() = %v, want %v", got, want)
		}
	})

	t.Run("Generic event", func(t *testing.T) {
		want := false
		if got := pred.Generic(genericEvent(pod)); got != want {
			t.Errorf("TestInstallerLabelPredicate.Generic() = %v, want %v", got, want)
		}
	})
	t.Run("Generic event with labels", func(t *testing.T) {
		want := true
		if got := pred.Generic(genericEvent(labeledPod)); got != want {
			t.Errorf("TestInstallerLabelPredicate.Generic() = %v, want %v", got, want)
		}
	})

	t.Run("Delete without labels", func(t *testing.T) {
		want := false
		if got := pred.Delete(deleteEvent(pod)); got != want {
			t.Errorf("TestInstallerLabelPredicate.Delete() = %v, want %v", got, want)
		}
	})

	t.Run("Delete with labels", func(t *testing.T) {
		want := true
		if got := pred.Delete(deleteEvent(labeledPod)); got != want {
			t.Errorf("TestInstallerLabelPredicate.Delete() = %v, want %v", got, want)
		}
	})
}
