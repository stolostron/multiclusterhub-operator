// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

// Package mcewatch registers the MultiClusterEngine watch that keeps MultiClusterHub
// reconciliation in sync with out-of-band MCE installs, upgrades and removals.
package mcewatch

import (
	"context"
	"fmt"
	"time"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// CRDName is the CRD that must exist before the MultiClusterEngine watch is registered.
	CRDName = "multiclusterengines.multicluster.openshift.io"
	// RetryInterval is how often Register re-checks for the MultiClusterEngine CRD.
	RetryInterval = 30 * time.Second
)

var log = logf.Log.WithName("mcewatch")

// Register waits for the MultiClusterEngine CRD to be installed and then adds a watch on
// MultiClusterEngine to the given controller. It is meant to run as a manager Runnable: when
// the manager context is cancelled the wait is aborted, so nothing is leaked on shutdown.
func Register(ctx context.Context, mgr manager.Manager, c controller.Controller, uncachedClient client.Client) error {
	crd := &apixv1.CustomResourceDefinition{}

	err := wait.PollUntilContextCancel(ctx, RetryInterval, true, func(ctx context.Context) (bool, error) {
		err := uncachedClient.Get(ctx, types.NamespacedName{Name: CRDName}, crd)
		if err == nil {
			return true, nil
		}
		if !apierrors.IsNotFound(err) {
			log.Error(err, "failed to check for the MultiClusterEngine CRD, retrying")
		}
		return false, nil
	})
	if err != nil {
		// The condition never returns an error, so this can only be context cancellation.
		log.Info("stopping MCE watch setup: manager is shutting down")
		return nil
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &mcev1.MultiClusterEngine{}, Handler(), Predicate())); err != nil {
		return fmt.Errorf("registering MultiClusterEngine watch: %w", err)
	}

	log.Info("mce watch added")
	return nil
}

// Handler enqueues the owning MultiClusterHub reconcile request for every MultiClusterEngine
// event. Create, update and delete are all handled so that out-of-band creation or removal of
// MCE is reconciled without waiting for the resync period.
func Handler() handler.TypedEventHandler[*mcev1.MultiClusterEngine, reconcile.Request] {
	enqueue := func(obj *mcev1.MultiClusterEngine, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		req, ok := ReconcileRequest(obj)
		if !ok {
			log.Info(fmt.Sprintf(
				"MCE %s/%s event ignored: missing installer.name or installer.namespace labels. Current labels: %v",
				obj.GetNamespace(), obj.GetName(), obj.GetLabels()))
			return
		}
		q.Add(req)
	}

	return handler.TypedFuncs[*mcev1.MultiClusterEngine, reconcile.Request]{
		CreateFunc: func(_ context.Context, e event.TypedCreateEvent[*mcev1.MultiClusterEngine], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueue(e.Object, q)
		},
		UpdateFunc: func(_ context.Context, e event.TypedUpdateEvent[*mcev1.MultiClusterEngine], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueue(e.ObjectNew, q)
		},
		DeleteFunc: func(_ context.Context, e event.TypedDeleteEvent[*mcev1.MultiClusterEngine], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			enqueue(e.Object, q)
		},
	}
}

// ReconcileRequest derives the MultiClusterHub reconcile request from the installer labels set
// on the MultiClusterEngine resource.
func ReconcileRequest(obj client.Object) (reconcile.Request, bool) {
	labels := obj.GetLabels()
	name := labels["installer.name"]
	if name == "" {
		name = labels["multiclusterhub.name"]
	}
	namespace := labels["installer.namespace"]
	if namespace == "" {
		namespace = labels["multiclusterhub.namespace"]
	}
	if name == "" || namespace == "" {
		return reconcile.Request{}, false
	}
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: namespace}}, true
}

// Predicate drops events for MultiClusterEngine resources that cannot map to a MultiClusterHub
// and no-op updates where neither spec, metadata nor status changed.
func Predicate() predicate.TypedPredicate[*mcev1.MultiClusterEngine] {
	hasHub := func(obj *mcev1.MultiClusterEngine) bool {
		_, ok := ReconcileRequest(obj)
		return ok
	}

	return predicate.TypedFuncs[*mcev1.MultiClusterEngine]{
		CreateFunc: func(e event.TypedCreateEvent[*mcev1.MultiClusterEngine]) bool {
			return hasHub(e.Object)
		},
		UpdateFunc: func(e event.TypedUpdateEvent[*mcev1.MultiClusterEngine]) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil || !hasHub(e.ObjectNew) {
				return false
			}
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() ||
				!equality.Semantic.DeepEqual(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels()) ||
				!equality.Semantic.DeepEqual(e.ObjectOld.GetAnnotations(), e.ObjectNew.GetAnnotations()) ||
				!equality.Semantic.DeepEqual(e.ObjectOld.Status, e.ObjectNew.Status)
		},
		DeleteFunc: func(e event.TypedDeleteEvent[*mcev1.MultiClusterEngine]) bool {
			return hasHub(e.Object)
		},
		GenericFunc: func(e event.TypedGenericEvent[*mcev1.MultiClusterEngine]) bool {
			return hasHub(e.Object)
		},
	}
}
