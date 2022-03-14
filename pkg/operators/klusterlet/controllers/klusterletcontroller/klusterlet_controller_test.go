package klusterletcontroller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	fakeapiextensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/version"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	fakekube "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	"k8s.io/klog/v2"

	fakeoperatorclient "open-cluster-management.io/api/client/operator/clientset/versioned/fake"
	operatorinformers "open-cluster-management.io/api/client/operator/informers/externalversions"
	fakeworkclient "open-cluster-management.io/api/client/work/clientset/versioned/fake"
	opratorapiv1 "open-cluster-management.io/api/operator/v1"
	workapiv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/registration-operator/pkg/helpers"
	testinghelper "open-cluster-management.io/registration-operator/pkg/helpers/testing"
)

type testController struct {
	controller         *klusterletController
	kubeClient         *fakekube.Clientset
	apiExtensionClient *fakeapiextensions.Clientset
	operatorClient     *fakeoperatorclient.Clientset
	dynamicClient      *fakedynamic.FakeDynamicClient
	workClient         *fakeworkclient.Clientset
	operatorStore      cache.Store

	managedKubeClient         *fakekube.Clientset
	managedApiExtensionClient *fakeapiextensions.Clientset
	managedWorkClient         *fakeworkclient.Clientset
}

func newSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{},
	}
}

func newServiceAccountSecret(name, namespace string) *corev1.Secret {
	secret := newSecret(name, namespace)
	secret.Data["token"] = []byte("test-token")
	secret.Type = corev1.SecretTypeServiceAccountToken
	return secret
}

func newKlusterlet(name, namespace, clustername string) *opratorapiv1.Klusterlet {
	return &opratorapiv1.Klusterlet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Finalizers: []string{klusterletFinalizer},
		},
		Spec: opratorapiv1.KlusterletSpec{
			RegistrationImagePullSpec: "testregistration",
			WorkImagePullSpec:         "testwork",
			ClusterName:               clustername,
			Namespace:                 namespace,
			ExternalServerURLs:        []opratorapiv1.ServerURL{},
		},
	}
}

func newKlusterletDetached(name, namespace, clustername string) *opratorapiv1.Klusterlet {
	klusterlet := newKlusterlet(name, namespace, clustername)
	klusterlet.Spec.DeployOption.Mode = opratorapiv1.InstallModeDetached
	return klusterlet
}

func newNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func newNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node-role.kubernetes.io/master": "",
			},
		},
	}
}

func newServiceAccount(name, namespace string, referenceSecret string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Secrets: []corev1.ObjectReference{
			{
				Name:      referenceSecret,
				Namespace: namespace,
			},
		},
	}
}

func newTestController(klusterlet *opratorapiv1.Klusterlet, appliedManifestWorks []runtime.Object, objects ...runtime.Object) *testController {
	fakeKubeClient := fakekube.NewSimpleClientset(objects...)
	fakeAPIExtensionClient := fakeapiextensions.NewSimpleClientset()
	fakeOperatorClient := fakeoperatorclient.NewSimpleClientset(klusterlet)
	fakeWorkClient := fakeworkclient.NewSimpleClientset(appliedManifestWorks...)
	operatorInformers := operatorinformers.NewSharedInformerFactory(fakeOperatorClient, 5*time.Minute)
	kubeVersion, _ := version.ParseGeneric("v1.18.0")

	scheme := runtime.NewScheme()
	dynamicClient := fakedynamic.NewSimpleDynamicClient(scheme)

	hubController := &klusterletController{
		klusterletClient:          fakeOperatorClient.OperatorV1().Klusterlets(),
		kubeClient:                fakeKubeClient,
		apiExtensionClient:        fakeAPIExtensionClient,
		dynamicClient:             dynamicClient,
		appliedManifestWorkClient: fakeWorkClient.WorkV1().AppliedManifestWorks(),
		klusterletLister:          operatorInformers.Operator().V1().Klusterlets().Lister(),
		kubeVersion:               kubeVersion,
		operatorNamespace:         "open-cluster-management",
		cache:                     resourceapply.NewResourceCache(),
	}

	store := operatorInformers.Operator().V1().Klusterlets().Informer().GetStore()
	store.Add(klusterlet)

	return &testController{
		controller:         hubController,
		kubeClient:         fakeKubeClient,
		apiExtensionClient: fakeAPIExtensionClient,
		dynamicClient:      dynamicClient,
		operatorClient:     fakeOperatorClient,
		workClient:         fakeWorkClient,
		operatorStore:      store,
	}
}

