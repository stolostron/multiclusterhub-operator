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
	"reflect"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	configv1 "github.com/openshift/api/config/v1"
	ocopv1 "github.com/openshift/api/operator/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
ensureAuthenticationIssuerNotEmpty ensures that the Authentication ServiceAccountIssuer is not empty.
*/
func (r *MultiClusterHubReconciler) ensureAuthenticationIssuerNotEmpty(ctx context.Context) (ctrl.Result, bool, error) {
	auth := &configv1.Authentication{}

	exists, err := r.ensureObjectExistsAndNotDeleted(ctx, auth, "cluster")

	if err != nil || !exists {
		return ctrl.Result{RequeueAfter: utils.WarningRefreshInterval}, false, err
	}

	stsEnabled := auth.Spec.ServiceAccountIssuer != "" // Determine STS enabled status

	if STSEnabledStatus && !stsEnabled {
		r.Log.Info("Cluster is no longer STS enabled due to empty Authentication ServiceAccountIssuer",
			"Name", auth.GetName())
	}

	return ctrl.Result{}, stsEnabled, nil
}

/*
ensureCloudCredentialModeManual ensures that the CloudCredential CredentialMode is set to Manual.
*/
func (r *MultiClusterHubReconciler) ensureCloudCredentialModeManual(ctx context.Context) (ctrl.Result, bool, error) {
	cloudCred := &ocopv1.CloudCredential{}

	exists, err := r.ensureObjectExistsAndNotDeleted(ctx, cloudCred, "cluster")

	if err != nil || !exists {
		return ctrl.Result{RequeueAfter: utils.WarningRefreshInterval}, false, err
	}

	stsEnabled := cloudCred.Spec.CredentialsMode == "Manual" // Determine STS enabled status

	if STSEnabledStatus && !stsEnabled {
		r.Log.Info("Cluster is no longer STS enabled due to CloudCredential CredentialMode not set to Manual.", "Name",
			cloudCred.GetName())
	}

	return ctrl.Result{}, stsEnabled, nil
}

/*
ensureInfrastructureAWS ensures that the infrastructure platform type is AWS.
*/
func (r *MultiClusterHubReconciler) ensureInfrastructureAWS(ctx context.Context) (ctrl.Result, bool, error) {
	infra := &configv1.Infrastructure{}

	exists, err := r.ensureObjectExistsAndNotDeleted(ctx, infra, "cluster")

	if err != nil || !exists {
		return ctrl.Result{RequeueAfter: utils.WarningRefreshInterval}, false, err
	}

	stsEnabled := infra.Spec.PlatformSpec.Type == "AWS"

	if STSEnabledStatus && !stsEnabled {
		r.Log.Info("Infrastructure platform type is not AWS. Cluster is not STS enabled", "Name", infra.GetName(),
			"Type", infra.Spec.PlatformSpec.Type)
	}
	return ctrl.Result{}, stsEnabled, nil
}

/*
verifyCRDExists checks if the crd exists in the environment
*/
func (r *MultiClusterHubReconciler) verifyCRDExists(ctx context.Context, gvk operatorv1.ResourceGVK) (bool, error) {
	crd := &apixv1.CustomResourceDefinition{}

	// Attempt to find the crd using name
	if err := r.Client.Get(ctx, types.NamespacedName{Name: gvk.Name}, crd); err != nil {
		// CRD does not exist, so we can return false and nil
		if errors.IsNotFound(err) {
			r.Log.Info("Warning: CRD does not exist", "Name", gvk.Name)
			return false, nil
		}

		r.Log.Error(err, "failed to get the CRD", "Name", gvk.Name)
		return false, err
	}

	//found crd
	return true, nil
}

/*
ensureObjectExistsAndNotDeleted ensures the existence of the specified object and that it has not been deleted.
*/
func (r *MultiClusterHubReconciler) ensureObjectExistsAndNotDeleted(ctx context.Context, obj client.Object,
	name string,
) (bool, error) {
	if err := r.Client.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info(
				fmt.Sprintf("%s was not found. Ignoring since object must be deleted",
					reflect.TypeOf(obj).Elem().Name()), "Name", name)
			return false, nil
		}

		r.Log.Error(err, fmt.Sprintf("failed to get %s", reflect.TypeOf(obj).Elem().Name()), "Name", name)
		return false, err
	}

	return true, nil
}

/*
isSTSEnabled checks if STS (Security Token Service) is enabled by verifying that all required conditions are met.
*/
func (r *MultiClusterHubReconciler) isSTSEnabled(ctx context.Context) (bool, error) {
	for _, crd := range operatorv1.RequiredSTSCRDs {
		if ok, err := r.verifyCRDExists(ctx, crd); err != nil || !ok {
			return ok, err
		}
	}

	_, authOK, err := r.ensureAuthenticationIssuerNotEmpty(ctx)
	if err != nil {
		return false, err
	}

	_, cloudCredOK, err := r.ensureCloudCredentialModeManual(ctx)
	if err != nil {
		return false, err
	}

	_, infraOK, err := r.ensureInfrastructureAWS(ctx)
	if err != nil {
		return false, err
	}

	// Check if all conditions are met
	allConditionsMet := authOK && cloudCredOK && infraOK

	// Check if the status has changed, and log the message if it has changed
	if allConditionsMet != STSEnabledStatus {
		STSEnabledStatus = allConditionsMet

		if STSEnabledStatus {
			r.Log.Info("STS is enabled.")
		} else {
			r.Log.Info("STS is not enabled.")
		}
	}

	// Return the combined result of all conditions
	return allConditionsMet, nil
}
