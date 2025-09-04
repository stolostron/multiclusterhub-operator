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
	"os"
	"strings"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/helpers"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MultiClusterHubReconciler) GetDefaultStorageClassName(storageClasses storagev1.StorageClassList) string {
	for _, sc := range storageClasses.Items {
		if annotations := sc.GetAnnotations(); annotations != nil {
			if strings.EqualFold(annotations[utils.AnnotationKubeDefaultStorageClass], "true") {
				return sc.GetName()
			}
		}
	}

	if len(storageClasses.Items) > 1 {
		log.Info("Warning: Multiple non-default storage classes found. A default storage class needs to be declared.")
	}
	return ""
}

func (r *MultiClusterHubReconciler) SetDefaultStorageClassName(ctx context.Context, m *operatorv1.MultiClusterHub) (
	ctrl.Result, error) {

	// Retrieve the default storage class name from the environment variable, if set.
	envStorageClass := os.Getenv(helpers.DefaultStorageClassName)

	/*
	   Check if the MultiClusterHub instance contains a default storage class annotation.
	   If the annotation is present and different from the environment variable, override it.
	*/
	if overrideStorageClass := utils.GetDefaultStorageClassOverride(m); overrideStorageClass != "" &&
		overrideStorageClass != envStorageClass {

		if err := os.Setenv(helpers.DefaultStorageClassName, overrideStorageClass); err != nil {
			log.Error(err, "unable to set the default StorageClass environment variable from annotation",
				helpers.DefaultStorageClassName, overrideStorageClass)

			return ctrl.Result{}, err
		}

		log.Info("Applied default StorageClass annotation override",
			"StorageClassName", overrideStorageClass)
		return ctrl.Result{}, nil
	}

	// If no annotation override is found, we need to discover the default storage class from the cluster.
	storageClasses := storagev1.StorageClassList{}
	if err := r.Client.List(ctx, &storageClasses); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("No StorageClass resources found in the cluster. Skipping default StorageClass update")
			return ctrl.Result{}, nil
		}

		r.Log.Error(err, "failed to list StorageClass resources")
		return ctrl.Result{}, err
	}

	// Retrieve the default storage class from the cluster's StorageClass resources.
	if defaultStorageClass := r.GetDefaultStorageClassName(storageClasses); defaultStorageClass != "" &&
		defaultStorageClass != envStorageClass {
		if err := os.Setenv(helpers.DefaultStorageClassName, defaultStorageClass); err != nil {
			return ctrl.Result{}, err
		}

		logf.Log.Info("Default StorageClassName set from cluster resources",
			"Name", defaultStorageClass)
	}
	return ctrl.Result{}, nil
}
