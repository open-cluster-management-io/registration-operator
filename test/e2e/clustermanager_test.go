package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	operatorapiv1 "open-cluster-management.io/api/operator/v1"
)

// clustermanager is used to store and receate the ClusterManager CR.
var clustermanager *operatorapiv1.ClusterManager

// This ordered describe is only used to test ClusterManager CR, it can not run in parallel with other tests.
// Please run `make test-e2e-in-parallel` in order to skip this case if needed.
// Also please add other testcases of cluster manager to a separate describe block.
var _ = Describe("Delete and recreate ClusterManager CR", Label("run-alone"), Ordered, func() {
	When("ClusterManager CR is deleted", func() {
		It("Should remove CRDs and namespace", func() {
			var err error

			// Cache the current ClusterManager CR
			clustermanager, err = t.GetClusterManager(context.TODO())
			Expect(err).To(BeNil())

			// Delete ClusterManager CR
			err = t.DeleteClusterManager(context.TODO())
			Expect(err).To(BeNil())

			// Check namespace are removed
			Eventually(func() error {
				_, err = t.KubeClient.CoreV1().Namespaces().Get(context.TODO(), t.clusterManagerNamespace, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("namespace %s still exists", t.clusterManagerNamespace)
			}, t.EventuallyTimeout, t.EventuallyInterval).Should(Succeed())
		})
	})

	When("ClusterManager CR is recreated", func() {
		It("Should recreate CRDs and namespace", func() {
			var err error

			// Recreate ClusterManager CR
			err = t.RecreateClusterManager(context.TODO(), clustermanager)
			Expect(err).To(BeNil())
			fmt.Println("recreate time:", time.Now().GoString())

			// Check CluserManager is ready again
			Eventually(t.CheckClusterManagerStatus, t.EventuallyTimeout, t.EventuallyInterval).Should(Succeed())

			// Check the hub ready again
			Eventually(t.CheckHubReady, t.EventuallyTimeout, t.EventuallyInterval).Should(Succeed())
		})
	})
})