func newTestControllerDetached(klusterlet *opratorapiv1.Klusterlet, appliedManifestWorks []runtime.Object, objects ...runtime.Object) *testController {
	fakeKubeClient := fakekube.NewSimpleClientset(objects...)
	fakeAPIExtensionClient := fakeapiextensions.NewSimpleClientset()
	fakeOperatorClient := fakeoperatorclient.NewSimpleClientset(klusterlet)
	fakeWorkClient := fakeworkclient.NewSimpleClientset()
	operatorInformers := operatorinformers.NewSharedInformerFactory(fakeOperatorClient, 5*time.Minute)
	kubeVersion, _ := version.ParseGeneric("v1.18.0")

	installedNamespace := helpers.KlusterletNamespace(klusterlet)
	saRegistrationSecret := newServiceAccountSecret(fmt.Sprintf("%s-token", registrationServiceAccountName(klusterlet.Name)), klusterlet.Name)
	saWorkSecret := newServiceAccountSecret(fmt.Sprintf("%s-token", workServiceAccountName(klusterlet.Name)), klusterlet.Name)
	fakeManagedKubeClient := fakekube.NewSimpleClientset()
	getRegistrationServiceAccountCount := 0
	getWorkServiceAccountCount := 0
	// fake the get serviceaccount, since there is no kubernetes controller to create service account related secret.
	// count the number of getting serviceaccount:
	// - the first call will return empty(Apply service account will invoke GetServiceAccount firstly),return empty will let the Apply service account to creation.
	// - After the first one, we will return a service account with secrets.
	fakeManagedKubeClient.PrependReactor("get", "serviceaccounts", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(clienttesting.GetAction).GetName()
		namespace := action.(clienttesting.GetAction).GetNamespace()
		if namespace == installedNamespace && name == registrationServiceAccountName(klusterlet.Name) {
			getRegistrationServiceAccountCount++
			if getRegistrationServiceAccountCount > 1 {
				sa := newServiceAccount(name, installedNamespace, saRegistrationSecret.Name)
				klog.Infof("return service account %s/%s, secret: %v", installedNamespace, name, sa.Secrets)
				return true, sa, nil
			}
		}

		if namespace == installedNamespace && name == workServiceAccountName(klusterlet.Name) {
			getWorkServiceAccountCount++
			if getWorkServiceAccountCount > 1 {
				sa := newServiceAccount(name, installedNamespace, saWorkSecret.Name)
				klog.Infof("return service account %s/%s, secret: %v", installedNamespace, name, sa.Secrets)
				return true, sa, nil
			}
		}
		return false, nil, nil
	})
	fakeManagedKubeClient.PrependReactor("get", "secrets", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(clienttesting.GetAction).GetName()
		namespace := action.(clienttesting.GetAction).GetNamespace()
		if namespace == installedNamespace && name == saRegistrationSecret.Name {
			return true, saRegistrationSecret, nil
		}
		if namespace == installedNamespace && name == saWorkSecret.Name {
			return true, saWorkSecret, nil
		}
		return false, nil, nil
	})

	fakeManagedAPIExtensionClient := fakeapiextensions.NewSimpleClientset()
	fakeManagedWorkClient := fakeworkclient.NewSimpleClientset(appliedManifestWorks...)
	hubController := &klusterletController{
		klusterletClient:          fakeOperatorClient.OperatorV1().Klusterlets(),
		kubeClient:                fakeKubeClient,
		apiExtensionClient:        fakeAPIExtensionClient,
		appliedManifestWorkClient: fakeWorkClient.WorkV1().AppliedManifestWorks(),
		klusterletLister:          operatorInformers.Operator().V1().Klusterlets().Lister(),
		kubeVersion:               kubeVersion,
		operatorNamespace:         "open-cluster-management",
		cache:                     resourceapply.NewResourceCache(),
		buildManagedClusterClientsDetachedMode: func(ctx context.Context, kubeClient kubernetes.Interface, namespace, secret string) (*managedClusterClients, error) {
			return &managedClusterClients{
				kubeClient:                fakeManagedKubeClient,
				apiExtensionClient:        fakeManagedAPIExtensionClient,
				appliedManifestWorkClient: fakeManagedWorkClient.WorkV1().AppliedManifestWorks(),
				kubeconfig: &rest.Config{
					Host: "testhost",
					TLSClientConfig: rest.TLSClientConfig{
						CAData: []byte("test"),
					},
				},
			}, nil
		},
	}

	store := operatorInformers.Operator().V1().Klusterlets().Informer().GetStore()
	store.Add(klusterlet)

	return &testController{
		controller:         hubController,
		kubeClient:         fakeKubeClient,
		apiExtensionClient: fakeAPIExtensionClient,
		operatorClient:     fakeOperatorClient,
		workClient:         fakeWorkClient,
		operatorStore:      store,

		managedKubeClient:         fakeManagedKubeClient,
		managedApiExtensionClient: fakeManagedAPIExtensionClient,
		managedWorkClient:         fakeManagedWorkClient,
	}
}

func getDeployments(actions []clienttesting.Action, verb, suffix string) *appsv1.Deployment {

	deployments := []*appsv1.Deployment{}
	for _, action := range actions {
		if action.GetVerb() != verb || action.GetResource().Resource != "deployments" {
			continue
		}

		if verb == "create" {
			object := action.(clienttesting.CreateActionImpl).Object
			deployments = append(deployments, object.(*appsv1.Deployment))
		}

		if verb == "update" {
			object := action.(clienttesting.UpdateActionImpl).Object
			deployments = append(deployments, object.(*appsv1.Deployment))
		}
	}

	for _, deployment := range deployments {
		if strings.HasSuffix(deployment.Name, suffix) {
			return deployment
		}
	}

	return nil
}

