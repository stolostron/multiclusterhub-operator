package webhookcert

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var enqueueLog = log.Log.WithName("eventhandler").WithName("EnqueueRequestForObject")

// NewConfigmapSource returns a source only for signing-cabundle configmap
func NewConfigmapSource(cmInformer cache.SharedIndexInformer) source.Source {
	return &Source{
		informer:     cmInformer,
		expectedType: reflect.TypeOf(&corev1.ConfigMap{}),
		name:         "signing-cabundle-configmap",
	}
}

// NewSecretSource returns a source only for signing-cert secret
func NewSecretSource(secretInformer cache.SharedIndexInformer) source.Source {
	return &Source{
		informer:     secretInformer,
		expectedType: reflect.TypeOf(&corev1.Secret{}),
		name:         "signing-cert-secret",
	}
}

// Source is the event source of specified objects
type Source struct {
	informer     cache.SharedIndexInformer
	expectedType reflect.Type
	name         string
}

var _ source.SyncingSource = &Source{}

func (s *Source) Start(ctx context.Context, handler handler.EventHandler,
	queue workqueue.RateLimitingInterface, predicates ...predicate.Predicate) error {
	_, err := s.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			newObj, ok := obj.(client.Object)
			if !ok {
				enqueueLog.Error(nil, "missing Object, type", obj)
				return
			}

			if objType := reflect.TypeOf(newObj); s.expectedType != objType {
				enqueueLog.Error(nil, "not expected Object", obj)
				return
			}

			createEvent := event.CreateEvent{Object: newObj}

			for _, p := range predicates {
				if !p.Create(createEvent) {
					return
				}
			}

			handler.Create(ctx, createEvent, queue)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldClientObj, ok := oldObj.(client.Object)
			if !ok {
				enqueueLog.Error(nil, "missing old Object,", oldObj)
				return
			}

			if objType := reflect.TypeOf(oldClientObj); s.expectedType != objType {
				enqueueLog.Error(nil, "not expected old Object", oldObj)
				return
			}

			newClientObj, ok := newObj.(client.Object)
			if !ok {
				enqueueLog.Error(nil, "missing old Object", newObj)
				return
			}

			if objType := reflect.TypeOf(newClientObj); s.expectedType != objType {
				enqueueLog.Error(nil, "not expected new Object", newObj)
				return
			}

			updateEvent := event.UpdateEvent{ObjectOld: oldClientObj, ObjectNew: newClientObj}

			for _, p := range predicates {
				if !p.Update(updateEvent) {
					return
				}
			}

			handler.Update(ctx, updateEvent, queue)
		},
		DeleteFunc: func(obj interface{}) {
			if _, ok := obj.(client.Object); !ok {
				// If the object doesn't have Metadata, assume it is a tombstone object of type DeletedFinalStateUnknown
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					enqueueLog.Error(nil, "error decoding objects. Expected cache.DeletedFinalStateUnknown", obj)
					return
				}

				// Set obj to the tombstone obj
				obj = tombstone.Obj
			}

			o, ok := obj.(client.Object)
			if !ok {
				enqueueLog.Error(nil, "missing deleted Object", obj)
				return
			}

			deleteEvent := event.DeleteEvent{Object: o}

			for _, p := range predicates {
				if !p.Delete(deleteEvent) {
					return
				}
			}

			handler.Delete(ctx, deleteEvent, queue)
		},
	})

	return err
}

func (s *Source) WaitForSync(ctx context.Context) error {
	if ok := cache.WaitForCacheSync(ctx.Done(), s.informer.HasSynced); !ok {
		return fmt.Errorf("never achieved initial sync")
	}

	return nil
}

func (s *Source) String() string {
	return s.name
}

// EnqueueRequestForObject enqueues a Request containing the Name and Namespace of the object that is the source of the Event.
// (e.g. the created / deleted / updated objects Name and Namespace).  handler.EnqueueRequestForObject is used by almost all
// Controllers that have associated Resources (e.g. CRDs) to reconcile the associated Resource.
type EnqueueRequestForObject struct {
	namespace string
}

// NewObjectEventHandler maps any event to an empty request
func NewObjectEventHandler(namespace string) *EnqueueRequestForObject {
	return &EnqueueRequestForObject{namespace: namespace}
}

var _ handler.EventHandler = &EnqueueRequestForObject{}

// Create implements EventHandler.
func (e *EnqueueRequestForObject) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Update implements EventHandler.
func (e *EnqueueRequestForObject) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.ObjectNew, q)
}

// Delete implements EventHandler.
func (e *EnqueueRequestForObject) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.add(evt.Object, q)
}

// Generic implements EventHandler.
func (e *EnqueueRequestForObject) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	// do nothing
}

func (e *EnqueueRequestForObject) add(obj client.Object, q workqueue.RateLimitingInterface) {
	if obj.GetNamespace() == e.namespace {
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName(),
			},
		}
		q.Add(request)
	}
}
