/*
 * Copyright 2022 Contributors to the Open Cluster Management project
 */

package clustermanagercontroller

import (
	"context"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	fakeapiextensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	migrationv1alpha1 "sigs.k8s.io/kube-storage-version-migrator/pkg/apis/migration/v1alpha1"
	fakemigrationclient "sigs.k8s.io/kube-storage-version-migrator/pkg/clients/clientset/fake"
)

func Test_crdReconcile_updateStoredVersion(t *testing.T) {
	tests := []struct {
		name             string
		wantErr          bool
		obj              []runtime.Object
		migrateCR        []runtime.Object
		expStoredVersion []string
	}{
		{
			name:    "Do not support migrate crd",
			wantErr: false,
			obj: []runtime.Object{
				&apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "clustermanagementaddons.addon.open-cluster-management.io",
					},
				},
			},
			expStoredVersion: []string{},
		},
		{
			name:    "Support migrate crd",
			wantErr: false,
			obj: []runtime.Object{
				&apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "storageversionmigrations.migration.k8s.io",
					},
				},
			},
			expStoredVersion: []string{},
		},
		{
			name:    "Migrate clusterset crd",
			wantErr: false,
			obj: []runtime.Object{
				&apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "storageversionmigrations.migration.k8s.io",
					},
				},
				&apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "managedclustersets.cluster.open-cluster-management.io",
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						StoredVersions: []string{
							"v1beta1",
							"v1beta2",
						},
					},
				},
				&apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "managedclustersetbindings.cluster.open-cluster-management.io",
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						StoredVersions: []string{
							"v1beta1",
							"v1beta2",
						},
					},
				},
			},
			migrateCR: []runtime.Object{
				&migrationv1alpha1.StorageVersionMigration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "managedclustersets.cluster.open-cluster-management.io",
					},
					Status: migrationv1alpha1.StorageVersionMigrationStatus{
						Conditions: []migrationv1alpha1.MigrationCondition{
							{
								Type:   migrationv1alpha1.MigrationSucceeded,
								Status: v1.ConditionTrue,
							},
						},
					},
				},
			},
			expStoredVersion: []string{"v1beta1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeAPIExtensionClient := fakeapiextensions.NewSimpleClientset(tt.obj...)
			fakeMigrationClient := fakemigrationclient.NewSimpleClientset(tt.migrateCR...)
			c := &crdReconcile{
				hubAPIExtensionClient: fakeAPIExtensionClient,
				hubMigrationClient:    fakeMigrationClient.MigrationV1alpha1(),
			}
			if err := c.updateStoredVersion(context.Background()); (err != nil) != tt.wantErr {
				t.Errorf("crdReconcile.updateStoredVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
			setcrd, err := c.hubAPIExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "managedclustersets.cluster.open-cluster-management.io", metav1.GetOptions{})
			if err != nil && !errors.IsNotFound(err) {
				t.Errorf("Get crd error: %v", err)
			}
			if err == nil && reflect.DeepEqual(setcrd.Status.StoredVersions, tt.expStoredVersion) {
				t.Errorf("Stored version is not equal: %v, %v", setcrd.Status.StoredVersions, tt.expStoredVersion)
			}
		})
	}
}

func Test_desiredStoredVersions(t *testing.T) {
	tests := []struct {
		name                    string
		curStoredVersion        []string
		requiredStoredVersion   string
		removedCRDStoredVersion string
		want                    []string
	}{
		{
			name:                    "current stored version is empty",
			curStoredVersion:        []string{},
			requiredStoredVersion:   "v1beta1",
			removedCRDStoredVersion: "v1beta2",
			want:                    []string{},
		},
		{
			name:                    "required stored version is empty",
			curStoredVersion:        []string{"v1beta1"},
			requiredStoredVersion:   "",
			removedCRDStoredVersion: "v1beta2",
			want:                    []string{"v1beta1"},
		},
		{
			name:                    "removed stored version is empty",
			curStoredVersion:        []string{"v1beta1"},
			requiredStoredVersion:   "v1beta1",
			removedCRDStoredVersion: "",
			want:                    []string{"v1beta1"},
		},
		{
			name:                    "before upgrade case ",
			curStoredVersion:        []string{"v1beta1"},
			requiredStoredVersion:   "v1beta2",
			removedCRDStoredVersion: "v1beta1",
			want:                    []string{"v1beta1"},
		},
		{
			name:                    "after upgrade case",
			curStoredVersion:        []string{"v1beta1", "v1beta2"},
			requiredStoredVersion:   "v1beta2",
			removedCRDStoredVersion: "v1beta1",
			want:                    []string{"v1beta2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := desiredStoredVersions(tt.curStoredVersion, tt.requiredStoredVersion, tt.removedCRDStoredVersion); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("case: %v, desiredStoredVersions() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
