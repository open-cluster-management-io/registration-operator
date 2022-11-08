package crdmanager

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	versionutil "k8s.io/apimachinery/pkg/version"
	"k8s.io/klog/v2"
	"open-cluster-management.io/registration-operator/pkg/version"
)

const versionLabelKey = "operator.open-cluster-management.io/version"

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	utilruntime.Must(apiextensionsv1.AddToScheme(genericScheme))
	utilruntime.Must(apiextensionsv1beta1.AddToScheme(genericScheme))
}

type CRD interface {
	*apiextensionsv1.CustomResourceDefinition | *apiextensionsv1beta1.CustomResourceDefinition
}

type manager[T CRD] struct {
	crds   map[string]T
	client crdClient[T]
}

type crdClient[T CRD] interface {
	Get(ctx context.Context, name string, opt metav1.GetOptions) (T, error)
	Create(ctx context.Context, obj T, opt metav1.CreateOptions) (T, error)
	Update(ctx context.Context, obj T, opt metav1.UpdateOptions) (T, error)
	Delete(ctx context.Context, name string, opt metav1.DeleteOptions) error
}

type RemainingCRDError struct {
	RemainingCRDs []string
}

func (r *RemainingCRDError) Error() string {
	return fmt.Sprintf("Thera are still reaming CRDs: %s", strings.Join(r.RemainingCRDs, ","))
}

func NewManager[T CRD](client crdClient[T], manifests resourceapply.AssetFunc, files ...string) (*manager[T], error) {
	manager := &manager[T]{
		crds:   map[string]T{},
		client: client,
	}

	var errs []error
	for _, file := range files {
		objBytes, err := manifests(file)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// apply v1beta1 crd if the object is crd v1beta1, we need to do this by using dynamic client
		// since the v1beta1 schema has been removed in kube 1.23.
		// TODO remove this logic after we do not support spoke with version lowner than 1.16
		requiredObj, _, err := genericCodec.Decode(objBytes, nil, nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		accessor, err := meta.Accessor(requiredObj)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		manager.crds[accessor.GetName()] = requiredObj.(T)
	}

	return manager, utilerrors.NewAggregate(errs)
}

func (m *manager[T]) Clean(ctx context.Context) error {
	var errs []error
	var remainingCRDs []string
	for name := range m.crds {
		err := m.client.Delete(ctx, name, metav1.DeleteOptions{})
		switch {
		case errors.IsNotFound(err):
			continue
		case err == nil:
			remainingCRDs = append(remainingCRDs, name)
		default:
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
	}
	if len(remainingCRDs) > 0 {
		return &RemainingCRDError{RemainingCRDs: remainingCRDs}
	}

	return nil
}

func (m *manager[T]) Apply(ctx context.Context) error {
	var errs []error
	for name, crd := range m.crds {
		err := m.ApplyOne(ctx, name, crd)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (m *manager[T]) ApplyOne(ctx context.Context, name string, required T) error {
	existing, err := m.client.Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err := m.client.Create(ctx, required, metav1.CreateOptions{})
		klog.Infof("crd %s is created", name)
		return err
	}
	if err != nil {
		return err
	}

	// if existingVersion is higher than the required version, do not update crd.
	accessor, err := meta.Accessor(existing)
	if err != nil {
		return err
	}
	existingVersion := accessor.GetLabels()[versionLabelKey]
	requiredVersion := version.Get().GitVersion
	if versionutil.CompareKubeAwareVersionStrings(requiredVersion, existingVersion) <= 0 {
		return nil
	}

	requiredAccessor, err := meta.Accessor(required)
	if err != nil {
		return err
	}

	labels := requiredAccessor.GetLabels()
	labels[versionLabelKey] = requiredVersion
	requiredAccessor.SetLabels(labels)
	requiredAccessor.SetResourceVersion(accessor.GetResourceVersion())

	_, err = m.client.Update(ctx, required, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.Infof("crd %s is updated to version %s", name, requiredVersion)

	return nil
}
