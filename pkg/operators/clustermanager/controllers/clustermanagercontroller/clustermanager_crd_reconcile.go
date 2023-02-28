/*
 * Copyright 2022 Contributors to the Open Cluster Management project
 */

package clustermanagercontroller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	operatorapiv1 "open-cluster-management.io/api/operator/v1"
	"open-cluster-management.io/registration-operator/manifests"
	"open-cluster-management.io/registration-operator/pkg/helpers"
	"open-cluster-management.io/registration-operator/pkg/operators/clustermanager/controllers/migrationcontroller"
	"open-cluster-management.io/registration-operator/pkg/operators/crdmanager"
	migrationclient "sigs.k8s.io/kube-storage-version-migrator/pkg/clients/clientset/typed/migration/v1alpha1"
)

var (
	// crdNames is the list of CRDs to be wiped out before deleting other resources when clusterManager is deleted.
	// The order of the list matters, the managedclusteraddon crd needs to be deleted at first so all addon related
	// manifestwork is deleted, then other manifestworks.
	crdNames = []string{
		"managedclusteraddons.addon.open-cluster-management.io",
		"manifestworks.work.open-cluster-management.io",
		"managedclusters.cluster.open-cluster-management.io",
	}

	// crdResourceFiles should be deployed in the hub cluster
	hubCRDResourceFiles = []string{
		"cluster-manager/hub/0000_00_addon.open-cluster-management.io_clustermanagementaddons.crd.yaml",
		"cluster-manager/hub/0000_00_clusters.open-cluster-management.io_managedclusters.crd.yaml",
		"cluster-manager/hub/0000_00_clusters.open-cluster-management.io_managedclustersets.crd.yaml",
		"cluster-manager/hub/0000_00_work.open-cluster-management.io_manifestworks.crd.yaml",
		"cluster-manager/hub/0000_01_addon.open-cluster-management.io_managedclusteraddons.crd.yaml",
		"cluster-manager/hub/0000_01_clusters.open-cluster-management.io_managedclustersetbindings.crd.yaml",
		"cluster-manager/hub/0000_02_clusters.open-cluster-management.io_placements.crd.yaml",
		"cluster-manager/hub/0000_02_addon.open-cluster-management.io_addondeploymentconfigs.crd.yaml",
		"cluster-manager/hub/0000_03_clusters.open-cluster-management.io_placementdecisions.crd.yaml",
		"cluster-manager/hub/0000_05_clusters.open-cluster-management.io_addonplacementscores.crd.yaml",
	}

	// removed CRD StoredVersions
	removedCRDStoredVersions = map[string]string{
		"managedclustersets.cluster.open-cluster-management.io":        "v1beta1",
		"managedclustersetbindings.cluster.open-cluster-management.io": "v1beta1",
	}
	// latest required CRD StoredVersions
	requiredCRDStoredVersion = map[string]string{
		"managedclustersets.cluster.open-cluster-management.io":        "v1beta2",
		"managedclustersetbindings.cluster.open-cluster-management.io": "v1beta2",
	}
)

type crdReconcile struct {
	hubAPIExtensionClient apiextensionsclient.Interface
	hubMigrationClient    migrationclient.StorageVersionMigrationsGetter
	skipRemoveCRDs        bool

	cache    resourceapply.ResourceCache
	recorder events.Recorder
}

func (c *crdReconcile) reconcile(ctx context.Context, cm *operatorapiv1.ClusterManager, config manifests.HubConfig) (*operatorapiv1.ClusterManager, reconcileState, error) {
	crdManager := crdmanager.NewManager[*apiextensionsv1.CustomResourceDefinition](
		c.hubAPIExtensionClient.ApiextensionsV1().CustomResourceDefinitions(),
		crdmanager.EqualV1,
	)

	if err := crdManager.Apply(ctx,
		func(name string) ([]byte, error) {
			template, err := manifests.ClusterManagerManifestFiles.ReadFile(name)
			if err != nil {
				return nil, err
			}
			objData := assets.MustCreateAssetFromTemplate(name, template, config).Data
			helpers.SetRelatedResourcesStatusesWithObj(&cm.Status.RelatedResources, objData)
			return objData, nil
		},
		hubCRDResourceFiles...); err != nil {
		meta.SetStatusCondition(&cm.Status.Conditions, metav1.Condition{
			Type:    clusterManagerApplied,
			Status:  metav1.ConditionFalse,
			Reason:  "CRDApplyFaild",
			Message: fmt.Sprintf("Failed to apply crd: %v", err),
		})
		return cm, reconcileStop, err
	}

	// update CRD StoredVersion
	if err := c.updateStoredVersion(ctx); err != nil {
		meta.SetStatusCondition(&cm.Status.Conditions, metav1.Condition{
			Type:    clusterManagerApplied,
			Status:  metav1.ConditionFalse,
			Reason:  "CRDStoredVersionUpdateFailed",
			Message: fmt.Sprintf("Failed to update crd stored version: %v", err),
		})
		return cm, reconcileStop, err
	}

	return cm, reconcileContinue, nil
}

