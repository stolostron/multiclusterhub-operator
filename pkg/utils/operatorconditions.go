package utils

import (
	"context"

	operatorframeworkv2 "github.com/operator-framework/api/pkg/operators/v2"
	"github.com/operator-framework/operator-lib/conditions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	operatorConditionFactory conditions.Factory
)

// Condition - We just need the Set method in our code.
type Condition interface {
	Set(ctx context.Context, status metav1.ConditionStatus, reason, message string) error
}

// OperatorCondition wraps operator-lib's Condition to make it not crash,
// when running locally or in Kubernetes without OLM.
type OperatorCondition struct {
	cond conditions.Condition
}

const (
	UpgradeableInitReason  = "Initializing"
	UpgradeableInitMessage = "The mch operator is starting up"

	UpgradeableUpgradingReason  = "AlreadyPerformingUpgrade"
	UpgradeableUpgradingMessage = "upgrading the mch operator to version "

	UpgradeableAllowReason  = "Upgradeable"
	UpgradeableAllowMessage = ""
)

var GetFactory = func(cl client.Client) conditions.Factory {
	if operatorConditionFactory == nil {
		operatorConditionFactory = conditions.InClusterFactory{Client: cl}
	}
	return operatorConditionFactory
}

func NewOperatorCondition(cl client.Client, condType string) (*OperatorCondition, error) {
	oc := &OperatorCondition{}

	cond, err := GetFactory(cl).NewCondition(operatorframeworkv2.ConditionType(condType))
	if err != nil {
		return nil, err
	}
	oc.cond = cond
	return oc, nil
}

func (oc *OperatorCondition) Set(ctx context.Context, status metav1.ConditionStatus, reason, message string) error {
	if oc == nil || oc.cond == nil {
		// no op
		return nil
	}

	return oc.cond.Set(ctx, status, conditions.WithReason(reason), conditions.WithMessage(message))
}