func assertRegistrationDeployment(t *testing.T, actions []clienttesting.Action, verb, serverURL, clusterName string, replica int32) {
	deployment := getDeployments(actions, verb, "registration-agent")
	if deployment == nil {
		t.Errorf("registration deployment not found")
	}
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("Expect 1 containers in deployment spec, actual %d", len(deployment.Spec.Template.Spec.Containers))
	}

	args := deployment.Spec.Template.Spec.Containers[0].Args
	expectedArgs := []string{
		"/registration",
		"agent",
		fmt.Sprintf("--cluster-name=%s", clusterName),
		"--bootstrap-kubeconfig=/spoke/bootstrap/kubeconfig",
		"--feature-gates=AddonManagement=true",
	}

	if serverURL != "" {
		expectedArgs = append(expectedArgs, fmt.Sprintf("--spoke-external-server-urls=%s", serverURL))
	}

	if *deployment.Spec.Replicas == 1 {
		expectedArgs = append(expectedArgs, "--disable-leader-election")
	}

	if !equality.Semantic.DeepEqual(args, expectedArgs) {
		t.Errorf("Expect args %v, but got %v", expectedArgs, args)
	}

	if *deployment.Spec.Replicas != replica {
		t.Errorf("Unexpected registration replica, expect %d, got %d", replica, *deployment.Spec.Replicas)
	}
}

func assertWorkDeployment(t *testing.T, actions []clienttesting.Action, verb, clusterName string, mode opratorapiv1.InstallMode, replica int32) {
	deployment := getDeployments(actions, verb, "work-agent")
	if deployment == nil {
		t.Errorf("work deployment not found")
	}
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("Expect 1 containers in deployment spec, actual %d", len(deployment.Spec.Template.Spec.Containers))
	}
	args := deployment.Spec.Template.Spec.Containers[0].Args
	expectArgs := []string{
		"/work",
		"agent",
		fmt.Sprintf("--spoke-cluster-name=%s", clusterName),
		"--hub-kubeconfig=/spoke/hub-kubeconfig/kubeconfig",
	}

	if mode == opratorapiv1.InstallModeDetached {
		expectArgs = append(expectArgs, "--spoke-kubeconfig=/spoke/config/kubeconfig")
	}

	if *deployment.Spec.Replicas == 1 {
		expectArgs = append(expectArgs, "--disable-leader-election")
	}

	if !equality.Semantic.DeepEqual(args, expectArgs) {
		t.Errorf("Expect args %v, but got %v", expectArgs, args)
	}
	if *deployment.Spec.Replicas != replica {
		t.Errorf("Unexpected registration replica, expect %d, got %d", replica, *deployment.Spec.Replicas)
	}
}

func ensureObject(t *testing.T, object runtime.Object, klusterlet *opratorapiv1.Klusterlet) {
	access, err := meta.Accessor(object)
	if err != nil {
		t.Errorf("Unable to access objectmeta: %v", err)
	}

	namespace := helpers.KlusterletNamespace(klusterlet)
	switch o := object.(type) {
	case *appsv1.Deployment:
		if strings.Contains(access.GetName(), "registration") {
			testinghelper.AssertEqualNameNamespace(
				t, access.GetName(), access.GetNamespace(),
				fmt.Sprintf("%s-registration-agent", klusterlet.Name), namespace)
			if klusterlet.Spec.RegistrationImagePullSpec != o.Spec.Template.Spec.Containers[0].Image {
				t.Errorf("Image does not match to the expected.")
			}
		} else if strings.Contains(access.GetName(), "work") {
			testinghelper.AssertEqualNameNamespace(
				t, access.GetName(), access.GetNamespace(),
				fmt.Sprintf("%s-work-agent", klusterlet.Name), namespace)
			if klusterlet.Spec.WorkImagePullSpec != o.Spec.Template.Spec.Containers[0].Image {
				t.Errorf("Image does not match to the expected.")
			}
		} else {
			t.Errorf("Unexpected deployment")
		}
	}
}

// TestSyncDeploy test deployment of klusterlet components
func TestSyncDeploy(t *testing.T) {
	klusterlet := newKlusterlet("klusterlet", "testns", "cluster1")
	bootStrapSecret := newSecret(helpers.BootstrapHubKubeConfig, "testns")
	hubKubeConfigSecret := newSecret(helpers.HubKubeConfig, "testns")
	hubKubeConfigSecret.Data["kubeconfig"] = []byte("dummuykubeconnfig")
	namespace := newNamespace("testns")
	controller := newTestController(klusterlet, nil, bootStrapSecret, hubKubeConfigSecret, namespace)
	syncContext := testinghelper.NewFakeSyncContext(t, "klusterlet")

	err := controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}

	createObjects := []runtime.Object{}
	kubeActions := controller.kubeClient.Actions()
	for _, action := range kubeActions {
		if action.GetVerb() == "create" {
			object := action.(clienttesting.CreateActionImpl).Object
			createObjects = append(createObjects, object)

		}
	}

	// Check if resources are created as expected
	// 7 managed static manifests + 8 management static manifests - 2 duplicated service account manifests + 1 addon namespace + 2 deployments
	if len(createObjects) != 16 {
		t.Errorf("Expect 16 objects created in the sync loop, actual %d", len(createObjects))
	}
	for _, object := range createObjects {
		ensureObject(t, object, klusterlet)
	}

	apiExtenstionAction := controller.apiExtensionClient.Actions()
	createCRDObjects := []runtime.Object{}
	for _, action := range apiExtenstionAction {
		if action.GetVerb() == "create" && action.GetResource().Resource == "customresourcedefinitions" {
			object := action.(clienttesting.CreateActionImpl).Object
			createCRDObjects = append(createCRDObjects, object)
		}
	}
	if len(createCRDObjects) != 2 {
		t.Errorf("Expect 2 objects created in the sync loop, actual %d", len(createCRDObjects))
	}

	operatorAction := controller.operatorClient.Actions()
	if len(operatorAction) != 2 {
		t.Errorf("Expect 2 actions in the sync loop, actual %#v", operatorAction)
	}

	testinghelper.AssertGet(t, operatorAction[0], "operator.open-cluster-management.io", "v1", "klusterlets")
	testinghelper.AssertAction(t, operatorAction[1], "update")
	testinghelper.AssertOnlyConditions(
		t, operatorAction[1].(clienttesting.UpdateActionImpl).Object,
		testinghelper.NamedCondition(klusterletApplied, "KlusterletApplied", metav1.ConditionTrue))
}

