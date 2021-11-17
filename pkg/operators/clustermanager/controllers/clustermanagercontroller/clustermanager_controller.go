package clustermanagercontroller

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	appsinformer "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	apiregistrationclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"

	operatorv1client "open-cluster-management.io/api/client/operator/clientset/versioned/typed/operator/v1"
	operatorinformer "open-cluster-management.io/api/client/operator/informers/externalversions/operator/v1"
	operatorlister "open-cluster-management.io/api/client/operator/listers/operator/v1"
	operatorapiv1 "open-cluster-management.io/api/operator/v1"
	"open-cluster-management.io/registration-operator/manifests"
	"open-cluster-management.io/registration-operator/pkg/helpers"
)

// default mode
var (
	crdNames = []string{
		"manifestworks.work.open-cluster-management.io",
		"managedclusters.cluster.open-cluster-management.io",
	}
	staticResourceFiles = []string{
		"cluster-manager/0000_00_addon.open-cluster-management.io_clustermanagementaddons.crd.yaml",
		"cluster-manager/0000_00_clusters.open-cluster-management.io_managedclusters.crd.yaml",
		"cluster-manager/0000_00_clusters.open-cluster-management.io_managedclustersets.crd.yaml",
		"cluster-manager/0000_00_work.open-cluster-management.io_manifestworks.crd.yaml",
		"cluster-manager/0000_01_addon.open-cluster-management.io_managedclusteraddons.crd.yaml",
		"cluster-manager/0000_01_clusters.open-cluster-management.io_managedclustersetbindings.crd.yaml",
		"cluster-manager/0000_03_clusters.open-cluster-management.io_placements.crd.yaml",
		"cluster-manager/0000_04_clusters.open-cluster-management.io_placementdecisions.crd.yaml",
		"cluster-manager/cluster-manager-registration-clusterrole.yaml",
		"cluster-manager/cluster-manager-registration-clusterrolebinding.yaml",
		"cluster-manager/cluster-manager-namespace.yaml",
		"cluster-manager/cluster-manager-registration-serviceaccount.yaml",
		"cluster-manager/cluster-manager-registration-webhook-clusterrole.yaml",
		"cluster-manager/cluster-manager-registration-webhook-clusterrolebinding.yaml",
		"cluster-manager/cluster-manager-registration-webhook-service.yaml",
		"cluster-manager/cluster-manager-registration-webhook-serviceaccount.yaml",
		"cluster-manager/cluster-manager-registration-webhook-apiservice.yaml",
		"cluster-manager/cluster-manager-registration-webhook-clustersetbinding-validatingconfiguration.yaml",
		"cluster-manager/cluster-manager-registration-webhook-validatingconfiguration.yaml",
		"cluster-manager/cluster-manager-registration-webhook-mutatingconfiguration.yaml",
		"cluster-manager/cluster-manager-work-webhook-clusterrole.yaml",
		"cluster-manager/cluster-manager-work-webhook-clusterrolebinding.yaml",
		"cluster-manager/cluster-manager-work-webhook-service.yaml",
		"cluster-manager/cluster-manager-work-webhook-serviceaccount.yaml",
		"cluster-manager/cluster-manager-work-webhook-apiservice.yaml",
		"cluster-manager/cluster-manager-work-webhook-validatingconfiguration.yaml",
		"cluster-manager/cluster-manager-placement-clusterrole.yaml",
		"cluster-manager/cluster-manager-placement-clusterrolebinding.yaml",
		"cluster-manager/cluster-manager-placement-serviceaccount.yaml",
	}

	deploymentFiles = []string{
		"cluster-manager/cluster-manager-registration-deployment.yaml",
		"cluster-manager/cluster-manager-registration-webhook-deployment.yaml",
		"cluster-manager/cluster-manager-work-webhook-deployment.yaml",
		"cluster-manager/cluster-manager-placement-deployment.yaml",
	}

	staticResourceFilesHostedMode = []string{
		"cluster-manager-hosted/0000_00_addon.open-cluster-management.io_clustermanagementaddons.crd.yaml",
		"cluster-manager-hosted/0000_00_clusters.open-cluster-management.io_managedclusters.crd.yaml",
		"cluster-manager-hosted/0000_00_clusters.open-cluster-management.io_managedclustersets.crd.yaml",
		"cluster-manager-hosted/0000_00_work.open-cluster-management.io_manifestworks.crd.yaml",
		"cluster-manager-hosted/0000_01_addon.open-cluster-management.io_managedclusteraddons.crd.yaml",
		"cluster-manager-hosted/0000_01_clusters.open-cluster-management.io_managedclustersetbindings.crd.yaml",
		"cluster-manager-hosted/0000_03_clusters.open-cluster-management.io_placements.crd.yaml",
		"cluster-manager-hosted/0000_04_clusters.open-cluster-management.io_placementdecisions.crd.yaml",
		"cluster-manager-hosted/cluster-manager-registration-clusterrole.yaml",
		"cluster-manager-hosted/cluster-manager-registration-clusterrolebinding.yaml",
		"cluster-manager-hosted/cluster-manager-namespace.yaml",
		"cluster-manager-hosted/cluster-manager-registration-serviceaccount.yaml",
		"cluster-manager-hosted/cluster-manager-registration-webhook-clusterrole.yaml",
		"cluster-manager-hosted/cluster-manager-registration-webhook-clusterrolebinding.yaml",
		"cluster-manager-hosted/cluster-manager-registration-webhook-service.yaml",
		"cluster-manager-hosted/cluster-manager-registration-webhook-serviceaccount.yaml",
		"cluster-manager-hosted/cluster-manager-registration-webhook-apiservice.yaml",
		"cluster-manager-hosted/cluster-manager-registration-webhook-clustersetbinding-validatingconfiguration.yaml",
		"cluster-manager-hosted/cluster-manager-registration-webhook-validatingconfiguration.yaml",
		"cluster-manager-hosted/cluster-manager-registration-webhook-mutatingconfiguration.yaml",
		"cluster-manager-hosted/cluster-manager-work-webhook-clusterrole.yaml",
		"cluster-manager-hosted/cluster-manager-work-webhook-clusterrolebinding.yaml",
		"cluster-manager-hosted/cluster-manager-work-webhook-service.yaml",
		"cluster-manager-hosted/cluster-manager-work-webhook-serviceaccount.yaml",
		"cluster-manager-hosted/cluster-manager-work-webhook-apiservice.yaml",
		"cluster-manager-hosted/cluster-manager-work-webhook-validatingconfiguration.yaml",
		"cluster-manager-hosted/cluster-manager-placement-clusterrole.yaml",
		"cluster-manager-hosted/cluster-manager-placement-clusterrolebinding.yaml",
		"cluster-manager-hosted/cluster-manager-placement-serviceaccount.yaml",
	}

	webhookServiceHosted = []string{
		"cluster-manager-hosted/cluster-manager-registration-webhook-service-hosted.yaml",
		"cluster-manager-hosted/cluster-manager-work-webhook-service-hosted.yaml",
	}

	deploymentFilesHosted = []string{
		"cluster-manager-hosted/cluster-manager-registration-deployment.yaml",
		"cluster-manager-hosted/cluster-manager-registration-webhook-deployment.yaml",
		"cluster-manager-hosted/cluster-manager-work-webhook-deployment.yaml",
		"cluster-manager-hosted/cluster-manager-placement-deployment.yaml",
	}
)

