// Copyright Contributors to the Open Cluster Management project

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"os"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/helpers"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	pkgerrors "github.com/pkg/errors"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *MultiClusterHubReconciler) applyTemplate(ctx context.Context, m *operatorv1.MultiClusterHub,
	template *unstructured.Unstructured) (ctrl.Result, error) {

	// Set owner reference.
	if (template.GetKind() == "ClusterRole") || (template.GetKind() == "ClusterRoleBinding") || (template.GetKind() == "ServiceMonitor") || (template.GetKind() == "CustomResourceDefinition") {
		utils.AddInstallerLabel(template, m.Name, m.Namespace)
	}

	if template.GetKind() == "APIService" {
		result, err := r.ensureUnstructuredResource(m, template)
		if err != nil {
			log.Info(err.Error())
			return result, err
		}
	} else {
		// Check if the resource exists before creating it.
		for _, gvk := range operatorv1.MCECRDs {
			if template.GroupVersionKind().Group == gvk.Group && template.GetKind() == gvk.Kind &&
				template.GroupVersionKind().Version == gvk.Version {
				crd := &apixv1.CustomResourceDefinition{}

				if err := r.Client.Get(ctx, types.NamespacedName{Name: gvk.Name}, crd); errors.IsNotFound(err) {
					log.Info("CustomResourceDefinition does not exist. Skipping resource creation",
						"Group", gvk.Group, "Version", gvk.Version, "Kind", gvk.Kind, "Name", template.GetName())
					return ctrl.Result{RequeueAfter: utils.WarningRefreshInterval}, nil

				} else if err != nil {
					log.Error(err, "failed to get CustomResourceDefinition", "Resource", gvk)
					return ctrl.Result{}, err
				}
			}
		}

		existing := template.DeepCopy()
		if err := r.Client.Get(ctx, types.NamespacedName{Name: existing.GetName(),
			Namespace: existing.GetNamespace()}, existing); err != nil {
			// Template resource does not exist
			if errors.IsNotFound(err) {
				if err := r.Client.Create(ctx, template, &client.CreateOptions{}); err != nil {
					return r.logAndSetCondition(err, "failed to create resource", template, m)
				}
				log.Info("Creating resource", "Kind", template.GetKind(), "Name", template.GetName())
			} else {
				return r.logAndSetCondition(err, "failed to get resource", existing, m)
			}
		} else {
			desiredVersion := os.Getenv("OPERATOR_VERSION")
			if desiredVersion == "" {
				log.Info("Warning: OPERATOR_VERSION environment variable is not set")
			}

			if !r.ensureResourceVersionAlignment(existing, desiredVersion) {
				condition := NewHubCondition(
					operatorv1.Progressing, metav1.ConditionTrue, ComponentsUpdatingReason,
					fmt.Sprintf("Updating %s/%s to target version: %s.", template.GetKind(),
						template.GetName(), desiredVersion),
				)
				SetHubCondition(&m.Status, *condition)
			}

			/*
				In ACM 2.13 we are applying a PersistentVolumeClaim (PVC) and StatefulSet (STS) for Edge Manager.
				When the PVC is created, we cannot patch the resource if there is a new storageClass available.
				The user would need to delete the pre-existing PVC and allow MCH to recreate a new version with the
				latest default storageClass version.
			*/
			if existing.GetKind() == "PersistentVolumeClaim" {
				storageClassName, found, err := unstructured.NestedString(existing.Object, "spec", "storageClassName")
				if err != nil {
					log.Error(err, "failed to retrieve storageClassName from PVC", "Name", existing.GetName())
					return ctrl.Result{}, err
				}

				if found && storageClassName != os.Getenv(helpers.DefaultStorageClassName) {
					log.Info(
						"To update the PVC with a new StorageClass, delete the existing PVC to allow it to be recreated.",
						"Name", existing.GetName(), "CurrentStorageClass", storageClassName,
						"NewStorageClass", os.Getenv(helpers.DefaultStorageClassName))
					return ctrl.Result{}, nil
				}
			} else if existing.GetKind() == "StatefulSet" {
				volumeClaimTemplates, found, err := unstructured.NestedSlice(existing.Object, "spec",
					"volumeClaimTemplates")

				if err != nil {
					log.Error(err, "failed to retrieve volumeClaimTemplates from StatefulSet", "Name",
						existing.GetName())
					return ctrl.Result{}, err
				}

				if found {
					// Loop through each volumeClaimTemplate to verify that the storage class name remains unchanged.
					for i, volumeClaimTemplate := range volumeClaimTemplates {
						// Extract the storageClassName from each volumeClaimTemplate
						storageClassName, found, err := unstructured.NestedString(
							volumeClaimTemplate.(map[string]interface{}), "spec", "storageClassName")

						if err != nil {
							log.Error(err, "failed to retrieve storageClassName from volumeClaimTemplate", "Index", i,
								"Name", existing.GetName())
							return ctrl.Result{}, err
						}

						if found && storageClassName != os.Getenv(helpers.DefaultStorageClassName) {
							log.Info(
								"To update the STS with a new StorageClass, delete the existing STS to allow it to be recreated.",
								"Name", existing.GetName(), "CurrentStorageClass", storageClassName,
								"NewStorageClass", os.Getenv(helpers.DefaultStorageClassName))
							return ctrl.Result{}, nil
						}
					}
				}
			}

			if !utils.IsTemplateAnnotationTrue(template, utils.AnnotationEditable) {
				// Resource exists; use the original template for patching to avoid issues with managedFields
				// Apply the object data.
				force := true
				if err := r.Client.Patch(ctx, template, client.Apply, &client.PatchOptions{
					Force: &force, FieldManager: "multiclusterhub-operator"}); err != nil {
					return r.logAndSetCondition(err, "failed to update resource", template, m)
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) deleteTemplate(ctx context.Context, m *operatorv1.MultiClusterHub,
	template *unstructured.Unstructured,
) (ctrl.Result, error) {
	err := r.Client.Get(ctx, types.NamespacedName{Name: template.GetName(), Namespace: template.GetNamespace()}, template)

	if err != nil && (errors.IsNotFound(err) || apimeta.IsNoMatchError(err)) {
		return ctrl.Result{}, nil
	}

	// set status progressing condition
	if err != nil {
		log.Error(err, "Odd error delete template")
		return ctrl.Result{}, err
	}

	err = r.Client.Delete(ctx, template)
	if err != nil {
		log.Error(err, "Failed to delete template")
		return ctrl.Result{}, err
	} else {
		r.Log.Info("Finalizing template... Deleting resource", "Kind", template.GetKind(), "Name", template.GetName())
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureResourceVersionAlignment(template *unstructured.Unstructured,
	desiredVersion string) bool {
	if desiredVersion == "" {
		return false
	}

	// Check the release version annotation on the existing resource
	annotations := template.GetAnnotations()
	currentVersion, ok := annotations[utils.AnnotationReleaseVersion]
	if !ok {
		log.Info(fmt.Sprintf("Annotation '%v' not found on resource", utils.AnnotationReleaseVersion),
			"Kind", template.GetKind(), "Name", template.GetName())
		return false
	}

	if currentVersion != desiredVersion {
		log.Info("Resource version mismatch detected; attempting to update resource",
			"Kind", template.GetKind(), "Name", template.GetName(),
			"CurrentVersion", currentVersion, "DesiredVersion", desiredVersion)

		return false
	}

	return true // Resource is aligned with the desired version
}

func (r *MultiClusterHubReconciler) logAndSetCondition(err error, message string,
	template *unstructured.Unstructured, m *operatorv1.MultiClusterHub) (ctrl.Result, error) {

	log.Error(err, message, "Kind", template.GetKind(), "Name", template.GetName())
	wrappedError := pkgerrors.Wrapf(err, "%s Kind: %s Name: %s", message, template.GetKind(), template.GetName())

	condType := fmt.Sprintf("%v: %v (Kind:%v)", operatorv1.ComponentFailure, template.GetName(),
		template.GetKind())

	SetHubCondition(&m.Status, *NewHubCondition(operatorv1.HubConditionType(condType), metav1.ConditionTrue,
		FailedApplyingComponent, wrappedError.Error()))

	return ctrl.Result{}, wrappedError
}