// TestSyncDeployDetached test deployment of klusterlet components in detached mode
func TestSyncDeployDetached(t *testing.T) {
	installedNamespace := "klusterlet"
	klusterlet := newKlusterletDetached("klusterlet", "testns", "cluster1")
	bootStrapSecret := newSecret(helpers.BootstrapHubKubeConfig, installedNamespace)
	hubKubeConfigSecret := newSecret(helpers.HubKubeConfig, installedNamespace)
	hubKubeConfigSecret.Data["kubeconfig"] = []byte("dummuykubeconnfig")
	// externalManagedSecret := newSecret(helpers.ExternalManagedKubeConfig, installedNamespace)
	// externalManagedSecret.Data["kubeconfig"] = []byte("dummuykubeconnfig")
	namespace := newNamespace(installedNamespace)
	pullSecret := newSecret(imagePullSecret, "open-cluster-management")
	controller := newTestControllerDetached(klusterlet, nil, bootStrapSecret, hubKubeConfigSecret, namespace, pullSecret /*externalManagedSecret*/)
	syncContext := testinghelper.NewFakeSyncContext(t, "klusterlet")

	err := controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}

	createObjectsManagement := []runtime.Object{}
	kubeActions := controller.kubeClient.Actions()
	for _, action := range kubeActions {
		if action.GetVerb() == "create" {
			object := action.(clienttesting.CreateActionImpl).Object
			klog.Infof("management kube create: %v\t resource:%v \t namespace:%v", object.GetObjectKind(), action.GetResource(), action.GetNamespace())
			createObjectsManagement = append(createObjectsManagement, object)
		}
	}
	// Check if resources are created as expected on the management cluster
	// 8 static manifests + 2 secrets(external-managed-kubeconfig-registration,external-managed-kubeconfig-work) + 2 deployments(registration-agent,work-agent) + 1 pull secret
	if len(createObjectsManagement) != 13 {
		t.Errorf("Expect 13 objects created in the sync loop, actual %d", len(createObjectsManagement))
	}
	for _, object := range createObjectsManagement {
		ensureObject(t, object, klusterlet)
	}

	createObjectsManaged := []runtime.Object{}
	for _, action := range controller.managedKubeClient.Actions() {
		if action.GetVerb() == "create" {

			object := action.(clienttesting.CreateActionImpl).Object
			klog.Infof("managed kube create: %v\t resource:%v \t namespace:%v", object.GetObjectKind().GroupVersionKind(), action.GetResource(), action.GetNamespace())
			createObjectsManaged = append(createObjectsManaged, object)
		}
	}
	// Check if resources are created as expected on the managed cluster
	// 7 static manifests + 2 namespaces + 1 pull secret in the addon namespace
	if len(createObjectsManaged) != 10 {
		t.Errorf("Expect 9 objects created in the sync loop, actual %d", len(createObjectsManaged))
	}
	for _, object := range createObjectsManaged {
		ensureObject(t, object, klusterlet)
	}

	apiExtenstionAction := controller.apiExtensionClient.Actions()
	createCRDObjects := []runtime.Object{}
	for _, action := range apiExtenstionAction {
		if action.GetVerb() == "create" && action.GetResource().Resource == "customresourcedefinitions" {
			object := action.(clienttesting.CreateActionImpl).Object
			createCRDObjects = append(createCRDObjects, object)
		}
	}
	if len(createCRDObjects) != 0 {
		t.Errorf("Expect 0 objects created in the sync loop, actual %d", len(createCRDObjects))
	}

	createCRDObjectsManaged := []runtime.Object{}
	for _, action := range controller.managedApiExtensionClient.Actions() {
		if action.GetVerb() == "create" && action.GetResource().Resource == "customresourcedefinitions" {
			object := action.(clienttesting.CreateActionImpl).Object
			createCRDObjectsManaged = append(createCRDObjectsManaged, object)
		}
	}
	if len(createCRDObjectsManaged) != 2 {
		t.Errorf("Expect 2 objects created in the sync loop, actual %d", len(createCRDObjectsManaged))
	}

	operatorAction := controller.operatorClient.Actions()
	for _, action := range operatorAction {
		klog.Infof("operator actions, verb:%v \t resource:%v \t namespace:%v", action.GetVerb(), action.GetResource(), action.GetNamespace())
	}

	if len(operatorAction) != 4 {
		t.Errorf("Expect 4 actions in the sync loop, actual %#v", len(operatorAction))
	}

	testinghelper.AssertGet(t, operatorAction[0], "operator.open-cluster-management.io", "v1", "klusterlets")
	testinghelper.AssertAction(t, operatorAction[1], "update")
	testinghelper.AssertGet(t, operatorAction[2], "operator.open-cluster-management.io", "v1", "klusterlets")
	testinghelper.AssertAction(t, operatorAction[3], "update")

	conditionReady := testinghelper.NamedCondition(klusterletReadyToApply, "KlusterletPrepared", metav1.ConditionTrue)
	conditionApplied := testinghelper.NamedCondition(klusterletApplied, "KlusterletApplied", metav1.ConditionTrue)
	testinghelper.AssertOnlyConditions(
		t, operatorAction[1].(clienttesting.UpdateActionImpl).Object, conditionReady)
	testinghelper.AssertOnlyConditions(
		t, operatorAction[3].(clienttesting.UpdateActionImpl).Object, conditionReady, conditionApplied)
}