func (c *crdReconcile) clean(ctx context.Context, cm *operatorapiv1.ClusterManager, config manifests.HubConfig) (*operatorapiv1.ClusterManager, reconcileState, error) {
	crdManager := crdmanager.NewManager[*apiextensionsv1.CustomResourceDefinition](
		c.hubAPIExtensionClient.ApiextensionsV1().CustomResourceDefinitions(),
		crdmanager.EqualV1,
	)

	// Remove crds in order at first
	for _, name := range crdNames {
		if err := crdManager.CleanOne(ctx, name, c.skipRemoveCRDs); err != nil {
			return cm, reconcileStop, err
		}
		c.recorder.Eventf("CRDDeleted", "crd %s is deleted", name)
	}
	if c.skipRemoveCRDs {
		return cm, reconcileContinue, nil
	}

	if err := crdManager.Clean(ctx, c.skipRemoveCRDs,
		func(name string) ([]byte, error) {
			template, err := manifests.ClusterManagerManifestFiles.ReadFile(name)
			if err != nil {
				return nil, err
			}
			objData := assets.MustCreateAssetFromTemplate(name, template, config).Data
			helpers.SetRelatedResourcesStatusesWithObj(&cm.Status.RelatedResources, objData)
			return objData, nil
		},
		hubCRDResourceFiles...); err != nil {
		return cm, reconcileStop, err
	}

	return cm, reconcileContinue, nil
}

// updateStoredVersion update(remove) deleted api version from CRD status.StoredVersions
func (c *crdReconcile) updateStoredVersion(ctx context.Context) error {
	//Check if current env support StorageVersionMigration
	supported, err := migrationcontroller.SupportStorageVersionMigration(ctx, c.hubAPIExtensionClient)
	if err != nil {
		return err
	}

	for name, version := range removedCRDStoredVersions {
		needMigrate := migrationcontroller.NeedMigrate(name)
		if supported && needMigrate {
			// Check migration status before update CRD stored version
			// If CRD's StorageVersionMigration succeed, it means that the current CRs were migrated successfully, and can continue to update the CRD's stored version.
			// If CRD's StorageVersionMigration is not found, it means StorageVersionMigration cr is not created. need wait it created.
			// If the migration failed, we should not contiue to update the stored version, that will caused the stored old version CRs inconsistent with latest CRD.
			migrateSucceed, err := migrationcontroller.IsStorageVersionMigrationSucceeded(c.hubMigrationClient, name)
			if migrateSucceed == false {
				if err != nil && !errors.IsNotFound(err) {
					return fmt.Errorf("failed to execute StorageVersionMigrations %v: %v", name, err)
				}
				klog.Infof("Wait StorageVersionMigrations %v succeed", name)
				continue
			}
		}
		// retrieve CRD
		crd, err := c.hubAPIExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			klog.Warningf("faield to get CRD %v: %v", crd.Name, err)
			continue
		}

		oldStoredVersions := crd.Status.StoredVersions

		newStoredVersions := desiredStoredVersions(oldStoredVersions, requiredCRDStoredVersion[name], version)

		if !reflect.DeepEqual(oldStoredVersions, newStoredVersions) {
			crd.Status.StoredVersions = newStoredVersions
			// update the status sub-resource
			crd, err = c.hubAPIExtensionClient.ApiextensionsV1().CustomResourceDefinitions().UpdateStatus(ctx, crd, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			klog.V(4).Infof("updated CRD %v status storedVersions: %v", crd.Name, crd.Status.StoredVersions)
		}
	}

	return nil
}

// desiredStoredVersions caculate the desired stored versions
// If the "requiredStoredVersion" do not exist, do not update the current stored versions
// If the "requiredStoredVersion" exist, delete the "removedCRDStoredVersion"
func desiredStoredVersions(curStoredVersion []string, requiredStoredVersion, removedCRDStoredVersion string) []string {
	// remove old versions from its status
	newStoredVersions := make([]string, 0, len(curStoredVersion))
	if len(removedCRDStoredVersion) == 0 {
		return curStoredVersion
	}

	//If required version do not exist, do not update the storedversion
	requiredVersionExist := false
	if len(requiredStoredVersion) != 0 {
		for _, stored := range curStoredVersion {
			if stored == requiredStoredVersion {
				requiredVersionExist = true
			}
		}
		if !requiredVersionExist {
			return curStoredVersion
		}
	}

	for _, stored := range curStoredVersion {
		if stored != removedCRDStoredVersion {
			newStoredVersions = append(newStoredVersions, stored)
		}
	}
	return newStoredVersions
}