const (
	clusterManagerFinalizer = "operator.open-cluster-management.io/cluster-manager-cleanup"
	clusterManagerApplied   = "Applied"
	clusterManagerAvailable = "Available"
	caBundleConfigmap       = "ca-bundle-configmap"
)

type clusterManagerController struct {
	clusterManagerClient  operatorv1client.ClusterManagerInterface
	clusterManagerLister  operatorlister.ClusterManagerLister
	kubeClient            kubernetes.Interface
	apiExtensionClient    apiextensionsclient.Interface
	apiRegistrationClient apiregistrationclient.APIServicesGetter
	currentGeneration     []int64
	configMapLister       corev1listers.ConfigMapLister
}

// NewClusterManagerController construct cluster manager hub controller
func NewClusterManagerController(
	kubeClient kubernetes.Interface,
	apiExtensionClient apiextensionsclient.Interface,
	apiRegistrationClient apiregistrationclient.APIServicesGetter,
	clusterManagerClient operatorv1client.ClusterManagerInterface,
	clusterManagerInformer operatorinformer.ClusterManagerInformer,
	deploymentInformer appsinformer.DeploymentInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	recorder events.Recorder) factory.Controller {
	controller := &clusterManagerController{
		kubeClient:            kubeClient,
		apiExtensionClient:    apiExtensionClient,
		apiRegistrationClient: apiRegistrationClient,
		clusterManagerClient:  clusterManagerClient,
		clusterManagerLister:  clusterManagerInformer.Lister(),
		configMapLister:       configMapInformer.Lister(),
		currentGeneration:     make([]int64, len(deploymentFiles)),
	}

	return factory.New().WithSync(controller.sync).
		ResyncEvery(3*time.Minute).
		WithInformersQueueKeyFunc(helpers.ClusterManagerDeploymentQueueKeyFunc(controller.clusterManagerLister), deploymentInformer.Informer()).
		WithFilteredEventsInformersQueueKeyFunc(
			helpers.ClusterManagerConfigmapQueueKeyFunc(controller.clusterManagerLister),
			func(obj interface{}) bool {
				accessor, _ := meta.Accessor(obj)
				if namespace := accessor.GetNamespace(); !helpers.IsClusterManagerNamespace(namespace) {
					return false
				}
				if name := accessor.GetName(); name != caBundleConfigmap {
					return false
				}
				return true
			},
			configMapInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			return accessor.GetName()
		}, clusterManagerInformer.Informer()).
		ToController("ClusterManagerController", recorder)
}

