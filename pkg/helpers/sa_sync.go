package helpers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// EnsureKubeocnfigFromSA create a secret with kubeconfig
// It will first get the SA token from the hosted cluster using hostedClient
// Then it will create a kubeconfig secret on currentClient using currentClient
// The secret name of kubeconfig should be saName+"-kubeconfig", and the namespace should be saNamespace
func EnsureKubeconfigFromSA(ctx context.Context, saName, saNamespace string, templateKubeconfig *rest.Config, renderSAToken func([]byte) error, hostedClient, currentClient kubernetes.Interface) error {
	// get kubeconfig secret
	_, err := currentClient.CoreV1().Secrets(saNamespace).Get(ctx, saName+"-kubeconfig", metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err == nil {
		// kubeconfig secret already exist, return directly
		return nil
	}

	// get the service account
	sa, err := hostedClient.CoreV1().ServiceAccounts(saNamespace).Get(ctx, saName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if len(sa.Secrets) == 0 {
		return fmt.Errorf("token secret for %s not exist yet", saName)
	}
	tokenSecretName := sa.Secrets[0].Name

	// get the token secret
	tokenSecret, err := hostedClient.CoreV1().Secrets(saNamespace).Get(ctx, tokenSecretName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	saToken, ok := tokenSecret.Data["token"]
	if !ok {
		return fmt.Errorf("no token in data for secret %s", tokenSecretName)
	}

	// generate a new kubeconfig
	templateKubeconfigContent, err := clientcmd.Write(clientcmdapi.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster": {
				Server:                   templateKubeconfig.Host,
				CertificateAuthorityData: templateKubeconfig.CAData,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"context": {
				Cluster:  "cluster",
				AuthInfo: "user",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"user": {
				Token: string(saToken),
			},
		},
		CurrentContext: "context",
	})
	if err != nil {
		return err
	}

	return renderSAToken(templateKubeconfigContent)
}
