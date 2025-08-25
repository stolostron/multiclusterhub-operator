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

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	renderer "github.com/stolostron/multiclusterhub-operator/pkg/rendering"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MultiClusterHubReconciler) fetchChartLocation(component string) string {
	switch component {
	case operatorv1.Appsub:
		return utils.AppsubChartLocation

	case operatorv1.ClusterBackup:
		return utils.ClusterBackupChartLocation

	case operatorv1.ClusterLifecycle:
		return utils.CLCChartLocation

	case operatorv1.ClusterPermission:
		return utils.ClusterPermissionChartLocation

	case operatorv1.Console:
		return utils.ConsoleChartLocation

	case operatorv1.EdgeManagerPreview:
		return utils.EdgeManagerChartLocation

	case operatorv1.FineGrainedRbacPreview:
		return utils.FineGrainedRbacChartLocation

	case operatorv1.GRC:
		return utils.GRCChartLocation

	case operatorv1.Insights:
		return utils.InsightsChartLocation

	case operatorv1.MCH:
		return ""

	case operatorv1.MultiClusterObservability:
		return utils.MCOChartLocation

	case operatorv1.Search:
		return utils.SearchV2ChartLocation

	case operatorv1.MTVIntegrationsPreview:
		return utils.MTVIntegrationsChartLocation

	case operatorv1.SiteConfig:
		return utils.SiteConfigChartLocation

	case operatorv1.SubmarinerAddon:
		return utils.SubmarinerAddonChartLocation

	case operatorv1.Volsync:
		return utils.VolsyncChartLocation

	default:
		log.Info(fmt.Sprintf("Unregistered component detected: %v", component))
		return fmt.Sprintf("/chart/toggle/%v", component)
	}
}

func (r *MultiClusterHubReconciler) ensureComponentOrNoComponent(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string, cachespec CacheSpec, ocpConsole, isSTSEnabled bool) (ctrl.Result, error) {
	var result ctrl.Result
	var err error

	if !m.Enabled(component) {
		if component == operatorv1.ClusterBackup {
			result, err = r.ensureNoComponent(ctx, m, component, cachespec, isSTSEnabled)
			if result != (ctrl.Result{}) || err != nil {
				return result, err
			}
			return r.ensureNoNamespace(m, BackupNamespaceUnstructured())
		}
		if component == operatorv1.EdgeManagerPreview {
			result, err := r.ensureNoComponent(ctx, m, component, cachespec, isSTSEnabled)
			if result != (ctrl.Result{}) || err != nil {
				return result, err
			}
			return r.deleteEdgeManagerResources(ctx, m)
		}

		return r.ensureNoComponent(ctx, m, component, cachespec, isSTSEnabled)

	} else {
		if component == operatorv1.ClusterBackup {
			result, err = r.ensureNamespaceAndPullSecret(m, BackupNamespace())
			if result != (ctrl.Result{}) || err != nil {
				return result, err
			}
		}

		if component == operatorv1.Console && !ocpConsole {
			log.Info("OCP console is not enabled")
			return r.ensureNoComponent(ctx, m, component, cachespec, isSTSEnabled)
		}

		return r.ensureComponent(ctx, m, component, cachespec, isSTSEnabled)
	}
}

func (r *MultiClusterHubReconciler) ensureNamespaceAndPullSecret(m *operatorv1.MultiClusterHub, ns *corev1.Namespace) (
	ctrl.Result, error,
) {
	var result ctrl.Result
	var err error

	result, err = r.ensureNamespace(m, ns)
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensurePullSecret(m, ns.Name)
	if result != (ctrl.Result{}) {
		return result, err
	}

	return result, err
}

func (r *MultiClusterHubReconciler) ensureComponent(ctx context.Context, m *operatorv1.MultiClusterHub, component string,
	cachespec CacheSpec, isSTSEnabled bool) (ctrl.Result, error) {
	/*
	   If the component is detected to be MCH, we can simply return successfully. MCH is only listed in the components
	   list for cleanup purposes.
	*/
	if component == operatorv1.MCH || component == operatorv1.MultiClusterEngine {
		return ctrl.Result{}, nil
	}

	chartLocation := r.fetchChartLocation(component)

	// Ensure that the InternalHubComponent CR instance is created for each component in MCH.
	if result, err := r.ensureInternalHubComponent(ctx, m, component); err != nil {
		return result, err
	}

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(chartLocation, m, cachespec.ImageOverrides, cachespec.TemplateOverrides,
		isSTSEnabled)

	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Apply overrides if available for the component
	if componentConfig, found := r.getComponentConfig(m.Spec.Overrides.Components, component); found {
		for _, template := range templates {
			if ok := template.GetKind() == "Deployment"; ok {
				if deploymentConfig, found := r.getDeploymentConfig(componentConfig.ConfigOverrides.Deployments,
					template.GetName()); found {

					log.V(2).Info("Applying deployment overrides for template", "Name", template.GetName())
					for _, container := range deploymentConfig.Containers {
						if err := r.applyEnvConfig(template, container.Name, container.Env); err != nil {
							return ctrl.Result{}, err
						}
					}

				} else {
					log.V(2).Info("No deployment config found for deployment", "Name", template.GetName())
				}
			}
		}
	} else {
		log.V(2).Info("No component config found", "Component", component)
	}

	// Applies all templates
	for _, template := range templates {
		annotations := template.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[utils.AnnotationReleaseVersion] = version.Version
		template.SetAnnotations(annotations)
		result, err := r.applyTemplate(ctx, m, template)
		if err != nil {
			return result, err
		}
	}

	switch component {
	case operatorv1.Console:
		return r.addPluginToConsole(m)

	case operatorv1.Search:
		return r.ensureSearchCR(m)

	default:
		return ctrl.Result{}, nil
	}
}

