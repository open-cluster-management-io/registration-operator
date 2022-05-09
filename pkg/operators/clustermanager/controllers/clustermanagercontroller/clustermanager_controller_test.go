package clustermanagercontroller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/events/eventstesting"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	fakeapiextensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	fakekube "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
	fakeapiregistration "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"
	apiregistrationclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
	fakeoperatorlient "open-cluster-management.io/api/client/operator/clientset/versioned/fake"
	operatorinformers "open-cluster-management.io/api/client/operator/informers/externalversions"
	operatorapiv1 "open-cluster-management.io/api/operator/v1"

	"open-cluster-management.io/registration-operator/pkg/helpers"
	testinghelper "open-cluster-management.io/registration-operator/pkg/helpers/testing"
)

var (
	ctx = context.Background()
)

type testController struct {
	clusterManagerController *clusterManagerController
	managementKubeClient     *fakekube.Clientset
	hubKubeClient            *fakekube.Clientset
	apiExtensionClient       *fakeapiextensions.Clientset
	apiRegistrationClient    *fakeapiregistration.Clientset
	operatorClient           *fakeoperatorlient.Clientset
}

func newClusterManager(name string) *operatorapiv1.ClusterManager {
	return &operatorapiv1.ClusterManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Finalizers: []string{clusterManagerFinalizer},
		},
		Spec: operatorapiv1.ClusterManagerSpec{
			RegistrationImagePullSpec: "testregistration",
			DeployOption: operatorapiv1.ClusterManagerDeployOption{
				Mode: operatorapiv1.InstallModeDefault,
			},
		},
	}
}

func newTestController(clustermanager *operatorapiv1.ClusterManager) *testController {
	kubeClient := fakekube.NewSimpleClientset()
	kubeInfomers := kubeinformers.NewSharedInformerFactory(kubeClient, 5*time.Minute)
	fakeOperatorClient := fakeoperatorlient.NewSimpleClientset(clustermanager)
	operatorInformers := operatorinformers.NewSharedInformerFactory(fakeOperatorClient, 5*time.Minute)

	clusterManagerController := &clusterManagerController{
		clusterManagerClient: fakeOperatorClient.OperatorV1().ClusterManagers(),
		clusterManagerLister: operatorInformers.Operator().V1().ClusterManagers().Lister(),
		configMapLister:      kubeInfomers.Core().V1().ConfigMaps().Lister(),
		cache:                resourceapply.NewResourceCache(),
	}

	store := operatorInformers.Operator().V1().ClusterManagers().Informer().GetStore()
	_ = store.Add(clustermanager)

	return &testController{
		clusterManagerController: clusterManagerController,
		operatorClient:           fakeOperatorClient,
	}
}

func setup(t *testing.T, tc *testController, crds ...runtime.Object) {
	fakeHubKubeClient := fakekube.NewSimpleClientset()
	fakeManagementKubeClient := fakekube.NewSimpleClientset()
	fakeAPIExtensionClient := fakeapiextensions.NewSimpleClientset(crds...)
	fakeAPIRegistrationClient := fakeapiregistration.NewSimpleClientset()

	// set clients in test controller
	tc.apiExtensionClient = fakeAPIExtensionClient
	tc.apiRegistrationClient = fakeAPIRegistrationClient
	tc.hubKubeClient = fakeHubKubeClient
	tc.managementKubeClient = fakeManagementKubeClient

	// set clients in clustermanager controller
	tc.clusterManagerController.recorder = eventstesting.NewTestingEventRecorder(t)
	tc.clusterManagerController.operatorKubeClient = fakeManagementKubeClient
	tc.clusterManagerController.generateHubClusterClients = func(hubKubeConfig *rest.Config) (kubernetes.Interface, apiextensionsclient.Interface, apiregistrationclient.APIServicesGetter, error) {
		return fakeHubKubeClient, fakeAPIExtensionClient, fakeAPIRegistrationClient.ApiregistrationV1(), nil
	}
	tc.clusterManagerController.ensureSAKubeconfigs = func(ctx context.Context, clusterManagerName, clusterManagerNamespace string, hubConfig *rest.Config, hubClient, managementClient kubernetes.Interface, recorder events.Recorder) error {
		return nil
	}
}

