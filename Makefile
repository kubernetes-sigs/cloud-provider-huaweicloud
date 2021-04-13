# Copyright 2020 The Kubernetes Authors.
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

GOOS ?= $(shell go env GOOS)
SOURCES := $(shell find . -type f  -name '*.go')
LDFLAGS := ""

# Images management
REGISTRY_REGION?="ap-southeast-1"
ACCESS_KEY?=""
REGISTRY_LOGIN_KEY?=""
SWR_SERVICE_ADDRESS?="swr.ap-southeast-1.myhuaweicloud.com"
REGISTRY?="${SWR_SERVICE_ADDRESS}/k8scloudcontrollermanager"

# Set you version by env or using latest tags from git
VERSION?=$(shell git describe --tags)

all: huawei-cloud-controller-manager

huawei-cloud-controller-manager: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o huawei-cloud-controller-manager \
		cmd/cloud-controller-manager/cloud-controller-manager.go

clean:
	rm -rf huawei-cloud-controller-manager

verify:
	hack/verify.sh

.PHONY: test
test:
	go test ./pkg/...

images: image-huawei-cloud-controller-manager

image-huawei-cloud-controller-manager: huawei-cloud-controller-manager
	cp huawei-cloud-controller-manager cluster/images/cloud-controller-manager && \
	docker build -t $(REGISTRY)/huawei-cloud-controller-manager:$(VERSION) cluster/images/cloud-controller-manager && \
	rm cluster/images/cloud-controller-manager/huawei-cloud-controller-manager

upload-images: images
	@echo "push images to $(REGISTRY)"
	docker login -u ${REGISTRY_REGION}@${ACCESS_KEY} -p ${REGISTRY_LOGIN_KEY} ${SWR_SERVICE_ADDRESS}
	docker push ${REGISTRY}/huawei-cloud-controller-manager:${VERSION}