func (r *MultiClusterHubReconciler) ensureNoComponent(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string, cachespec CacheSpec, isSTSEnabled bool) (result ctrl.Result, err error) {
	/*
	   If the component is detected to be MCH, we can simply return successfully. MCH is only listed in the components
	   list for cleanup purposes. If the component is detected to be MCE, we can simply return successfully.
	   MCE is only listed in the components list for webhook validation purposes.
	*/
	if component == operatorv1.MCH || component == operatorv1.MultiClusterEngine {
		return ctrl.Result{}, nil
	}

	if result, err := r.ensureNoInternalHubComponent(ctx, m, component); result != (ctrl.Result{}) || err != nil {
		return result, err
	}

	chartLocation := r.fetchChartLocation(component)

	switch component {
	case operatorv1.Console:
		ocpConsole, err := r.CheckConsole(ctx)
		if err != nil {
			r.Log.Error(err, "error finding OCP Console")
			return ctrl.Result{}, err
		}
		if !ocpConsole {
			// If Openshift console is disabled then no cleanup to be done, because MCH console cannot be installed
			return ctrl.Result{}, nil
		}

		result, err := r.removePluginFromConsole(m)
		if result != (ctrl.Result{}) {
			return result, err
		}

	// SearchV2
	case operatorv1.Search:
		result, err := r.ensureNoSearchCR(m)
		if err != nil {
			return result, err
		}

	/*
	   In ACM 2.9 we need to ensure that the submariner ClusterManagementAddOn is removed before
	   removing the submariner-addon component.
	*/
	case operatorv1.SubmarinerAddon:
		result, err := r.ensureNoClusterManagementAddOn(m, component)
		if err != nil {
			return result, err
		}
	}

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(chartLocation, m, cachespec.ImageOverrides, cachespec.TemplateOverrides,
		isSTSEnabled)

	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Deletes all templates
	for _, template := range templates {
		result, err := r.deleteTemplate(ctx, m, template)
		if err != nil {
			logf.Log.Error(err, fmt.Sprintf("Failed to delete template: %s", template.GetName()))
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

/*
getComponentConfig searches for a component configuration in the provided list
by component name. It returns the configuration and a boolean indicating
whether it was found.
*/
func (r *MultiClusterHubReconciler) getComponentConfig(components []operatorv1.ComponentConfig, componentName string) (
	operatorv1.ComponentConfig, bool) {
	for _, c := range components {
		if c.Name == componentName {
			return c, true
		}
	}
	return operatorv1.ComponentConfig{}, false
}

/*
getDeploymentConfig searches for a deployment configuration in the provided list
by deployment name. It returns a pointer to the configuration and nil if not found.
*/
func (r *MultiClusterHubReconciler) getDeploymentConfig(deployments []operatorv1.DeploymentConfig,
	deploymentName string) (*operatorv1.DeploymentConfig, bool) {
	for _, d := range deployments {
		if d.Name == deploymentName {
			return &d, true
		}
	}
	return &operatorv1.DeploymentConfig{}, false
}

/*
applyEnvConfig updates the specified container in the provided template with
new environment variables. Logs errors if encountered during retrieval or update operations.
*/
func (r *MultiClusterHubReconciler) applyEnvConfig(template *unstructured.Unstructured, containerName string,
	envConfigs []operatorv1.EnvConfig) error {

	containers, found, err := unstructured.NestedSlice(template.Object, "spec", "template", "spec", "containers")
	if err != nil || !found {
		log.Error(err, "Failed to get containers from template", "Kind", template.GetKind(), "Name", template.GetName())
		return err
	}

	for i, container := range containers {
		// We need to cast the container to a map of string interfaces to access the container fields.
		containerMap := container.(map[string]interface{})

		if containerMap["name"] == containerName {
			existingEnv, _, _ := unstructured.NestedSlice(containerMap, "env")
			for _, envConfig := range envConfigs {
				envVar := map[string]interface{}{
					"name":  envConfig.Name,
					"value": envConfig.Value,
				}
				existingEnv = append(existingEnv, envVar)
			}

			if err := unstructured.SetNestedSlice(containerMap, existingEnv, "env"); err != nil {
				log.Error(err, "Failed to set environment variable", "Container", containerName)
				return err

			} else {
				containers[i] = containerMap
			}
			break
		}
	}

	if err = unstructured.SetNestedSlice(template.Object, containers, "spec", "template", "spec", "containers"); err != nil {
		log.Error(err, "Failed to set containers in template", "Template", template.GetName())
		return err
	}

	return nil
}
