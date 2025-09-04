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
	"path"
	"path/filepath"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/deploying"
	renderer "github.com/stolostron/multiclusterhub-operator/pkg/rendering"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

func (r *MultiClusterHubReconciler) finalizeHub(reqLogger logr.Logger, m *operatorv1.MultiClusterHub, ocpConsole,
	isSTSEnabled bool) error {
	if err := r.cleanupAppSubscriptions(reqLogger, m); err != nil {
		return err
	}

	for _, c := range operatorv1.MCHComponents {
		result, err := r.ensureNoComponent(context.TODO(), m, c, r.CacheSpec, isSTSEnabled)
		if err != nil {
			return err
		}

		if result != (ctrl.Result{}) {
			return errors.NewBadRequest(fmt.Sprintf("Requeue needed for component: %v", c))
		}
	}

	cleanupFunctions := []func(reqLogger logr.Logger, m *operatorv1.MultiClusterHub) error{
		r.cleanupNamespaces, r.cleanupClusterRoles, r.cleanupClusterRoleBindings,
		r.cleanupMultiClusterEngine, r.orphanOwnedMultiClusterEngine,
	}

	for _, cleanupFn := range cleanupFunctions {
		if err := cleanupFn(reqLogger, m); err != nil {
			return err
		}
	}

	_, err := r.deleteEdgeManagerResources(context.Background(), m)
	if err != nil {
		return err
	}

	reqLogger.Info("Successfully finalized multiClusterHub")
	return nil
}

func (r *MultiClusterHubReconciler) installCRDs(reqLogger logr.Logger, m *operatorv1.MultiClusterHub) (string, error) {
	crdDir, ok := os.LookupEnv(crdPathEnvVar)
	if !ok {
		err := fmt.Errorf("%s environment variable is required", crdPathEnvVar)
		reqLogger.Error(err, err.Error())
		return CRDRenderReason, err
	}

	crds, errs := renderer.RenderCRDs(crdDir, m)
	if len(errs) > 0 {
		message := mergeErrors(errs)
		err := fmt.Errorf("failed to render CRD templates: %s", message)
		reqLogger.Error(err, err.Error())
		return CRDRenderReason, err
	}

	for _, crd := range crds {
		utils.AddInstallerLabel(crd, m.GetName(), m.GetNamespace())
		err, ok := deploying.Deploy(r.Client, crd)
		if err != nil {
			reqLogger.Error(err, "failed to deploy", "Kind", crd.GetKind(), "Name", crd.GetName())
			return DeployFailedReason, err
		}
		if ok {
			message := fmt.Sprintf("created new resource: %s %s", crd.GetKind(), crd.GetName())
			condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, message)
			SetHubCondition(&m.Status, *condition)
		}
	}
	return "", nil
}

func (r *MultiClusterHubReconciler) deployResources(reqLogger logr.Logger, m *operatorv1.MultiClusterHub) (string, error) {
	resourceDir, ok := os.LookupEnv(templatesPathEnvVar)
	if !ok {
		err := fmt.Errorf("%s environment variable is required", templatesPathEnvVar)
		reqLogger.Error(err, err.Error())
		return ResourceRenderReason, err
	}

	resourceDir = path.Join(resourceDir, templatesKind, "base")
	files, err := os.ReadDir(resourceDir)
	if err != nil {
		err := fmt.Errorf("unable to read resource files from %s : %s", resourceDir, err)
		reqLogger.Error(err, err.Error())
		return ResourceRenderReason, err
	}

	resources := make([]*unstructured.Unstructured, 0, len(files))
	errs := make([]error, 0, len(files))
	for _, file := range files {
		fileName := file.Name()
		if filepath.Ext(fileName) != ".yaml" {
			continue
		}

		path := path.Join(resourceDir, fileName)
		src, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 (filepath cleaned)
		if err != nil {
			errs = append(errs, fmt.Errorf("error reading file %s : %s", fileName, err))
			continue
		}

		resource := &unstructured.Unstructured{}
		if err = yaml.Unmarshal(src, resource); err != nil {
			errs = append(errs, fmt.Errorf("error unmarshalling file %s to unstructured: %s", fileName, err))
			continue
		}

		resources = append(resources, resource)
	}

	if len(errs) > 0 {
		message := mergeErrors(errs)
		err := fmt.Errorf("failed to render resources: %s", message)
		reqLogger.Error(err, err.Error())
		return CRDRenderReason, err
	}

	for _, res := range resources {
		if res.GetNamespace() == m.Namespace {
			err := controllerutil.SetControllerReference(m, res, r.Scheme)
			if err != nil {
				r.Log.Error(
					err,
					fmt.Sprintf(
						"Failed to set controller reference on %s %s/%s",
						res.GetKind(), m.Namespace, res.GetName(),
					),
				)
			}
		}
		err, ok := deploying.Deploy(r.Client, res)
		if err != nil {
			reqLogger.Error(err, "failed to deploy resource", "Kind", res.GetKind(), "Name", res.GetName())
			return DeployFailedReason, err
		}

		if ok {
			message := fmt.Sprintf("created new resource: %s %s", res.GetKind(), res.GetName())
			condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionTrue, NewComponentReason, message)
			SetHubCondition(&m.Status, *condition)
		}
	}

	return "", nil
}
