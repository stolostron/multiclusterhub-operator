// Copyright (c) 2020 Red Hat, Inc.

// Package predicate defines custom predicates used to filter event triggers
package predicate

import (
	"testing"

	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestDeletePredicate(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "biz", Name: "baz"},
	}
	labeledPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "biz",
			Name:      "baz",
			Labels: map[string]string{
				"installer.name":      "foo",
				"installer.namespace": "bar",
			},
		},
	}
	pred := DeletePredicate{}

	t.Run("Create event", func(t *testing.T) {
		e := event.CreateEvent{
			Object: labeledPod,
			Meta:   labeledPod.GetObjectMeta(),
		}
		want := false
		if got := pred.Create(e); got != want {
			t.Errorf("DeletePredicate.Update() = %v, want %v", got, want)
		}
	})

	t.Run("Update event", func(t *testing.T) {
		e := event.UpdateEvent{
			ObjectOld: labeledPod,
			MetaOld:   labeledPod.GetObjectMeta(),
			ObjectNew: labeledPod,
			MetaNew:   labeledPod.GetObjectMeta(),
		}
		want := false
		if got := pred.Update(e); got != want {
			t.Errorf("DeletePredicate.Update() = %v, want %v", got, want)
		}
	})

	t.Run("Generic event", func(t *testing.T) {
		e := event.GenericEvent{
			Object: labeledPod,
			Meta:   labeledPod.GetObjectMeta(),
		}
		want := false
		if got := pred.Generic(e); got != want {
			t.Errorf("DeletePredicate.Update() = %v, want %v", got, want)
		}
	})

	t.Run("Delete without labels", func(t *testing.T) {
		e := event.DeleteEvent{
			Object: pod,
			Meta:   pod.GetObjectMeta(),
		}
		want := false
		if got := pred.Delete(e); got != want {
			t.Errorf("DeletePredicate.Delete() = %v, want %v", got, want)
		}
	})

	t.Run("Delete with labels", func(t *testing.T) {
		e := event.DeleteEvent{
			Object: labeledPod,
			Meta:   labeledPod.GetObjectMeta(),
		}
		want := true
		if got := pred.Delete(e); got != want {
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
