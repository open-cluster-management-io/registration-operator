package certrotationcontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	errorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	operatorinformer "open-cluster-management.io/api/client/operator/informers/externalversions/operator/v1"
	operatorlister "open-cluster-management.io/api/client/operator/listers/operator/v1"
	"open-cluster-management.io/registration-operator/pkg/certrotation"
	"open-cluster-management.io/registration-operator/pkg/helpers"
)

const (
	signerSecret      = "signer-secret"
	caBundleConfigmap = "ca-bundle-configmap"
	signerNamePrefix  = "cluster-manager-webhook"
)

// Follow the rules below to set the value of SigningCertValidity/TargetCertValidity/ResyncInterval:
//
// 1) SigningCertValidity * 1/5 * 1/5 > ResyncInterval * 2
// 2) TargetCertValidity * 1/5 > ResyncInterval * 2
var SigningCertValidity = time.Hour * 24 * 365
var TargetCertValidity = time.Hour * 24 * 30
var ResyncInterval = time.Minute * 5

// certRotationController does:
//
// 1) continuously create a self-signed signing CA (via SigningRotation).
//    It creates the next one when a given percentage of the validity of the old CA has passed.
// 2) maintain a CA bundle with all not yet expired CA certs.
// 3) continuously create target cert/key pairs signed by the latest signing CA
//    It creates the next one when a given percentage of the validity of the previous cert has
//    passed, or when a new CA has been created.
type certRotationController struct {
	rotationMap          map[string]rotations // key is clusterManager's name, value is a rotations struct
	kubeClient           kubernetes.Interface
	secretInformer       corev1informers.SecretInformer
	configMapInformer    corev1informers.ConfigMapInformer
	recorder             events.Recorder
	clusterManagerLister operatorlister.ClusterManagerLister
}

type rotations struct {
	signingRotation  certrotation.SigningRotation
	caBundleRotation certrotation.CABundleRotation
	targetRotations  []certrotation.TargetRotation
}

func NewCertRotationController(
	kubeClient kubernetes.Interface,
	secretInformer corev1informers.SecretInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	clusterManagerInformer operatorinformer.ClusterManagerInformer,
	recorder events.Recorder,
) factory.Controller {
	c := &certRotationController{
		rotationMap:          make(map[string]rotations),
		kubeClient:           kubeClient,
		secretInformer:       secretInformer,
		configMapInformer:    configMapInformer,
		recorder:             recorder,
		clusterManagerLister: clusterManagerInformer.Lister(),
	}
	return factory.New().
		ResyncEvery(ResyncInterval).
		WithSync(c.sync).
		WithInformers(
			secretInformer.Informer(),
			configMapInformer.Informer(),
			clusterManagerInformer.Informer(),
		).
		ToController("CertRotationController", recorder)
}

func (c certRotationController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	// do nothing if there is no cluster manager
	clustermanagers, err := c.clusterManagerLister.List(labels.Everything())
	if err != nil {
		return err
	}
	if len(clustermanagers) == 0 {
		klog.V(4).Infof("No ClusterManager found")
		return nil
	}

	for i := range clustermanagers {
		clustermanagerName := clustermanagers[i].Name

		klog.Infof("Reconciling ClusterManager %q", clustermanagerName)
		// do nothing if the cluster manager is deleting
		if !clustermanagers[0].DeletionTimestamp.IsZero() {
			return nil
		}

		// check if namespace exists or not
		clustermanagerNamespace := helpers.ClusterManagerNamespace(clustermanagerName)
		_, err = c.kubeClient.CoreV1().Namespaces().Get(ctx, clustermanagerNamespace, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return fmt.Errorf("namespace %q does not exist yet", clustermanagerNamespace)
		}
		if err != nil {
			return err
		}

		// check if rotations exist, if not exist then create one
		if _, ok := c.rotationMap[clustermanagerName]; !ok {
			signingRotation := certrotation.SigningRotation{
				Namespace:        clustermanagerNamespace,
				Name:             signerSecret,
				SignerNamePrefix: signerNamePrefix,
				Validity:         SigningCertValidity,
				Lister:           c.secretInformer.Lister(),
				Client:           c.kubeClient.CoreV1(),
				EventRecorder:    c.recorder,
			}
			caBundleRotation := certrotation.CABundleRotation{
				Namespace:     clustermanagerNamespace,
				Name:          caBundleConfigmap,
				Lister:        c.configMapInformer.Lister(),
				Client:        c.kubeClient.CoreV1(),
				EventRecorder: c.recorder,
			}
			targetRotations := []certrotation.TargetRotation{
				{
					Namespace:     clustermanagerNamespace,
					Name:          helpers.RegistrationWebhookSecret,
					Validity:      TargetCertValidity,
					HostNames:     []string{fmt.Sprintf("%s.%s.svc", helpers.RegistrationWebhookService, clustermanagerNamespace)},
					Lister:        c.secretInformer.Lister(),
					Client:        c.kubeClient.CoreV1(),
					EventRecorder: c.recorder,
				},
				{
					Namespace:     clustermanagerNamespace,
					Name:          helpers.WorkWebhookSecret,
					Validity:      TargetCertValidity,
					HostNames:     []string{fmt.Sprintf("%s.%s.svc", helpers.WorkWebhookService, clustermanagerNamespace)},
					Lister:        c.secretInformer.Lister(),
					Client:        c.kubeClient.CoreV1(),
					EventRecorder: c.recorder,
				},
			}
			c.rotationMap[clustermanagerName] = rotations{
				signingRotation:  signingRotation,
				caBundleRotation: caBundleRotation,
				targetRotations:  targetRotations,
			}
		}

		rotations := c.rotationMap[clustermanagerName]

		// reconcile cert/key pair for signer
		signingCertKeyPair, err := rotations.signingRotation.EnsureSigningCertKeyPair()
		if err != nil {
			return err
		}

		// reconcile ca bundle
		cabundleCerts, err := rotations.caBundleRotation.EnsureConfigMapCABundle(signingCertKeyPair)
		if err != nil {
			return err
		}

		// reconcile target cert/key pairs
		errs := []error{}
		for _, targetRotation := range rotations.targetRotations {
			if err := targetRotation.EnsureTargetCertKeyPair(signingCertKeyPair, cabundleCerts); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) != 0 {
			return errorhelpers.NewMultiLineAggregate(errs)
		}

	}
	return nil
}
