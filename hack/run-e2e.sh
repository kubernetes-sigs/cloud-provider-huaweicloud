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

kubectl version

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

ARTIFACTS_PATH=${ARTIFACTS_PATH:-"${REPO_ROOT}/e2e-logs"}
mkdir -p "${ARTIFACTS_PATH}"

# Install ginkgo
GO111MODULE=on go install github.com/onsi/ginkgo/v2/ginkgo@v2.6.1
GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export PATH=$PATH:$GOPATH/bin

# Pre run e2e for extra components
echo -e "\nRun pre run e2e"
"${REPO_ROOT}"/hack/pre-run-e2e.sh

# Run e2e
echo -e "\nRun e2e"
set +e
ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/
TESTING_RESULT=$?

# Collect logs
kubectl logs deployment/huawei-cloud-controller-manager -n kube-system > ${ARTIFACTS_PATH}/huawei-cloud-controller-manager.log
echo -e "\nCollected logs at ${ARTIFACTS_PATH}:"

# Post run e2e for delete extra components
echo -e "\nRun post run e2e"
"${REPO_ROOT}"/hack/post-run-e2e.sh

exit $TESTING_RESULT