// TestSyncDelete test cleanup hub deploy
func TestSyncDelete(t *testing.T) {
	klusterlet := newKlusterlet("klusterlet", "testns", "")
	now := metav1.Now()
	klusterlet.ObjectMeta.SetDeletionTimestamp(&now)
	bootstrapKubeConfigSecret := newSecret(helpers.BootstrapHubKubeConfig, "testns")
	bootstrapKubeConfigSecret.Data["kubeconfig"] = newKubeConfig("testhost")
	namespace := newNamespace("testns")
	appliedManifestWorks := []runtime.Object{
		newAppliedManifestWorks("testhost", nil, false),
		newAppliedManifestWorks("testhost", []string{appliedManifestWorkFinalizer}, true),
		newAppliedManifestWorks("testhost-2", []string{appliedManifestWorkFinalizer}, false),
	}
	controller := newTestController(klusterlet, appliedManifestWorks, namespace, bootstrapKubeConfigSecret)
	syncContext := testinghelper.NewFakeSyncContext(t, "klusterlet")

	err := controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}

	deleteActions := []clienttesting.DeleteActionImpl{}
	kubeActions := controller.kubeClient.Actions()
	for _, action := range kubeActions {
		if action.GetVerb() == "delete" {
			deleteAction := action.(clienttesting.DeleteActionImpl)
			klog.Infof("kube delete name: %v\t resource:%v \t namespace:%v", deleteAction.Name, deleteAction.GetResource(), deleteAction.GetNamespace())
			deleteActions = append(deleteActions, deleteAction)
		}
	}

	// 7 managed static manifests + 8 management static manifests + 1 hub kubeconfig + 2 namespaces + 2 deployments
	if len(deleteActions) != 20 {
		t.Errorf("Expected 20 delete actions, but got %d", len(deleteActions))
	}

	deleteCRDActions := []clienttesting.DeleteActionImpl{}
	crdActions := controller.apiExtensionClient.Actions()
	for _, action := range crdActions {
		if action.GetVerb() == "delete" {
			deleteAction := action.(clienttesting.DeleteActionImpl)
			deleteCRDActions = append(deleteCRDActions, deleteAction)
		}
	}

	if len(deleteCRDActions) != 2 {
		t.Errorf("Expected 2 delete actions, but got %d", len(deleteCRDActions))
	}

	updateWorkActions := []clienttesting.UpdateActionImpl{}
	workActions := controller.workClient.Actions()
	for _, action := range workActions {
		if action.GetVerb() == "update" {
			updateAction := action.(clienttesting.UpdateActionImpl)
			updateWorkActions = append(updateWorkActions, updateAction)
			continue
		}
	}

	// update 1 appliedminifestwork to remove appliedManifestWorkFinalizer
	if len(updateWorkActions) != 1 {
		t.Errorf("Expected 1 update action, but got %d", len(updateWorkActions))
	}
}

