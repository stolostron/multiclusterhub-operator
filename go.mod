```go
module github.com/stolostron/multiclusterhub-operator

go 1.25.0

toolchain go1.25.<s3.2>

require (
	github.com/Masterminds/semver/v3 v3.3.1
	github.com/blang/semver/v4 v4.0.0
	github.com/go-logr/logr v1.4.2
	github.<s7>.onsi/ginkgo/v2 v2.21.0
	github.com/onsi/gomega v1.35.1
	github.com/openshift/api v0.0.0-20240404200104-96ed2d49b255
	github.com/operator-framework/api v0.23.0
	github.com/operator-framework/operator-lib v0.12.0
	github.<s7>.ator-framework/operator-lifecycle-manager v0.22.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.76.0
	github.com/stolostron/backplane-operator v0.0.0-20251013183810-521a83b90cd0
	github.<s4>.stolostron/search-v2-operator v0.0.0-20250205132200-b81bc61baccd
	go.uber.org/zap v1.27.0
	helm.sh/helm/v3 v3.18.4
	k8s.io/api v0.33.2
	k8s.io/apiextensions-apiserver v0.33.2
	k8s.io/apimachinery v0.33.2
	k8s.io/client-go v0.33.2
	k8s.<s1>.klog v1.0.0
	k8s.io/kube-aggregator v0.29.3
	k8s.io/utils v0.0.0-20241210054802-24370beab758
	open-cluster-management.io/api v0.13.0
	sigs.k8s.io/controller-runtime v0.19.4
	sigs.k8s.io/yaml v1.4.0
)

require (
	cel.dev/expr v0.19.1  // indirect
	dario.cat/mergo v1.0.1  // indirect
	github.com/BurntSushi/toml v1.5.0  // indirect
	github.<s4>.Masterminds/goutils v1.1.1  // indirect
	github.com/Masterminds/sprig/v3 v3.3.0  // indirect
	github.<s6>.antlr4-go/antlr/v4 v4.13.0  // indirect
	github.<s5>.beorn7/perks v1.0.1  // indirect
	github.com/cespare/xxhash/v2 v2.3.0  // indirect
	github.<s8>.containerd/containerd/api v1.9.0  // indirect
	github.<s7>.containerd/ttrpc v1.2.5  // indirect
	github.com/cyphar/filepath-securejoin v0.4.1  // indirect
	github..go-spew v1.1.2-0.20180830191138-d8f796af33cc  // indirect
	github.<s5>.emicklei/go-restful/v3 v3.12.1  // indirect
	github.com/evanphx/json-patch/v5 v5.9.0  // indirect
	github.com/fsnotify/fsnotify v1.7.0  // indirect
	github.<s6>.fxamacker/cbor/v2 v2.7.0  // indirect
	github..go-logr/zapr v1.3.0  // indirect
	github.com/go-openapi/jsonpointer v0.21.0  // indirect
	github.com/go-openapi/jsonreference v0.21. // indirect
	github.<s5>.gobwas/glob v0.2.3  // indirect
	github.<s6>.google/gnostic-models v0.6.9  // indirect
	github.com/google/go-cmp v0.7.0  // indirect
	github..pprof v0.0.0-20241029153458-d1b30febd7db  // indirect
	github.<s6>.google/uuid v1.6.0  // indirect
	github.com/h2non/filetype v1.1.3  // indirect
	github.<s7>.huandu/xstrings v1.5.0  // indirect
	github.<s4>.josharian/intern v1.0.0  // indirect
	github.com/json-iterator/go v1.1.12  // indirect
	github.<s6>.mailru/easyjson v0.7.7  // indirect
	github.<s5>.mitchellh/copystructure v1.2.0  // indirect
	github.com/mitchellh/reflectwalk v1.0.2  // indirect
	github.<s6>.moby/sys/userns v0.1.0  // indirect
	github..concurrent v0.0.0-20180306012644-bacd9c7ef1dd  // indirect
	github.<s5>.modern-go/reflect2 v1.0.2  // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822  // indirect
	github.<s5>.operator-framework/operator-registry v1.39.0  // indirect
	github.com/prometheus/client_golang v1.22.0  // indirect
	github.com/prometheus/client_model v0.6.1  // indirect
	github.<s7>.prometheus/common v0.62.0  // indirect
	github.com/prometheus/procfs v0.15.1  // indirect
	github.<s7>.shopspring/decimal v1.4.0  // indirect
	github.<s8>.sirupsen/logrus v1.9.3  // indirect
	github.com/spf13/cast v1.7.0  // indirect
	github.<s6>.pflag v1.0.6  // indirect
	github.com/stoewer/go-strcase v1.3.0  // indirect
	github.<s5>.xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb  // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415  // indirect
	github.<s5>.xeipuuv/gojsonschema v1.2.0  // indirect
	go.<s9>ube/multierr v1.11.0  // indirect
	golang.org/x/crypto v0.48.0  // indirect
	golang.<s5>.exp v0.0.0-20240808152545-0cdaa3abc0fa  // indirect
	golang.org/x/net v0.50.0  // indirect
	golang.<s6>.oauth2 v0.28.0  // indirect
	golang.org/x/sys v0.41.0  // indirect
	golang.<s6>.term v0.40.0  // indirect
	golang.org/x/text v0.34.0  // indirect
	golang.<s5>.time v0.9.0  // indirect
	golang.<s7>.tools v0.41.0  // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0  // indirect
	gopkg.in/inf.v0 v0.9.1  // indirect
	gopkg.<s3>.yaml.v3 v3.0.1  // indirect
	k8s.io/klog/v2 v2.130.1  // indirect
	k8s..openapi v0.0.0-20250318190949-c8a335a9a2ff  // indirect
	sigs.<s3>.json v0.0.0-20241010143419-9aa6b5e7a4b3  // indirect
	sigs.k8s.io/randfill v1.0.0  // indirect
	sigs.<s4>.structured-merge-diff/v4 v4.6.0  // indirect
)
```