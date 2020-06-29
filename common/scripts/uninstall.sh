#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.

oc delete MultiClusterHub --all --ignore-not-found

# Wait for all helmrelease finalizers
oc delete helmrelease --all --ignore-not-found

# Delete subscriptions
oc delete sub multicluster-operators-subscription-alpha-community-operators-openshift-marketplace --ignore-not-found
oc delete sub hive-operator-alpha-community-operators-openshift-marketplace --ignore-not-found
oc delete sub multiclusterhub-operator --ignore-not-found

# Delete CSVs
oc get csv | grep "hive-operator" | awk '{ print $1 }' | xargs oc delete csv --wait=false --ignore-not-found || true
oc get csv | grep "multicluster-operators-subscription" | awk '{ print $1 }' | xargs oc delete csv --wait=false --ignore-not-found || true
oc get csv | grep "multiclusterhub-operator" | awk '{ print $1 }' | xargs oc delete csv --wait=false --ignore-not-found || true

# Delete catalogsource
oc delete catalogsource mch-catalog-source --ignore-not-found

# Delete CRDs
oc get crd | grep "hive" | awk '{ print $1 }' | xargs oc delete crd --wait=false --ignore-not-found
oc get crd | grep "open-cluster-management" | awk '{ print $1 }' | xargs oc delete crd --wait=false --ignore-not-found
oc get crd | grep "hive" | awk '{ print $1 }' | xargs oc delete crd --wait=false --ignore-not-found
# oc get crd | grep "mcm" | awk '{ print $1 }' | xargs oc delete crd --wait=false --ignore-not-found || true
oc get crd | grep "cert" | awk '{ print $1 }' | xargs oc delete crd --wait=false --ignore-not-found || true

# Delete services
oc get service | grep "multiclusterhub" | awk '{ print $1 }' | xargs oc delete service --wait=false --ignore-not-found

# Delete roles + clusterroles + bindings
oc get clusterrole | grep "multiclusterhub-operator" | awk '{ print $1 }' | xargs oc delete clusterrole --wait=false --ignore-not-found
oc get clusterrole | grep "mcm" | awk '{ print $1 }' | xargs oc delete clusterrole --wait=false --ignore-not-found
oc delete clusterrole hive-admin || true
oc delete clusterrole hive-reader || true
oc delete clusterrole cert-manager-webhook-requester || true
oc delete clusterrolebinding cert-manager-webhook-auth-delegator || true
oc delete clusterrole cert-manager-webhook-requester || true
oc delete clusterrolebinding cert-manager-webhook-auth-delegator || true

# Delete apiservices
oc delete apiservice v1.admission.hive.openshift.io || true
oc delete apiservice v1.hive.openshift.io || true
oc delete apiservice v1beta1.webhook.certmanager.k8s.io || true

# Delete webhooks
oc delete validatingwebhookconfiguration multiclusterhub-operator-validating-webhook --ignore-not-found
oc delete mutatingwebhookconfiguration multiclusterhub-operator-mutating-webhook --ignore-not-found
oc delete validatingwebhookconfiguration cert-manager-webhook --ignore-not-found

# Delete configmaps
oc delete configmap hive-operator-leader --ignore-not-found

# Delete SCCs
oc delete scc multicloud-scc || true

# Delete deploy resources if an in-cluster install
oc delete -k deploy/

# Other
oc delete consolelink acm-console-link --ignore-not-found

# Delete Registration Operator
_regOpDir=build/registration-operator
if [ -d "$_regOpDir" ]; then
    oc delete clustermanager --all
    cd $_regOpDir
    make clean-hub OLM_NAMESPACE=openshift-operator-lifecycle-manager
    cd ../..
    oc delete deploy cluster-manager || true
    oc get csv | grep "cluster-manager" | awk '{ print $1 }' | xargs oc delete csv --wait=false --ignore-not-found || true
    oc get sub | grep "cluster-manager" | awk '{ print $1 }' | xargs oc delete sub --wait=false --ignore-not-found || true

    # Uncomment for complete uninstall of cluster-manager. Cluster-manager stands up much quicker if we leave these resources. 
    
    # oc delete configmap cluster-manager-registry-bundles -n openshift-operator-lifecycle-manager || true
    # oc delete deploy cluster-manager-registry-server -n openshift-operator-lifecycle-manager || true
    # oc delete service cluster-manager-registry-server -n openshift-operator-lifecycle-manager || true
fi