package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

var _ = Describe("Create klusterlet and then create a configmap by manifestwork", func() {
	var klusterletName = fmt.Sprintf("e2e-klusterlet-%s", rand.String(6))
	var clusterName = fmt.Sprintf("e2e-managedcluster-%s", rand.String(6))
	var agentNamespace = "open-cluster-management-agent"
	var workName = fmt.Sprintf("e2e-work-configmap-%s", rand.String(6))
	var configMapNamespace = "default"
	var configMapName = fmt.Sprintf("e2e-configmap-%s", rand.String(6))

	AfterEach(func() {
		By(fmt.Sprintf("clean klusterlet %v resources after the test case", klusterletName))
		t.cleanKlusterletResources(klusterletName, clusterName)
	})

	It("Create configmap using manifestwork and then delete klusterlet", func() {
		var err error
		By(fmt.Sprintf("create klusterlet %v with managed cluster name %v", klusterletName, clusterName))
		_, err = t.CreateKlusterlet(klusterletName, clusterName, agentNamespace)
		Expect(err).ToNot(HaveOccurred())

		By(fmt.Sprintf("waiting for the managed cluster %v to be created", clusterName))
		Eventually(func() error {
			_, err = t.GetCreatedManagedCluster(clusterName)
			return err
		}, t.EventuallyTimeout*5, t.EventuallyInterval*5).Should(Succeed())

		By(fmt.Sprintf("approve the created managed cluster %v", clusterName))
		Eventually(func() error {
			return t.ApproveCSR(clusterName)
		}, t.EventuallyTimeout, t.EventuallyInterval).Should(Succeed())

		By(fmt.Sprintf("accept the created managed cluster %v", clusterName))
		Eventually(func() error {
			return t.AcceptsClient(clusterName)
		}, t.EventuallyTimeout, t.EventuallyInterval).Should(Succeed())

		By(fmt.Sprintf("waiting for the managed cluster %v to be ready", clusterName))
		Eventually(func() error {
			return t.CheckManagedClusterStatus(clusterName)
		}, t.EventuallyTimeout*5, t.EventuallyInterval*5).Should(Succeed())

		By(fmt.Sprintf("create configmap %v/%v using manifestwork %v/%v", configMapNamespace,
			configMapName, clusterName, workName))
		_, err = t.CreateWorkOfConfigMap(workName, clusterName, configMapName, configMapNamespace)
		Expect(err).ToNot(HaveOccurred())

		By(fmt.Sprintf("waiting for configmap %v/%v to be created", configMapNamespace, configMapName))
		lines := int64(10)
		Eventually(func() error {
			pods, err := t.KubeClient.CoreV1().Pods(agentNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: "app=klusterlet-manifestwork-agent"})
			for _, pod := range pods.Items {
				fmt.Printf("klusterlet pod %v : %#v\n", pod.Name, pod)
				req := t.KubeClient.CoreV1().Pods(agentNamespace).GetLogs(pod.Name, &corev1.PodLogOptions{TailLines: &lines})
				podLogs, _ := req.Stream(context.TODO())
				buf := new(bytes.Buffer)
				_, err = io.Copy(buf, podLogs)
				if err != nil {
					return err
				}
				fmt.Printf("%s\n", buf.String())
				podLogs.Close()
			}
			_, err = t.KubeClient.CoreV1().ConfigMaps(configMapNamespace).
				Get(context.TODO(), configMapName, metav1.GetOptions{})
			return err
		}, t.EventuallyTimeout*5, t.EventuallyInterval*5).Should(Succeed())

		By(fmt.Sprintf("delete manifestwork %v/%v", clusterName, workName))
		err = t.WorkClient.WorkV1().ManifestWorks(clusterName).Delete(context.Background(), workName, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() bool {
			_, err := t.WorkClient.WorkV1().ManifestWorks(clusterName).Get(context.Background(), workName, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return true
			}
			return false
		}, t.EventuallyTimeout*5, t.EventuallyInterval*5).Should(BeTrue())
	})
})
