#!/usr/bin/env bash

# Copyright 2022 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

export REGISTRY_SERVER_ADDRESS=swr.ap-southeast-1.myhuaweicloud.com
export REGISTRY=${REGISTRY_SERVER_ADDRESS}/cloud-native
export VERSION=v$(echo $RANDOM | sha1sum |cut -c 1-5)

echo -e "\n:::::: Check KUBECONFIG ::::::"
echo "KUBECONFIG="$KUBECONFIG

echo -e "\n:::::: Check cloud-config secre ::::::t"
count=$(kubectl get -n kube-system secret | grep cloud-config | wc -l)
if [[ "$count" -ne 1 ]]; then
  echo ":::::: Please create the cloud-config secret."
  exit 1
fi

echo -e "\n:::::: Build images ::::::"
# todo: Maybe we need load the image to target cluster node.
make image-huawei-cloud-controller-manager

tmpPath=$(mktemp -d)
is_containerd=$(command -v containerd)
echo "is_containerd: ${is_containerd}"
if [[ -x ${is_containerd} ]]; then
  docker save -o "${tmpPath}/huawei-cloud-controller-manager.tar" ${REGISTRY}/huawei-cloud-controller-manager:${VERSION}
  ctr -n=k8s.io i import ${tmpPath}/huawei-cloud-controller-manager.tar
  rm -rf ${tmpPath}/huawei-cloud-controller-manager.tar
fi

# Remove the existing provider if it exists.
kubectl delete -n kube-system deployment --ignore-not-found=true huawei-cloud-controller-manager

echo -e "\n:::::: Deploy huawei-cloud-controller-manager ::::::"

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
image_url=${REGISTRY}/huawei-cloud-controller-manager:${VERSION}
cp -rf ${REPO_ROOT}/hack/deploy/huawei-cloud-controller-manager.yaml ${tmpPath}

sed -i'' -e "s|{{image_url}}|${image_url}|g" "${tmpPath}"/huawei-cloud-controller-manager.yaml

kubectl apply -f "${tmpPath}/huawei-cloud-controller-manager.yaml"
rm -rf "${tmpPath}/huawei-cloud-controller-manager.yaml"

kubectl rollout status deployment huawei-cloud-controller-manager -n kube-system