// hubConfig is used to render the template of hub manifests
type hubConfig struct {
	ClusterManagerName             string
	ClusterManagerNamespace        string
	RegistrationImage              string
	RegistrationAPIServiceCABundle string
	WorkImage                      string
	WorkAPIServiceCABundle         string
	PlacementImage                 string
	Replica                        int32
}

func (n *clusterManagerController) sync(ctx context.Context, controllerContext factory.SyncContext) error {
	clusterManagerName := controllerContext.QueueKey()
	klog.Infof("Reconciling ClusterManager %q", clusterManagerName)

	clusterManager, err := n.clusterManagerLister.Get(clusterManagerName)
	if errors.IsNotFound(err) {
		// ClusterManager not found, could have been deleted, do nothing.
		return nil
	}
	if err != nil {
		return err
	}
	clusterManager = clusterManager.DeepCopy()

	// Update finalizer at first
	if clusterManager.DeletionTimestamp.IsZero() {
		hasFinalizer := false
		for i := range clusterManager.Finalizers {
			if clusterManager.Finalizers[i] == clusterManagerFinalizer {
				hasFinalizer = true
				break
			}
		}
		if !hasFinalizer {
			clusterManager.Finalizers = append(clusterManager.Finalizers, clusterManagerFinalizer)
			_, err := n.clusterManagerClient.Update(ctx, clusterManager, metav1.UpdateOptions{})
			return err
		}
	}

	config := hubConfig{
		ClusterManagerName:      clusterManager.Name,
		ClusterManagerNamespace: helpers.ClusterManagerNamespace(clusterManager.Name, clusterManager.Spec.DeployOption.Mode),
		RegistrationImage:       clusterManager.Spec.RegistrationImagePullSpec,
		WorkImage:               clusterManager.Spec.WorkImagePullSpec,
		PlacementImage:          clusterManager.Spec.PlacementImagePullSpec,
		Replica:                 helpers.DetermineReplicaByNodes(ctx, n.kubeClient),
	}

	currentGenerations := []operatorapiv1.GenerationStatus{}
	errs := []error{}

	// get the mode of the clustermanager
	if clusterManager.Spec.DeployOption.Mode == helpers.DeployModeHosted {
		// in Hosted mode

		// get clients of external-hub-cluster
		externalHubKubeconfig, err := helpers.GetExternalKubeconfig(ctx, n.kubeClient, clusterManagerName)
		if err != nil {
			return err
		}
		externalClient, err := kubernetes.NewForConfig(externalHubKubeconfig)
		if err != nil {
			return err
		}
		externalAPIExtensionClient, err := apiextensionsclient.NewForConfig(externalHubKubeconfig)
		if err != nil {
			return err
		}
		externalAPIRegistrationClient, err := apiregistrationclient.NewForConfig(externalHubKubeconfig)
		if err != nil {
			return err
		}

		// ClusterManager is deleting, we remove its related resources on hub
		if !clusterManager.DeletionTimestamp.IsZero() {
			if err := cleanUp(ctx, controllerContext, config, externalClient, externalAPIExtensionClient, externalAPIRegistrationClient); err != nil {
				return err
			}
			return n.removeClusterManagerFinalizer(ctx, clusterManager)
		}

		// try to load ca bundle from configmap
		clustermanagerNamespace := helpers.ClusterManagerNamespace(clusterManagerName, clusterManager.Spec.DeployOption.Mode)
		caBundle := "placeholder"
		configmap, err := n.configMapLister.ConfigMaps(clustermanagerNamespace).Get(caBundleConfigmap)
		switch {
		case errors.IsNotFound(err):
			// do nothing
			klog.Infof("no cabundle in %s namespace", clustermanagerNamespace)
		case err != nil:
			return err
		default:
			if cb := configmap.Data["ca-bundle.crt"]; len(cb) > 0 {
				caBundle = cb
			}
		}
		encodedCaBundle := base64.StdEncoding.EncodeToString([]byte(caBundle))
		config.RegistrationAPIServiceCABundle = encodedCaBundle
		config.WorkAPIServiceCABundle = encodedCaBundle

		// Apply static files
		resourceResults := helpers.ApplyDirectly(
			externalClient,
			externalAPIExtensionClient,
			externalAPIRegistrationClient,
			controllerContext.Recorder(),
			func(name string) ([]byte, error) {
				template, err := manifests.ClusterManagerHostedManifestFiles.ReadFile(name)
				if err != nil {
					return nil, err
				}
				return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
			},
			staticResourceFilesHostedMode...,
		)

		for _, result := range resourceResults {
			if result.Error != nil {
				errs = append(errs, fmt.Errorf("%q (%T): %v", result.File, result.Type, result.Error))
			}
		}

		// Apply service to export webhook TODO NodePort for now, need a better solution later
		webhookResourceResults := helpers.ApplyDirectly(
			n.kubeClient,
			n.apiExtensionClient,
			n.apiRegistrationClient,
			controllerContext.Recorder(),
			func(name string) ([]byte, error) {
				template, err := manifests.ClusterManagerHostedManifestFiles.ReadFile(name)
				if err != nil {
					return nil, err
				}
				return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
			},
			webhookServiceHosted...,
		)

		for _, result := range webhookResourceResults {
			if result.Error != nil {
				errs = append(errs, fmt.Errorf("%q (%T): %v", result.File, result.Type, result.Error))
			}
		}

		// Render deployment manifest and apply
		for _, file := range deploymentFilesHosted {
			currentGeneration, err := helpers.ApplyDeployment(
				n.kubeClient,
				clusterManager.Status.Generations,
				clusterManager.Spec.NodePlacement,
				func(name string) ([]byte, error) {
					template, err := manifests.ClusterManagerHostedManifestFiles.ReadFile(name)
					if err != nil {
						return nil, err
					}
					return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
				},
				controllerContext.Recorder(),
				file)
			if err != nil {
				errs = append(errs, err)
			}
			currentGenerations = append(currentGenerations, currentGeneration)
		}
	} else {
		// in Default mode

		// ClusterManager is deleting, we remove its related resources on hub
		if !clusterManager.DeletionTimestamp.IsZero() {
			if err := cleanUp(ctx, controllerContext, config, n.kubeClient, n.apiExtensionClient, n.apiRegistrationClient); err != nil {
				return err
			}
			return n.removeClusterManagerFinalizer(ctx, clusterManager)
		}

		// try to load ca bundle from configmap
		caBundle := "placeholder"
		configmap, err := n.configMapLister.ConfigMaps(helpers.ClusterManagerNamespace(clusterManagerName, clusterManager.Spec.DeployOption.Mode)).Get(caBundleConfigmap)
		switch {
		case errors.IsNotFound(err):
			// do nothing
		case err != nil:
			return err
		default:
			if cb := configmap.Data["ca-bundle.crt"]; len(cb) > 0 {
				caBundle = cb
			}
		}
		encodedCaBundle := base64.StdEncoding.EncodeToString([]byte(caBundle))
		config.RegistrationAPIServiceCABundle = encodedCaBundle
		config.WorkAPIServiceCABundle = encodedCaBundle

		// Apply static files
		resourceResults := helpers.ApplyDirectly(
			n.kubeClient,
			n.apiExtensionClient,
			n.apiRegistrationClient,
			controllerContext.Recorder(),
			func(name string) ([]byte, error) {
				template, err := manifests.ClusterManagerManifestFiles.ReadFile(name)
				if err != nil {
					return nil, err
				}
				return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
			},
			staticResourceFiles...,
		)

		for _, result := range resourceResults {
			if result.Error != nil {
				errs = append(errs, fmt.Errorf("%q (%T): %v", result.File, result.Type, result.Error))
			}
		}

		// Render deployment manifest and apply
		for _, file := range deploymentFiles {
			currentGeneration, err := helpers.ApplyDeployment(
				n.kubeClient,
				clusterManager.Status.Generations,
				clusterManager.Spec.NodePlacement,
				func(name string) ([]byte, error) {
					template, err := manifests.ClusterManagerManifestFiles.ReadFile(name)
					if err != nil {
						return nil, err
					}
					return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
				},
				controllerContext.Recorder(),
				file)
			if err != nil {
				errs = append(errs, err)
			}
			currentGenerations = append(currentGenerations, currentGeneration)
		}
	}

	// Set status conditions
	conditions := &clusterManager.Status.Conditions
	observedKlusterletGeneration := clusterManager.Status.ObservedGeneration
	if len(errs) == 0 {
		meta.SetStatusCondition(conditions, metav1.Condition{
			Type:    clusterManagerApplied,
			Status:  metav1.ConditionTrue,
			Reason:  "ClusterManagerApplied",
			Message: "Components of cluster manager is applied",
		})
		observedKlusterletGeneration = clusterManager.Generation
	} else {
		meta.SetStatusCondition(conditions, metav1.Condition{
			Type:    clusterManagerApplied,
			Status:  metav1.ConditionFalse,
			Reason:  "ClusterManagerApplyFailed",
			Message: "Components of cluster manager fail to be applied",
		})
	}

	// Update status
	_, _, updatedErr := helpers.UpdateClusterManagerStatus(
		ctx, n.clusterManagerClient, clusterManager.Name,
		helpers.UpdateClusterManagerConditionFn(*conditions...),
		helpers.UpdateClusterManagerGenerationsFn(currentGenerations...),
		func(oldStatus *operatorapiv1.ClusterManagerStatus) error {
			oldStatus.ObservedGeneration = observedKlusterletGeneration
			return nil
		},
	)
	if updatedErr != nil {
		errs = append(errs, updatedErr)
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

func (n *clusterManagerController) removeClusterManagerFinalizer(ctx context.Context, deploy *operatorapiv1.ClusterManager) error {
	copiedFinalizers := []string{}
	for i := range deploy.Finalizers {
		if deploy.Finalizers[i] == clusterManagerFinalizer {
			continue
		}
		copiedFinalizers = append(copiedFinalizers, deploy.Finalizers[i])
	}

	if len(deploy.Finalizers) != len(copiedFinalizers) {
		deploy.Finalizers = copiedFinalizers
		_, err := n.clusterManagerClient.Update(ctx, deploy, metav1.UpdateOptions{})
		return err
	}

	return nil
}

// removeCRD removes crd, and check if crd resource is removed. Since the related cr is still being deleted,
// it will check the crd existence after deletion, and only return nil when crd is not found.
func removeCRD(ctx context.Context, name string, apiExtensionClient apiextensionsclient.Interface) error {
	err := apiExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Delete(
		ctx, name, metav1.DeleteOptions{})
	switch {
	case errors.IsNotFound(err):
		return nil
	case err != nil:
		return err
	}

	_, err = apiExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, name, metav1.GetOptions{})
	switch {
	case errors.IsNotFound(err):
		return nil
	case err != nil:
		return err
	}

	return fmt.Errorf("CRD %s is still being deleted", name)
}

func cleanUp(ctx context.Context, controllerContext factory.SyncContext, config hubConfig,
	kubeClient kubernetes.Interface, apiExtensionClient apiextensionsclient.Interface,
	apiRegistrationClient apiregistrationclient.APIServicesGetter) error {
	// Remove crd
	for _, name := range crdNames {
		err := removeCRD(ctx, name, apiExtensionClient)
		if err != nil {
			return err
		}
		controllerContext.Recorder().Eventf("CRDDeleted", "crd %s is deleted", name)
	}

	// Remove Static files
	for _, file := range staticResourceFiles {
		err := helpers.CleanUpStaticObject(
			ctx,
			kubeClient,
			apiExtensionClient,
			apiRegistrationClient,
			func(name string) ([]byte, error) {
				template, err := manifests.ClusterManagerManifestFiles.ReadFile(name)
				if err != nil {
					return nil, err
				}
				return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
			},
			file,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
