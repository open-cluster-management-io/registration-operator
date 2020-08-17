module github.com/open-cluster-management/registration-operator

go 1.13

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/gorilla/websocket v1.4.2 // indirect; required by spf13/cobra, after spf13/cobra has new release we can remove this
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/open-cluster-management/api v0.0.0-20200806152638-5caae08712f5
	github.com/openshift/api v0.0.0-20200521101457-60c476765272
	github.com/openshift/build-machinery-go v0.0.0-20200424080330-082bf86082cc
	github.com/openshift/library-go v0.0.0-20200617154932-eaf8c138def4
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.18.3
	k8s.io/apiextensions-apiserver v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v0.18.3
	k8s.io/component-base v0.18.3
	k8s.io/klog v1.0.0
	k8s.io/kube-aggregator v0.18.3
	sigs.k8s.io/controller-runtime v0.6.0
)

// this is required by openshift/library-go, after openshift/library-go update this lib, we can remove this
replace github.com/opencontainers/runc => github.com/opencontainers/runc v1.0.0-rc10
