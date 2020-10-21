package helpers

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/library-go/pkg/controller/factory"

	operatorlister "github.com/open-cluster-management/api/client/operator/listers/operator/v1"
	operatorapiv1 "github.com/open-cluster-management/api/operator/v1"
)

const (
	// KlusterletDefaultNamespace is the default namespace of klusterlet
	KlusterletDefaultNamespace = "open-cluster-management-agent"
	// BootstrapHubKubeConfig is the secret name of bootstrap kubeconfig secret to connect to hub
	BootstrapHubKubeConfig = "bootstrap-hub-kubeconfig"
	// HubKubeConfig is the secret name of kubeconfig secret to connect to hub with mtls
	HubKubeConfig = "hub-kubeconfig-secret"
	// ClusterManagerNamespace is the namespace to deploy cluster manager components
	ClusterManagerNamespace = "open-cluster-management-hub"
)

func KlusterlsetSecretEventFilter(obj interface{}) bool {
	accessor, _ := meta.Accessor(obj)
	name := accessor.GetName()
	return name == HubKubeConfig || name == BootstrapHubKubeConfig
}

func KlusterletDeploymentEventFilter(obj interface{}) bool {
	accessor, _ := meta.Accessor(obj)
	name := accessor.GetName()
	return strings.HasSuffix(name, "registration-agent") || strings.HasSuffix(name, "work-agent")
}

func ClusterManagerDeploymentEventFilter(obj interface{}) bool {
	accessor, _ := meta.Accessor(obj)
	name := accessor.GetName()
	namespace := accessor.GetNamespace()
	if namespace != ClusterManagerNamespace {
		return false
	}
	if strings.HasSuffix(name, "registration-controller") || strings.HasSuffix(name, "work-controller") {
		return true
	}
	return false
}

func KlusterletQueueKeyFunc(klusterletLister operatorlister.KlusterletLister) factory.ObjectQueueKeyFunc {
	return func(obj runtime.Object) string {
		accessor, _ := meta.Accessor(obj)
		namespace := accessor.GetNamespace()
		klusterlets, err := klusterletLister.List(labels.Everything())
		if err != nil {
			return ""
		}

		if klusterlet := FindKlusterletByNamespace(klusterlets, namespace); klusterlet != nil {
			return klusterlet.Name
		}

		return ""
	}
}

func ClusterManagerQueueKeyFunc(clusterManagerLister operatorlister.ClusterManagerLister) factory.ObjectQueueKeyFunc {
	return func(obj runtime.Object) string {
		clustermanagers, err := clusterManagerLister.List(labels.Everything())
		if err != nil {
			return ""
		}

		for _, clustermanager := range clustermanagers {
			return clustermanager.Name
		}

		return ""
	}
}

func FindKlusterletByNamespace(klusterlets []*operatorapiv1.Klusterlet, namespace string) *operatorapiv1.Klusterlet {
	for _, klusterlet := range klusterlets {
		klusterletNS := klusterlet.Spec.Namespace
		if klusterletNS == "" {
			klusterletNS = KlusterletDefaultNamespace
		}
		if namespace == klusterletNS {
			return klusterlet
		}
	}
	return nil
}
