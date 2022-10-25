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

kubectl delete -n kube-system deployment --ignore-not-found=true huawei-cloud-controller-manager
kubectl delete -n kube-system --ignore-not-found=true serviceaccount cloud-controller-manager
kubectl delete clusterrole --ignore-not-found=true system:cloud-controller-manager
kubectl delete clusterrolebinding --ignore-not-found=true system:cloud-controller-manager
