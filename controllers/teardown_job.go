// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	teardownJobName      = "hubteardown-cleanup"
	teardownJobFinalizer = "operator.open-cluster-management.io/teardown-job"

	envTeardownName      = "TEARDOWN_NAME"
	envTeardownNamespace = "TEARDOWN_NAMESPACE"
	envPodNamespace      = "POD_NAMESPACE"
)

// ensureTeardownJob creates or verifies the resilient cleanup Job that survives
// operator pod deletion. The Job runs the same operator binary with a special
// entrypoint flag to continue teardown independently.
func (r *HubTeardownReconciler) ensureTeardownJob(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) error {
	jobKey := types.NamespacedName{
		Name:      teardownJobName,
		Namespace: td.Namespace,
	}

	existing := &batchv1.Job{}
	err := r.Client.Get(ctx, jobKey, existing)
	if err == nil {
		log.Info("Teardown Job already exists", "job", jobKey)
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("checking for existing teardown Job: %w", err)
	}

	operatorImage := os.Getenv("OPERATOR_IMAGE")
	if operatorImage == "" {
		operatorImage = os.Getenv("RELATED_IMAGE_MULTICLUSTERHUB_OPERATOR")
	}
	if operatorImage == "" {
		log.Info("OPERATOR_IMAGE not set, skipping Job creation (teardown will rely on controller only)")
		return nil
	}

	ns := td.Namespace
	if ns == "" {
		ns = os.Getenv(envPodNamespace)
	}

	backoffLimit := int32(6)
	ttlSeconds := int32(3600)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      teardownJobName,
			Namespace: ns,
			Labels: map[string]string{
				"app": "hubteardown",
				"operator.open-cluster-management.io/teardown": "true",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "hubteardown",
						"operator.open-cluster-management.io/teardown": "true",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: "multiclusterhub-operator",
					Containers: []corev1.Container{
						{
							Name:    "teardown",
							Image:   operatorImage,
							Command: []string{"multiclusterhub-operator"},
							Args:    []string{"--teardown-mode"},
							Env: []corev1.EnvVar{
								{Name: envTeardownName, Value: td.Name},
								{Name: envTeardownNamespace, Value: ns},
								{Name: "OPERATOR_VERSION", Value: os.Getenv("OPERATOR_VERSION")},
							},
						},
					},
				},
			},
		},
	}

	if err := r.Client.Create(ctx, job); err != nil {
		return fmt.Errorf("creating teardown Job: %w", err)
	}

	r.Recorder.Event(td, corev1.EventTypeNormal, "TeardownJobCreated",
		"Created resilient teardown Job that will continue cleanup if operator is removed")
	log.Info("Created teardown Job", "job", jobKey, "image", operatorImage)
	return nil
}

// cleanupTeardownJob removes the teardown Job when teardown is complete.
func (r *HubTeardownReconciler) cleanupTeardownJob(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) error {
	jobKey := types.NamespacedName{
		Name:      teardownJobName,
		Namespace: td.Namespace,
	}

	existing := &batchv1.Job{}
	if err := r.Client.Get(ctx, jobKey, existing); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	propagation := metav1.DeletePropagationBackground
	if err := r.Client.Delete(ctx, existing, &client.DeleteOptions{
		PropagationPolicy: &propagation,
	}); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("cleaning up teardown Job: %w", err)
	}

	log.Info("Cleaned up teardown Job")
	return nil
}

// getTeardownJobStatus returns the current status of the teardown Job.
func (r *HubTeardownReconciler) getTeardownJobStatus(ctx context.Context, td *operatorv1.HubTeardown) (*batchv1.JobConditionType, error) {
	jobKey := types.NamespacedName{
		Name:      teardownJobName,
		Namespace: td.Namespace,
	}

	job := &batchv1.Job{}
	if err := r.Client.Get(ctx, jobKey, job); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, cond := range job.Status.Conditions {
		if cond.Status == corev1.ConditionTrue {
			return &cond.Type, nil
		}
	}
	return nil, nil
}
