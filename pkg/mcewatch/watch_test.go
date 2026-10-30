// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package mcewatch

import (
	"context"
	"testing"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newMCE(labels map[string]string) *mcev1.MultiClusterEngine {
	return &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterengine",
			Namespace: "multicluster-engine",
			Labels:    labels,
		},
	}
}

func installerLabels() map[string]string {
	return map[string]string{
		"installer.name":      "multiclusterhub",
		"installer.namespace": "open-cluster-management",
	}
}

func newQueue() workqueue.TypedRateLimitingInterface[reconcile.Request] {
	return workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
}

func assertEnqueued(t *testing.T, q workqueue.TypedRateLimitingInterface[reconcile.Request], want bool) {
	t.Helper()

	got := q.Len() > 0
	if got != want {
		t.Fatalf("expected enqueued=%t, got queue length %d", want, q.Len())
	}
	if !want {
		return
	}

	item, _ := q.Get()
	q.Done(item)
	if item.Name != "multiclusterhub" || item.Namespace != "open-cluster-management" {
		t.Fatalf("unexpected request: %v", item)
	}
}

func TestHandlerEnqueuesOnCreate(t *testing.T) {
	q := newQueue()
	Handler().Create(context.Background(), event.TypedCreateEvent[*mcev1.MultiClusterEngine]{
		Object: newMCE(installerLabels()),
	}, q)

	assertEnqueued(t, q, true)
}

func TestHandlerEnqueuesOnDelete(t *testing.T) {
	q := newQueue()
	Handler().Delete(context.Background(), event.TypedDeleteEvent[*mcev1.MultiClusterEngine]{
		Object: newMCE(installerLabels()),
	}, q)

	assertEnqueued(t, q, true)
}

func TestHandlerEnqueuesOnUpdate(t *testing.T) {
	q := newQueue()
	Handler().Update(context.Background(), event.TypedUpdateEvent[*mcev1.MultiClusterEngine]{
		ObjectOld: newMCE(nil),
		ObjectNew: newMCE(installerLabels()),
	}, q)

	assertEnqueued(t, q, true)
}

func TestHandlerUsesLegacyLabels(t *testing.T) {
	q := newQueue()
	Handler().Create(context.Background(), event.TypedCreateEvent[*mcev1.MultiClusterEngine]{
		Object: newMCE(map[string]string{
			"multiclusterhub.name":      "multiclusterhub",
			"multiclusterhub.namespace": "open-cluster-management",
		}),
	}, q)

	assertEnqueued(t, q, true)
}

func TestHandlerIgnoresMissingLabels(t *testing.T) {
	q := newQueue()
	Handler().Create(context.Background(), event.TypedCreateEvent[*mcev1.MultiClusterEngine]{
		Object: newMCE(nil),
	}, q)
	Handler().Delete(context.Background(), event.TypedDeleteEvent[*mcev1.MultiClusterEngine]{
		Object: newMCE(map[string]string{"installer.name": "multiclusterhub"}),
	}, q)

	assertEnqueued(t, q, false)
}

func TestPredicate(t *testing.T) {
	pred := Predicate()
	labeled := newMCE(installerLabels())

	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{
			name: "create without labels",
			got:  pred.Create(event.TypedCreateEvent[*mcev1.MultiClusterEngine]{Object: newMCE(nil)}),
			want: false,
		},
		{
			name: "create with labels",
			got:  pred.Create(event.TypedCreateEvent[*mcev1.MultiClusterEngine]{Object: labeled}),
			want: true,
		},
		{
			name: "delete with labels",
			got:  pred.Delete(event.TypedDeleteEvent[*mcev1.MultiClusterEngine]{Object: labeled}),
			want: true,
		},
		{
			name: "no-op update",
			got: pred.Update(event.TypedUpdateEvent[*mcev1.MultiClusterEngine]{
				ObjectOld: labeled.DeepCopy(),
				ObjectNew: labeled.DeepCopy(),
			}),
			want: false,
		},
		{
			name: "generation change",
			got: func() bool {
				oldObj := labeled.DeepCopy()
				newObj := labeled.DeepCopy()
				newObj.Generation = oldObj.Generation + 1
				return pred.Update(event.TypedUpdateEvent[*mcev1.MultiClusterEngine]{ObjectOld: oldObj, ObjectNew: newObj})
			}(),
			want: true,
		},
		{
			name: "status change",
			got: func() bool {
				oldObj := labeled.DeepCopy()
				newObj := labeled.DeepCopy()
				newObj.Status.Phase = "Unavailable"
				return pred.Update(event.TypedUpdateEvent[*mcev1.MultiClusterEngine]{ObjectOld: oldObj, ObjectNew: newObj})
			}(),
			want: true,
		},
		{
			name: "update losing labels",
			got: pred.Update(event.TypedUpdateEvent[*mcev1.MultiClusterEngine]{
				ObjectOld: labeled.DeepCopy(),
				ObjectNew: newMCE(nil),
			}),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("predicate returned %t, want %t", tt.got, tt.want)
			}
		})
	}
}

func TestRegisterStopsWhenContextCancelled(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := apixv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}
	uncachedClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := Register(ctx, nil, nil, uncachedClient); err != nil {
		t.Fatalf("Register() = %v, want nil on context cancellation", err)
	}
}