func TestSyncDeleteDetached(t *testing.T) {
	klusterlet := newKlusterletDetached("klusterlet", "testns", "cluster1")
	now := metav1.Now()
	klusterlet.ObjectMeta.SetDeletionTimestamp(&now)
	installedNamespace := helpers.KlusterletNamespace(klusterlet)
	bootstrapKubeConfigSecret := newSecret(helpers.BootstrapHubKubeConfig, installedNamespace)
	bootstrapKubeConfigSecret.Data["kubeconfig"] = newKubeConfig("testhost")
	// externalManagedSecret := newSecret(helpers.ExternalManagedKubeConfig, installedNamespace)
	// externalManagedSecret.Data["kubeconfig"] = []byte("dummuykubeconnfig")
	namespace := newNamespace(installedNamespace)
	appliedManifestWorks := []runtime.Object{
		newAppliedManifestWorks("testhost", nil, false),
		newAppliedManifestWorks("testhost", []string{appliedManifestWorkFinalizer}, true),
		newAppliedManifestWorks("testhost-2", []string{appliedManifestWorkFinalizer}, false),
	}
	controller := newTestControllerDetached(klusterlet, appliedManifestWorks, bootstrapKubeConfigSecret, namespace /*externalManagedSecret*/)
	syncContext := testinghelper.NewFakeSyncContext(t, klusterlet.Name)

	err := controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}

	deleteActionsManagement := []clienttesting.DeleteActionImpl{}
	kubeActions := controller.kubeClient.Actions()
	for _, action := range kubeActions {
		if action.GetVerb() == "delete" {
			deleteAction := action.(clienttesting.DeleteActionImpl)
			klog.Infof("management kube delete name: %v\t resource:%v \t namespace:%v", deleteAction.Name, deleteAction.GetResource(), deleteAction.GetNamespace())
			deleteActionsManagement = append(deleteActionsManagement, deleteAction)
		}
	}

	// 8 static manifests + 3 secrets(hub-kubeconfig-secret, external-managed-kubeconfig-registration,external-managed-kubeconfig-work)
	// + 2 deployments(registration-agent,work-agent) + 1 namespace
	if len(deleteActionsManagement) != 14 {
		t.Errorf("Expected 14 delete actions, but got %d", len(deleteActionsManagement))
	}

	deleteActionsManaged := []clienttesting.DeleteActionImpl{}
	for _, action := range controller.managedKubeClient.Actions() {
		if action.GetVerb() == "delete" {
			deleteAction := action.(clienttesting.DeleteActionImpl)
			klog.Infof("managed kube delete name: %v\t resource:%v \t namespace:%v", deleteAction.Name, deleteAction.GetResource(), deleteAction.GetNamespace())
			deleteActionsManaged = append(deleteActionsManaged, deleteAction)
		}
	}

	// 7 static manifests + 2 namespaces
	if len(deleteActionsManaged) != 9 {
		t.Errorf("Expected 9 delete actions, but got %d", len(deleteActionsManaged))
	}

	deleteCRDActions := []clienttesting.DeleteActionImpl{}
	crdActions := controller.managedApiExtensionClient.Actions()
	for _, action := range crdActions {
		if action.GetVerb() == "delete" {
			deleteAction := action.(clienttesting.DeleteActionImpl)
			deleteCRDActions = append(deleteCRDActions, deleteAction)
		}
	}

	if len(deleteCRDActions) != 2 {
		t.Errorf("Expected 2 delete actions, but got %d", len(deleteCRDActions))
	}

	updateWorkActions := []clienttesting.UpdateActionImpl{}
	workActions := controller.managedWorkClient.Actions()
	for _, action := range workActions {
		if action.GetVerb() == "update" {
			updateAction := action.(clienttesting.UpdateActionImpl)
			updateWorkActions = append(updateWorkActions, updateAction)
			continue
		}
	}

	// update 1 appliedminifestwork to remove appliedManifestWorkFinalizer
	if len(updateWorkActions) != 1 {
		t.Errorf("Expected 1 update action, but got %d", len(updateWorkActions))
	}
}

// TestGetServersFromKlusterlet tests getServersFromKlusterlet func
func TestGetServersFromKlusterlet(t *testing.T) {
	cases := []struct {
		name     string
		servers  []string
		expected string
	}{
		{
			name:     "Null",
			servers:  nil,
			expected: "",
		},
		{
			name:     "Empty string",
			servers:  []string{},
			expected: "",
		},
		{
			name:     "Single server",
			servers:  []string{"https://server1"},
			expected: "https://server1",
		},
		{
			name:     "Multiple servers",
			servers:  []string{"https://server1", "https://server2"},
			expected: "https://server1,https://server2",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			klusterlet := newKlusterlet("klusterlet", "testns", "")
			for _, server := range c.servers {
				klusterlet.Spec.ExternalServerURLs = append(klusterlet.Spec.ExternalServerURLs,
					opratorapiv1.ServerURL{URL: server})
			}
			actual := getServersFromKlusterlet(klusterlet)
			if actual != c.expected {
				t.Errorf("Expected to be same, actual %q, expected %q", actual, c.expected)
			}
		})
	}
}

func TestReplica(t *testing.T) {
	klusterlet := newKlusterlet("klusterlet", "testns", "cluster1")
	hubSecret := newSecret(helpers.HubKubeConfig, "testns")
	hubSecret.Data["kubeconfig"] = []byte("dummuykubeconnfig")
	hubSecret.Data["cluster-name"] = []byte("cluster1")
	objects := []runtime.Object{
		newNamespace("testns"),
		newSecret(helpers.BootstrapHubKubeConfig, "testns"),
		hubSecret,
	}

	controller := newTestController(klusterlet, nil, objects...)
	syncContext := testinghelper.NewFakeSyncContext(t, "klusterlet")

	err := controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}

	// should have 1 replica for registration deployment and 0 for work
	assertRegistrationDeployment(t, controller.kubeClient.Actions(), "create", "", "cluster1", 1)
	assertWorkDeployment(t, controller.kubeClient.Actions(), "create", "cluster1", opratorapiv1.InstallModeDefault, 0)

	klusterlet = newKlusterlet("klusterlet", "testns", "cluster1")
	klusterlet.Status.Conditions = []metav1.Condition{
		{
			Type:   hubConnectionDegraded,
			Status: metav1.ConditionFalse,
		},
	}

	controller.operatorStore.Update(klusterlet)

	controller.kubeClient.ClearActions()
	controller.operatorClient.ClearActions()

	err = controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}

	// should have 1 replica for work
	assertWorkDeployment(t, controller.kubeClient.Actions(), "update", "cluster1", opratorapiv1.InstallModeDefault, 1)

	controller.kubeClient.PrependReactor("list", "nodes", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetVerb() != "list" {
			return false, nil, nil
		}

		nodes := &corev1.NodeList{Items: []corev1.Node{*newNode("master1"), *newNode("master2"), *newNode("master3")}}

		return true, nodes, nil
	})

	controller.kubeClient.ClearActions()
	controller.operatorClient.ClearActions()

	err = controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}

	assertRegistrationDeployment(t, controller.kubeClient.Actions(), "update", "", "cluster1", 3)
	assertWorkDeployment(t, controller.kubeClient.Actions(), "update", "cluster1", opratorapiv1.InstallModeDefault, 3)
}

