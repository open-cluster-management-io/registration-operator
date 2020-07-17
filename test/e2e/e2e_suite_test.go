package e2e

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var t *Tester
var registrationImage string
var workImage string

func init() {
	flag.StringVar(&registrationImage, "registration-image", "quay.io/open-cluster-management/registration", "registration-image is the image of registraiton agent")
	flag.StringVar(&workImage, "work-image", "quay.io/open-cluster-management/work", "work-image is the image of work agent")
	flag.Parse()
}

// This suite is sensitive to the following environment variables:
//
// - KUBECONFIG is the location of the kubeconfig file to use
var _ = BeforeSuite(func() {
	var err error

	t, err = NewTester("", registrationImage, workImage)
	Expect(err).ToNot(HaveOccurred())

	Eventually(func() error {
		return t.CheckHubReady()
	}, t.EventuallyTimeout, t.EventuallyInterval).Should(Succeed())

	Eventually(func() error {
		return t.CheckKlusterletOperatorReady()
	}, t.EventuallyTimeout, t.EventuallyInterval).Should(Succeed())

	err = t.SetBootstrapHubSecret("")
	Expect(err).ToNot(HaveOccurred())
})
