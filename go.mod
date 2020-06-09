module github.com/open-cluster-management/multicloudhub-operator

go 1.13

require (
	github.com/fatih/structs v1.1.0
	github.com/go-logr/logr v0.1.0
	github.com/openshift/api v0.0.0-20200205133042-34f0ec8dab87
	github.com/operator-framework/operator-sdk v0.18.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.18.3
	k8s.io/apiextensions-apiserver v0.18.2
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v13.0.0+incompatible
	k8s.io/kube-aggregator v0.18.3
	sigs.k8s.io/controller-runtime v0.6.0
	sigs.k8s.io/kustomize/v3 v3.3.1
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.18.2 // Required by prometheus-operator
)
