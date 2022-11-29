package crdmanager

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

type Manager[T CRD] struct {
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

func NewManager[T CRD](client crdClient[T]) *Manager[T] {
	manager := &Manager[T]{
		client: client,
	}

	return manager
}

func (m *Manager[T]) CleanOne(ctx context.Context, name string, skip bool) error {
	// remove version label if skip clean
	if skip {
		existing, err := m.client.Get(ctx, name, metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			return nil
		case err != nil:
			return err
		}
		accessor, err := meta.Accessor(existing)
		if err != nil {
			return err
		}
		labels := accessor.GetLabels()
		if _, ok := labels[versionLabelKey]; ok {
			delete(labels, versionLabelKey)
		}
		accessor.SetLabels(labels)
		_, err = m.client.Update(ctx, existing, metav1.UpdateOptions{})
		return err
	}

	err := m.client.Delete(ctx, name, metav1.DeleteOptions{})
	switch {
	case apierrors.IsNotFound(err):
		return nil
	case err == nil:
		return &RemainingCRDError{RemainingCRDs: []string{name}}
	}

	return err
}

func (m *Manager[T]) Clean(ctx context.Context, skip bool, manifests resourceapply.AssetFunc, files ...string) error {
	var errs []error
	var remainingCRDs []string

	for _, file := range files {
		objBytes, err := manifests(file)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		requiredObj, _, err := genericCodec.Decode(objBytes, nil, nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		accessor, err := meta.Accessor(requiredObj)
		if err != nil {
			return err
		}

		err = m.CleanOne(ctx, accessor.GetName(), skip)
		var remainingErr *RemainingCRDError
		switch {
		case errors.As(err, &remainingErr):
			remainingCRDs = append(remainingCRDs, accessor.GetName())
		case err != nil:
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

func (m *Manager[T]) Apply(ctx context.Context, manifests resourceapply.AssetFunc, files ...string) error {
	var errs []error

	for _, file := range files {
		objBytes, err := manifests(file)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		requiredObj, _, err := genericCodec.Decode(objBytes, nil, nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = m.applyOne(ctx, requiredObj.(T))
		if err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (m *Manager[T]) applyOne(ctx context.Context, required T) error {
	accessor, err := meta.Accessor(required)
	if err != nil {
		return err
	}
	existing, err := m.client.Get(ctx, accessor.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := m.client.Create(ctx, required, metav1.CreateOptions{})
		klog.Infof("crd %s is created", accessor.GetName())
		return err
	}
	if err != nil {
		return err
	}

	// if existingVersion is higher than the required version, do not update crd.
	accessor, err = meta.Accessor(existing)
	if err != nil {
		return err
	}
	existingVersion, ok := accessor.GetLabels()[versionLabelKey]
	requiredVersion := version.Get().GitVersion
	if ok && versionutil.CompareKubeAwareVersionStrings(requiredVersion, existingVersion) <= 0 {
		return nil
	}

	requiredAccessor, err := meta.Accessor(required)
	if err != nil {
		return err
	}

	labels := requiredAccessor.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[versionLabelKey] = requiredVersion
	requiredAccessor.SetLabels(labels)
	requiredAccessor.SetResourceVersion(accessor.GetResourceVersion())

	_, err = m.client.Update(ctx, required, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.Infof("crd %s is updated to version %s", accessor.GetName(), requiredVersion)

	return nil
}
