#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/init.sh"

for f in $CRD_FILES
do
    diff -N $f ./manifests/hub/$(basename $f) || ( echo 'crd content is incorrect' && false )
done

diff -N $CLUSTER_MANAGER_CRD_FILE ./deploy/clustermanager/crds/$(basename $CLUSTER_MANAGER_CRD_FILE) || ( echo 'crd content is incorrect' && false )
diff -N $MANAGED_CLUSTER_CRD_FILE ./deploy/klusterlet/crds/$(basename $MANAGED_CLUSTER_CRD_FILE) || ( echo 'crd content is incorrect' && false )