func TestClusterNameChange(t *testing.T) {
	klusterlet := newKlusterlet("klusterlet", "testns", "cluster1")
	namespace := newNamespace("testns")
	bootStrapSecret := newSecret(helpers.BootstrapHubKubeConfig, "testns")
	hubSecret := newSecret(helpers.HubKubeConfig, "testns")
	hubSecret.Data["kubeconfig"] = []byte("dummuykubeconnfig")
	hubSecret.Data["cluster-name"] = []byte("cluster1")
	controller := newTestController(klusterlet, nil, bootStrapSecret, hubSecret, namespace)
	syncContext := testinghelper.NewFakeSyncContext(t, "klusterlet")

	err := controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}

	// Check if deployment has the right cluster name set
	assertRegistrationDeployment(t, controller.kubeClient.Actions(), "create", "", "cluster1", 1)

	operatorAction := controller.operatorClient.Actions()
	if len(operatorAction) != 2 {
		t.Errorf("Expect 2 actions in the sync loop, actual %#v", operatorAction)
	}

	testinghelper.AssertGet(t, operatorAction[0], "operator.open-cluster-management.io", "v1", "klusterlets")
	testinghelper.AssertAction(t, operatorAction[1], "update")
	updatedKlusterlet := operatorAction[1].(clienttesting.UpdateActionImpl).Object.(*opratorapiv1.Klusterlet)
	testinghelper.AssertOnlyGenerationStatuses(
		t, updatedKlusterlet,
		testinghelper.NamedDeploymentGenerationStatus("klusterlet-registration-agent", "testns", 0),
		testinghelper.NamedDeploymentGenerationStatus("klusterlet-work-agent", "testns", 0),
	)

	// Update klusterlet with unset cluster name and rerun sync
	controller.kubeClient.ClearActions()
	controller.operatorClient.ClearActions()
	klusterlet = newKlusterlet("klusterlet", "testns", "")
	klusterlet.Generation = 1
	controller.operatorStore.Update(klusterlet)

	err = controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}
	assertRegistrationDeployment(t, controller.kubeClient.Actions(), "update", "", "", 1)

	// Update hubconfigsecret and sync again
	hubSecret.Data["cluster-name"] = []byte("cluster2")
	controller.kubeClient.PrependReactor("get", "secrets", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetVerb() != "get" {
			return false, nil, nil
		}

		getAction := action.(clienttesting.GetActionImpl)
		if getAction.Name != helpers.HubKubeConfig {
			return false, nil, errors.NewNotFound(
				corev1.Resource("secrets"), helpers.HubKubeConfig)
		}
		return true, hubSecret, nil
	})
	controller.kubeClient.ClearActions()

	err = controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}
	assertWorkDeployment(t, controller.kubeClient.Actions(), "update", "cluster2", "", 0)

	// Update klusterlet with different cluster name and rerun sync
	klusterlet = newKlusterlet("klusterlet", "testns", "cluster3")
	klusterlet.Generation = 2
	klusterlet.Spec.ExternalServerURLs = []opratorapiv1.ServerURL{{URL: "https://localhost"}}
	controller.kubeClient.ClearActions()
	controller.operatorClient.ClearActions()
	controller.operatorStore.Update(klusterlet)

	err = controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}
	assertRegistrationDeployment(t, controller.kubeClient.Actions(), "update", "https://localhost", "cluster3", 1)
	assertWorkDeployment(t, controller.kubeClient.Actions(), "update", "cluster3", "", 0)
}

func TestSyncWithPullSecret(t *testing.T) {
	klusterlet := newKlusterlet("klusterlet", "testns", "cluster1")
	bootStrapSecret := newSecret(helpers.BootstrapHubKubeConfig, "testns")
	hubKubeConfigSecret := newSecret(helpers.HubKubeConfig, "testns")
	hubKubeConfigSecret.Data["kubeconfig"] = []byte("dummuykubeconnfig")
	namespace := newNamespace("testns")
	pullSecret := newSecret(imagePullSecret, "open-cluster-management")
	controller := newTestController(klusterlet, nil, bootStrapSecret, hubKubeConfigSecret, namespace, pullSecret)
	syncContext := testinghelper.NewFakeSyncContext(t, "klusterlet")

	err := controller.controller.sync(context.TODO(), syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}

	var createdSecret *corev1.Secret
	kubeActions := controller.kubeClient.Actions()
	for _, action := range kubeActions {
		if action.GetVerb() == "create" && action.GetResource().Resource == "secrets" {
			createdSecret = action.(clienttesting.CreateActionImpl).Object.(*corev1.Secret)
			break
		}
	}

	if createdSecret == nil || createdSecret.Name != imagePullSecret {
		t.Errorf("Failed to sync pull secret")
	}
}

