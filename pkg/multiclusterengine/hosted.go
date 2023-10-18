// Copyright Contributors to the Open Cluster Management project

package multiclusterengine

import (
	"context"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
NewHostedMultiClusterEngine creates a hosted MultiClusterEngine (MCE) configured based on the provided MultiClusterHub
(MCH). It constructs the MCE object with labels, annotations, and various configuration options derived from the MCH.
*/
func NewHostedMultiClusterEngine(m *operatorv1.MultiClusterHub) *mcev1.MultiClusterEngine {
	labels := map[string]string{
		"installer.name":        m.GetName(),
		"installer.namespace":   m.GetNamespace(),
		utils.MCEManagedByLabel: "true",
	}
	annotations := GetHostedAnnotations(m)
	availConfig := mcev1.HAHigh
	if m.Spec.AvailabilityConfig == operatorv1.HABasic {
		availConfig = mcev1.HABasic
	}

	mce := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Name:        HostedMCEName(m),
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: mcev1.MultiClusterEngineSpec{
			ImagePullSecret:    m.Spec.ImagePullSecret,
			Tolerations:        utils.GetTolerations(m),
			NodeSelector:       m.Spec.NodeSelector,
			AvailabilityConfig: availConfig,
			TargetNamespace:    HostedMCENamespace(m).Name,
			Overrides: &mcev1.Overrides{
				Components: utils.GetMCEComponents(m),
			},
		},
	}

	if m.Spec.Overrides != nil && m.Spec.Overrides.ImagePullPolicy != "" {
		mce.Spec.Overrides.ImagePullPolicy = m.Spec.Overrides.ImagePullPolicy
	}

	return mce
}

/*
RenderHostedMultiClusterEngine takes an existing hosted MultiClusterEngine (MCE) and a MultiClusterHub (MCH) as input.
It updates and returns a modified MCE by applying changes from the MCH. This includes updating annotations,
image pull secret, tolerations, node selector, availability configuration, and component overrides.
*/
func RenderHostedMultiClusterEngine(existingMCE *mcev1.MultiClusterEngine,
	m *operatorv1.MultiClusterHub) *mcev1.MultiClusterEngine {
	copy := existingMCE.DeepCopy()

	// add annotations
	annotations := GetHostedAnnotations(m)
	if len(annotations) > 0 {
		newAnnotations := copy.GetAnnotations()
		if newAnnotations == nil {
			newAnnotations = make(map[string]string)
		}
		for key, val := range annotations {
			newAnnotations[key] = val
		}
		copy.SetAnnotations(newAnnotations)
	}

	if m.Spec.AvailabilityConfig == operatorv1.HABasic {
		copy.Spec.AvailabilityConfig = mcev1.HABasic
	} else {
		copy.Spec.AvailabilityConfig = mcev1.HAHigh
	}

	copy.Spec.ImagePullSecret = m.Spec.ImagePullSecret
	copy.Spec.Tolerations = utils.GetTolerations(m)
	copy.Spec.NodeSelector = m.Spec.NodeSelector

	for _, component := range utils.GetMCEComponents(m) {
		if component.Enabled {
			copy.Enable(component.Name)
		} else {
			copy.Disable(component.Name)
		}
	}

	if m.Spec.Overrides != nil && m.Spec.Overrides.ImagePullPolicy != "" {
		copy.Spec.Overrides.ImagePullPolicy = m.Spec.Overrides.ImagePullPolicy
	}

	return copy
}

/*
HostedMCEName generates a name for the hosted MultiClusterEngine (MCE) based on the name of the provided
MultiClusterHub (MCH). The generated name follows the pattern "<mch-name>-engine."
*/
func HostedMCEName(m *operatorv1.MultiClusterHub) string {
	return m.Name + "-engine"
}

/*
HostedMCENamespace creates a Kubernetes namespace object for the hosted MultiClusterEngine (MCE) based on the
namespace of the provided MultiClusterHub (MCH). The namespace name follows the pattern "<mch-namespace>-engine."
*/
func HostedMCENamespace(m *operatorv1.MultiClusterHub) *corev1.Namespace {
	namespace := m.Namespace + "-engine"
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
}

/*
GetHostedAnnotations extracts and copies relevant annotations from the MultiClusterHub (MCH) to be associated
with the hosted MultiClusterEngine (MCE). It includes annotations related to image repositories, kubeconfig, and
deployment mode. The generated annotations are returned as a map.
*/
func GetHostedAnnotations(m *operatorv1.MultiClusterHub) map[string]string {
	mceAnnotations := make(map[string]string)
	if m.GetAnnotations() != nil {
		if val, ok := m.GetAnnotations()[utils.AnnotationImageRepo]; ok && val != "" {
			mceAnnotations["imageRepository"] = val
		}
		// Hosted specific annotations
		if val, ok := m.GetAnnotations()[utils.AnnotationKubeconfig]; ok && val != "" {
			mceAnnotations["mce-kubeconfig"] = val
		}
	}
	mceAnnotations["deploymentmode"] = "Hosted"
	return mceAnnotations
}

/*
GetHostedMCE retrieves the associated hosted MultiClusterEngine (MCE) based on the provided MultiClusterHub (MCH)
using the name derived from the MCH. It interacts with a Kubernetes client to fetch the MCE.
If the MCE doesn't exist, it returns nil.
*/
func GetHostedMCE(ctx context.Context, k8sClient client.Client, m *operatorv1.MultiClusterHub) (
	*mcev1.MultiClusterEngine, error) {
	mce := &mcev1.MultiClusterEngine{}
	// We can derive the name of the MCE from MCH name
	key := types.NamespacedName{Name: HostedMCEName(m)}
	err := k8sClient.Get(ctx, key, mce)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mce, nil
}
