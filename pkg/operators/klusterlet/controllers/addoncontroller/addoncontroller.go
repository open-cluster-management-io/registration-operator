package addoncontroller

import (
	"context"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"k8s.io/apimachinery/pkg/runtime"
	coreinformer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"open-cluster-management.io/registration-operator/pkg/helpers"
)

const (
	imagePullSecret = "open-cluster-management-image-pull-credentials"
)

// AddonController is used to sync pull image secret from operator namespace to addon namespaces(with annotation "addon.open-cluster-management.io/namespace":"true")
type addoncontroller struct {
	operatorNamespace string
	namespaceInformer coreinformer.NamespaceInformer
	kubeClient        kubernetes.Interface
	recorder          events.Recorder
}

func NewAddonController(operatorNamespace string, namespaceInformer coreinformer.NamespaceInformer, recorder events.Recorder) factory.Controller {
	ac := &addoncontroller{
		operatorNamespace: operatorNamespace,
		namespaceInformer: namespaceInformer,
		kubeClient:        kubeClient,
		recorder:          recorder,
	}
	return factory.New().WithInformersQueueKeyFunc(func(o runtime.Object) string {
		namespace := o.(*corev1.Namespace)
		if namespace.Annotations["addon.open-cluster-management.io/namespace"] != "true" {
			return ""
		}
		return namespace.GetName()
	}, namespaceInformer).WithSync(ac.sync).ToController("AddonController", &factory.ControllerOptions{})
}

func (c *addoncontroller) sync(ctx context.Context, controllerContext factory.SyncContext) error {
	// Sync secret if namespace is created
	namespace := controllerContext.QueueKey()
	if namespace == "" {
		return nil
	}
	_, _, err := helpers.SyncSecret(
		ctx,
		c.kubeClient,
		c.kubeClient,
		c.recorder,
		c.operatorNamespace,
		imagePullSecret,
		namespace,
		imagePullSecret,
		[]metav1.OwnerReference{},
	)
	if err != nil {
		return err
	}
	return nil
}
