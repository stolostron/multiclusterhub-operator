// Copyright (c) 2025 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterengineutils

import (
	"context"
	"fmt"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MCEManagedByLabel is the label used to mark resources managed by Multicluster Hub.
const MCEManagedByLabel = "multiclusterhubs.operator.open-cluster-management.io/managed-by"

// Finds MCE by managed label. Returns nil if none found.
func GetManagedMCE(ctx context.Context, k8sClient client.Client) (*mcev1.MultiClusterEngine, error) {
	mceList := &mcev1.MultiClusterEngineList{}
	if err := k8sClient.List(ctx, mceList, &client.MatchingLabels{MCEManagedByLabel: "true"}); err != nil {
		return nil, err
	}

	if len(mceList.Items) == 1 {
		return &mceList.Items[0], nil

	} else if len(mceList.Items) > 1 {
		// will require manual resolution
		return nil, fmt.Errorf("multiple MCEs found managed by MCH. Only one MCE is supported")
	}

	return nil, nil
}
