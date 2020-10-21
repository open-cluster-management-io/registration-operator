package helpers

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	fakeoperatorclient "github.com/open-cluster-management/api/client/operator/clientset/versioned/fake"
	operatorinformers "github.com/open-cluster-management/api/client/operator/informers/externalversions"
	operatorapiv1 "github.com/open-cluster-management/api/operator/v1"
)

func newSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{},
	}
}

func newKlusterlet(name, namespace, clustername string) *operatorapiv1.Klusterlet {
	return &operatorapiv1.Klusterlet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: operatorapiv1.KlusterletSpec{
			RegistrationImagePullSpec: "testregistration",
			WorkImagePullSpec:         "testwork",
			ClusterName:               clustername,
			Namespace:                 namespace,
			ExternalServerURLs:        []operatorapiv1.ServerURL{},
		},
	}
}

func newClusterManager(name string) *operatorapiv1.ClusterManager {
	return &operatorapiv1.ClusterManager{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: operatorapiv1.ClusterManagerSpec{
			RegistrationImagePullSpec: "testregistration",
		},
	}
}

func TestKlusterletSecretQueueKeyFunc(t *testing.T) {
	cases := []struct {
		name             string
		object           runtime.Object
		klusterlet       *operatorapiv1.Klusterlet
		expectedFiltered bool
		expectedKey      string
	}{
		{
			name:             "key by hub config secret",
			object:           newSecret(HubKubeConfig, "test"),
			klusterlet:       newKlusterlet("testklusterlet", "test", ""),
			expectedFiltered: true,
			expectedKey:      "testklusterlet",
		},
		{
			name:             "key by bootstrap secret",
			object:           newSecret(BootstrapHubKubeConfig, "test"),
			klusterlet:       newKlusterlet("testklusterlet", "test", ""),
			expectedFiltered: true,
			expectedKey:      "testklusterlet",
		},
		{
			name:             "key by wrong secret",
			object:           newSecret("dummy", "test"),
			klusterlet:       newKlusterlet("testklusterlet", "test", ""),
			expectedFiltered: false,
		},
		{
			name:             "key by klusterlet with empty namespace",
			object:           newSecret(BootstrapHubKubeConfig, KlusterletDefaultNamespace),
			klusterlet:       newKlusterlet("testklusterlet", "", ""),
			expectedKey:      "testklusterlet",
			expectedFiltered: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeOperatorClient := fakeoperatorclient.NewSimpleClientset(c.klusterlet)
			operatorInformers := operatorinformers.NewSharedInformerFactory(fakeOperatorClient, 5*time.Minute)
			store := operatorInformers.Operator().V1().Klusterlets().Informer().GetStore()
			store.Add(c.klusterlet)
			keyFunc := KlusterletQueueKeyFunc(operatorInformers.Operator().V1().Klusterlets().Lister())
			if KlusterlsetSecretEventFilter(c.object) != c.expectedFiltered {
				t.Errorf("Queued key should be filtered")
				return
			}
			if c.expectedFiltered {
				actualKey := keyFunc(c.object)
				if actualKey != c.expectedKey {
					t.Errorf("Queued key is not correct: actual %s, expected %s", actualKey, c.expectedKey)
				}
			}
		})
	}
}

func TestKlusterletDeploymentQueueKeyFunc(t *testing.T) {
	cases := []struct {
		name             string
		object           runtime.Object
		klusterlet       *operatorapiv1.Klusterlet
		expectedFiltered bool
		expectedKey      string
	}{
		{
			name:             "key by work agent",
			object:           newDeployment("testklusterlet-work-agent", "test", 0),
			klusterlet:       newKlusterlet("testklusterlet", "test", ""),
			expectedFiltered: true,
			expectedKey:      "testklusterlet",
		},
		{
			name:             "key by registrartion agent",
			object:           newDeployment("testklusterlet-registration-agent", "test", 0),
			klusterlet:       newKlusterlet("testklusterlet", "test", ""),
			expectedFiltered: true,
			expectedKey:      "testklusterlet",
		},
		{
			name:             "key by wrong deployment",
			object:           newDeployment("dummy", "test", 0),
			klusterlet:       newKlusterlet("testklusterlet", "test", ""),
			expectedFiltered: false,
			expectedKey:      "",
		},
		{
			name:             "key by klusterlet with empty namespace",
			object:           newDeployment("testklusterlet-work-agent", KlusterletDefaultNamespace, 0),
			klusterlet:       newKlusterlet("testklusterlet", "", ""),
			expectedFiltered: true,
			expectedKey:      "testklusterlet",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeOperatorClient := fakeoperatorclient.NewSimpleClientset(c.klusterlet)
			operatorInformers := operatorinformers.NewSharedInformerFactory(fakeOperatorClient, 5*time.Minute)
			store := operatorInformers.Operator().V1().Klusterlets().Informer().GetStore()
			store.Add(c.klusterlet)
			if KlusterletDeploymentEventFilter(c.object) != c.expectedFiltered {
				t.Errorf("Queued key should be filtered")
				return
			}
			if c.expectedFiltered {
				keyFunc := KlusterletQueueKeyFunc(operatorInformers.Operator().V1().Klusterlets().Lister())
				actualKey := keyFunc(c.object)
				if actualKey != c.expectedKey {
					t.Errorf("Queued key is not correct: actual %s, expected %s", actualKey, c.expectedKey)
				}
			}
		})
	}
}

func TestClusterManagerDeploymentQueueKeyFunc(t *testing.T) {
	cases := []struct {
		name             string
		object           runtime.Object
		clusterManager   *operatorapiv1.ClusterManager
		expectedFiltered bool
		expectedKey      string
	}{
		{
			name:             "key by work controller",
			object:           newDeployment("testhub-work-controller", ClusterManagerNamespace, 0),
			clusterManager:   newClusterManager("testhub"),
			expectedFiltered: true,
			expectedKey:      "testhub",
		},
		{
			name:             "key by registrartion controller",
			object:           newDeployment("testhub-registration-controller", ClusterManagerNamespace, 0),
			clusterManager:   newClusterManager("testhub"),
			expectedFiltered: true,
			expectedKey:      "testhub",
		},
		{
			name:             "key by wrong deployment",
			object:           newDeployment("dummy", "test", 0),
			clusterManager:   newClusterManager("testhub"),
			expectedFiltered: false,
			expectedKey:      "",
		},
		{
			name:             "key by wrong namespace",
			object:           newDeployment("testklusterlet-registration-controller", "test", 0),
			clusterManager:   newClusterManager("testhub"),
			expectedFiltered: false,
			expectedKey:      "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeOperatorClient := fakeoperatorclient.NewSimpleClientset(c.clusterManager)
			operatorInformers := operatorinformers.NewSharedInformerFactory(fakeOperatorClient, 5*time.Minute)
			store := operatorInformers.Operator().V1().ClusterManagers().Informer().GetStore()
			store.Add(c.clusterManager)
			if ClusterManagerDeploymentEventFilter(c.object) != c.expectedFiltered {
				t.Errorf("Queued key should be filtered")
			}
			if c.expectedFiltered {
				keyFunc := ClusterManagerQueueKeyFunc(operatorInformers.Operator().V1().ClusterManagers().Lister())
				actualKey := keyFunc(c.object)
				if actualKey != c.expectedKey {
					t.Errorf("Queued key is not correct: actual %s, expected %s", actualKey, c.expectedKey)
				}
			}
		})
	}
}
