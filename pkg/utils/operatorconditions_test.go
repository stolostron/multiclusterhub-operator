// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "reflect"
	"testing"
)

func TestNewOperatorCondition(t *testing.T) {
	t.Run("Unpaused MCH", func(t *testing.T) {
		oc := &OperatorCondition{}
		msg := UpgradeableAllowMessage
		status := metav1.ConditionTrue
		reason := UpgradeableAllowReason
		ctx := context.Background()
		err := oc.Set(ctx, status, reason, msg)
		if err != nil {
			t.Errorf("Unable to set")
		}
	})
}
