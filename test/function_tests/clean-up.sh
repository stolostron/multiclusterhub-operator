#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project


if [ -z ${ACM_NAMESPACE+x} ]; 
then 
    echo "ACM_NAMESPACE must be set"
    exit 1
else 
    echo "Cleaning up resources in namespace: $ACM_NAMESPACE"
fi

oc delete mch --all -n $ACM_NAMESPACE
oc delete apiservice v1.admission.cluster.open-cluster-management.io v1.admission.work.open-cluster-management.io
oc delete clusterimageset --all
oc delete clusterrole multiclusterengines.multicluster.openshift.io-v1-admin multiclusterengines.multicluster.openshift.io-v1-crdview multiclusterengines.multicluster.openshift.io-v1-edit multiclusterengines.multicluster.openshift.io-v1-view open-cluster-management:addons:application-manager open-cluster-management:admin-aggregate open-cluster-management:cert-policy-controller-hub open-cluster-management:cluster-manager-admin-aggregate open-cluster-management:config-policy-controller-hub open-cluster-management:edit-aggregate open-cluster-management:iam-policy-controller-hub open-cluster-management:policy-framework-hub open-cluster-management:view-aggregate
oc delete crd klusterletaddonconfigs.agent.open-cluster-management.io placementbindings.policy.open-cluster-management.io policies.policy.open-cluster-management.io userpreferences.console.open-cluster-management.io discoveredclusters.discovery.open-cluster-management.io discoveryconfigs.discovery.open-cluster-management.io
oc delete mutatingwebhookconfiguration ocm-mutating-webhook managedclustermutators.admission.cluster.open-cluster-management.io multicluster-observability-operator
oc delete validatingwebhookconfiguration channels.apps.open.cluster.management.webhook.validator application-webhook-validator multiclusterhub-operator-validating-webhook ocm-validating-webhook multicluster-observability-operator multiclusterengines.multicluster.openshift.io

exit 0