func TestDeployOnKube111(t *testing.T) {
	klusterlet := newKlusterlet("klusterlet", "testns", "cluster1")
	bootStrapSecret := newSecret(helpers.BootstrapHubKubeConfig, "testns")
	bootStrapSecret.Data["kubeconfig"] = newKubeConfig("testhost")
	hubKubeConfigSecret := newSecret(helpers.HubKubeConfig, "testns")
	hubKubeConfigSecret.Data["kubeconfig"] = []byte("dummuykubeconnfig")
	namespace := newNamespace("testns")
	controller := newTestController(klusterlet, nil, bootStrapSecret, hubKubeConfigSecret, namespace)
	kubeVersion, _ := version.ParseGeneric("v1.11.0")
	controller.controller.kubeVersion = kubeVersion
	syncContext := testinghelper.NewFakeSyncContext(t, "klusterlet")

	ctx := context.TODO()
	err := controller.controller.sync(ctx, syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}

	createObjects := []runtime.Object{}
	kubeActions := controller.kubeClient.Actions()
	for _, action := range kubeActions {
		if action.GetVerb() == "create" {
			object := action.(clienttesting.CreateActionImpl).Object
			createObjects = append(createObjects, object)
		}
	}

	dynamicAction := controller.dynamicClient.Actions()
	createCRDObjects := []runtime.Object{}
	for _, action := range dynamicAction {
		if action.GetVerb() == "create" {
			object := action.(clienttesting.CreateActionImpl).Object
			createCRDObjects = append(createCRDObjects, object)
		}
	}

	// Check if resources are created as expected
	// 7 managed static manifests + 8 management static manifests - 2 duplicated service account manifests + 1 addon namespace + 2 deployments + 2 kube111 clusterrolebindings
	if len(createObjects) != 18 {
		t.Errorf("Expect 18 objects created in the sync loop, actual %d", len(createObjects))
	}
	for _, object := range createObjects {
		ensureObject(t, object, klusterlet)
	}
	if len(createCRDObjects) != 2 {
		t.Errorf("Expect 2 v1beta1 crd objects created in the sync loop, actual %d", len(createCRDObjects))
	}

	operatorAction := controller.operatorClient.Actions()
	if len(operatorAction) != 2 {
		t.Errorf("Expect 2 actions in the sync loop, actual %#v", operatorAction)
	}

	testinghelper.AssertGet(t, operatorAction[0], "operator.open-cluster-management.io", "v1", "klusterlets")
	testinghelper.AssertAction(t, operatorAction[1], "update")
	testinghelper.AssertOnlyConditions(
		t, operatorAction[1].(clienttesting.UpdateActionImpl).Object,
		testinghelper.NamedCondition(klusterletApplied, "KlusterletApplied", metav1.ConditionTrue))

	// Delete the klusterlet
	now := metav1.Now()
	klusterlet.ObjectMeta.SetDeletionTimestamp(&now)
	controller.operatorStore.Update(klusterlet)
	controller.kubeClient.ClearActions()
	err = controller.controller.sync(ctx, syncContext)
	if err != nil {
		t.Errorf("Expected non error when sync, %v", err)
	}

	deleteActions := []clienttesting.DeleteActionImpl{}
	kubeActions = controller.kubeClient.Actions()
	for _, action := range kubeActions {
		if action.GetVerb() == "delete" {
			deleteAction := action.(clienttesting.DeleteActionImpl)
			deleteActions = append(deleteActions, deleteAction)
		}
	}

	// 7 managed static manifests + 8 management static manifests + 1 hub kubeconfig + 2 namespaces + 2 deployments + 2 kube111 clusterrolebindings
	if len(deleteActions) != 22 {
		t.Errorf("Expected 22 delete actions, but got %d", len(kubeActions))
	}
}

func newKubeConfig(host string) []byte {
	configData, _ := runtime.Encode(clientcmdlatest.Codec, &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{"test-cluster": {
			Server:                host,
			InsecureSkipTLSVerify: true,
		}},
		Contexts: map[string]*clientcmdapi.Context{"test-context": {
			Cluster: "test-cluster",
		}},
		CurrentContext: "test-context",
	})
	return configData
}

func newAppliedManifestWorks(host string, finalizers []string, terminated bool) *workapiv1.AppliedManifestWork {
	w := &workapiv1.AppliedManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:       fmt.Sprintf("%s-%s", fmt.Sprintf("%x", sha256.Sum256([]byte(host))), rand.String(6)),
			Finalizers: finalizers,
		},
	}

	if terminated {
		now := metav1.Now()
		w.DeletionTimestamp = &now
	}

	return w
}