func ensureObject(t *testing.T, object runtime.Object, hubCore *operatorapiv1.ClusterManager) {
	access, err := meta.Accessor(object)
	if err != nil {
		t.Errorf("Unable to access objectmeta: %v", err)
	}

	switch o := object.(type) {
	case *corev1.Namespace:
		testinghelper.AssertEqualNameNamespace(t, access.GetName(), "", helpers.ClusterManagerNamespace(hubCore.Name, hubCore.Spec.DeployOption.Mode), "")
	case *appsv1.Deployment:
		if strings.Contains(o.Name, "registration") && hubCore.Spec.RegistrationImagePullSpec != o.Spec.Template.Spec.Containers[0].Image {
			t.Errorf("Registration image does not match to the expected.")
		}
		if strings.Contains(o.Name, "placement") && hubCore.Spec.PlacementImagePullSpec != o.Spec.Template.Spec.Containers[0].Image {
			t.Errorf("Placement image does not match to the expected.")
		}
	}
}

// TestSyncDeploy tests sync manifests of hub component
func TestSyncDeploy(t *testing.T) {
	clusterManager := newClusterManager("testhub")
	tc := newTestController(clusterManager)
	setup(t, tc)

	syncContext := testinghelper.NewFakeSyncContext(t, "testhub")

	err := tc.clusterManagerController.sync(ctx, syncContext)
	if err != nil {
		t.Fatalf("Expected no error when sync, %v", err)
	}

	createKubeObjects := []runtime.Object{}
	kubeActions := append(tc.hubKubeClient.Actions(), tc.managementKubeClient.Actions()...) // record objects from both hub and management cluster
	for _, action := range kubeActions {
		if action.GetVerb() == "create" {
			object := action.(clienttesting.CreateActionImpl).Object
			createKubeObjects = append(createKubeObjects, object)
		}
	}

	// Check if resources are created as expected
	// We expect creat the namespace twice respectively in the management cluster and the hub cluster.
	testinghelper.AssertEqualNumber(t, len(createKubeObjects), 24)
	for _, object := range createKubeObjects {
		ensureObject(t, object, clusterManager)
	}

	createCRDObjects := []runtime.Object{}
	crdActions := tc.apiExtensionClient.Actions()
	for _, action := range crdActions {
		if action.GetVerb() == "create" {
			object := action.(clienttesting.CreateActionImpl).Object
			createCRDObjects = append(createCRDObjects, object)
		}
	}
	// Check if resources are created as expected
	testinghelper.AssertEqualNumber(t, len(createCRDObjects), 9)

	createAPIServiceObjects := []runtime.Object{}
	apiServiceActions := tc.apiRegistrationClient.Actions()
	for _, action := range apiServiceActions {
		if action.GetVerb() == "create" {
			object := action.(clienttesting.CreateActionImpl).Object
			createAPIServiceObjects = append(createAPIServiceObjects, object)
		}
	}
	// Check if resources are created as expected
	testinghelper.AssertEqualNumber(t, len(createAPIServiceObjects), 2)

	clusterManagerAction := tc.operatorClient.Actions()
	testinghelper.AssertEqualNumber(t, len(clusterManagerAction), 2)
	testinghelper.AssertAction(t, clusterManagerAction[1], "update")
	testinghelper.AssertOnlyConditions(
		t, clusterManagerAction[1].(clienttesting.UpdateActionImpl).Object,
		testinghelper.NamedCondition(clusterManagerApplied, "ClusterManagerApplied", metav1.ConditionTrue))
}

