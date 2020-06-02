#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/init.sh"

for f in $CRD_FILES
do
    cp $f ./manifests/hub/
done

cp $CLUSTER_MANAGER_CRD_FILE ./deploy/clustermanager/crds/
cp $MANAGED_CLUSTER_CRD_FILE ./deploy/klusterlet/crds/