// TestSyncDelete test cleanup hub deploy
func TestSyncDelete(t *testing.T) {
	clusterManager := newClusterManager("testhub")
	now := metav1.Now()
	clusterManager.ObjectMeta.SetDeletionTimestamp(&now)

	tc := newTestController(clusterManager)
	setup(t, tc)

	syncContext := testinghelper.NewFakeSyncContext(t, "testhub")
	clusterManagerNamespace := helpers.ClusterManagerNamespace(clusterManager.ClusterName, clusterManager.Spec.DeployOption.Mode)

	err := tc.clusterManagerController.sync(ctx, syncContext)
	if err != nil {
		t.Fatalf("Expected non error when sync, %v", err)
	}

	deleteKubeActions := []clienttesting.DeleteActionImpl{}
	kubeActions := append(tc.hubKubeClient.Actions(), tc.managementKubeClient.Actions()...)
	for _, action := range kubeActions {
		if action.GetVerb() == "delete" {
			deleteKubeAction := action.(clienttesting.DeleteActionImpl)
			deleteKubeActions = append(deleteKubeActions, deleteKubeAction)
		}
	}
	testinghelper.AssertEqualNumber(t, len(deleteKubeActions), 20) // delete namespace both from the hub cluster and the mangement cluster

	deleteCRDActions := []clienttesting.DeleteActionImpl{}
	crdActions := tc.apiExtensionClient.Actions()
	for _, action := range crdActions {
		if action.GetVerb() == "delete" {
			deleteCRDAction := action.(clienttesting.DeleteActionImpl)
			deleteCRDActions = append(deleteCRDActions, deleteCRDAction)
		}
	}
	// Check if resources are created as expected
	testinghelper.AssertEqualNumber(t, len(deleteCRDActions), 11)

	deleteAPIServiceActions := []clienttesting.DeleteActionImpl{}
	apiServiceActions := tc.apiRegistrationClient.Actions()
	for _, action := range apiServiceActions {
		if action.GetVerb() == "delete" {
			deleteAPIServiceAction := action.(clienttesting.DeleteActionImpl)
			deleteAPIServiceActions = append(deleteAPIServiceActions, deleteAPIServiceAction)
		}
	}
	// Check if resources are created as expected
	testinghelper.AssertEqualNumber(t, len(deleteAPIServiceActions), 2)

	for _, action := range deleteKubeActions {
		switch action.Resource.Resource {
		case "namespaces":
			testinghelper.AssertEqualNameNamespace(t, action.Name, "", clusterManagerNamespace, "")
		}
	}
}

// TestDeleteCRD test delete crds
func TestDeleteCRD(t *testing.T) {
	clusterManager := newClusterManager("testhub")
	now := metav1.Now()
	clusterManager.ObjectMeta.SetDeletionTimestamp(&now)
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: crdNames[0],
		},
	}

	tc := newTestController(clusterManager)
	setup(t, tc, crd)

	// Return crd with the first get, and return not found with the 2nd get
	getCount := 0
	tc.apiExtensionClient.PrependReactor("get", "customresourcedefinitions", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		if getCount == 0 {
			getCount = getCount + 1
			return true, crd, nil
		}
		return true, &apiextensionsv1.CustomResourceDefinition{}, errors.NewNotFound(
			apiextensionsv1.Resource("customresourcedefinitions"), crdNames[0])

	})
	syncContext := testinghelper.NewFakeSyncContext(t, "testhub")
	err := tc.clusterManagerController.sync(ctx, syncContext)
	if err == nil {
		t.Fatalf("Expected error when sync at first time")
	}

	err = tc.clusterManagerController.sync(ctx, syncContext)
	if err != nil {
		t.Fatalf("Expected no error when sync at second time: %v", err)
	}
}

func TestIsIPFormat(t *testing.T) {
	cases := []struct {
		address    string
		isIPFormat bool
	}{
		{
			address:    "127.0.0.1",
			isIPFormat: true,
		},
		{
			address:    "localhost",
			isIPFormat: false,
		},
	}
	for _, c := range cases {
		if isIPFormat(c.address) != c.isIPFormat {
			t.Fatalf("expected %v, got %v", c.isIPFormat, isIPFormat(c.address))
		}
	}
}
