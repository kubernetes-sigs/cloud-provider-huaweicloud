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

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

SOURCES := $(shell find ./cmd ./pkg -type f -name '*.go')
LDFLAGS := ""

# Images management
REGISTRY_USERNAME       ?=
REGISTRY_PASSWORD       ?=
REGISTRY_SERVER_ADDRESS ?=
REGISTRY                ?= $(REGISTRY_SERVER_ADDRESS)/k8s-cloudprovider

IMAGE   := $(REGISTRY)/huawei-cloud-controller-manager
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "latest")

DOCKERFILE := cluster/images/cloud-controller-manager/Dockerfile

.PHONY: build
build: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags $(LDFLAGS) \
		-o huawei-cloud-controller-manager \
		cmd/cloud-controller-manager/cloud-controller-manager.go

# NOTE: Huawei Cloud SWR Basic Edition does not support OCI format. Add
# --provenance=false to force Docker native manifests, otherwise manifest push
# will fail with a parse error.

.PHONY: images
images: docker-login image-amd64 image-arm64

.PHONY: release
release: docker-login
	docker buildx build \
		--platform $(GOOS)/amd64,$(GOOS)/arm64 \
		--provenance=false \
		--output type=image,oci-mediatypes=false,push=true \
		-t $(IMAGE):$(VERSION) \
		-f $(DOCKERFILE) \
		.

.PHONY: docker-login
docker-login:
	@echo ":: Login to $(REGISTRY_SERVER_ADDRESS) ::"
	@if [ -n "$(REGISTRY_USERNAME)" ] && [ -n "$(REGISTRY_PASSWORD)" ]; then \
		docker login -u "$(REGISTRY_USERNAME)" -p "$(REGISTRY_PASSWORD)" "$(REGISTRY_SERVER_ADDRESS)"; \
	else \
		echo "Skipping login: username or password missing"; \
	fi

.PHONY: image-amd64
image-amd64: docker-login
	docker buildx build \
		--provenance=false \
		--output type=image,oci-mediatypes=false,push=true \
		--platform $(GOOS)/amd64 \
		-f $(DOCKERFILE) \
		-t $(IMAGE):$(VERSION)-amd64 \
		.

.PHONY: image-arm64
image-arm64: docker-login
	docker buildx build \
		--provenance=false \
		--output type=image,oci-mediatypes=false,push=true \
		--platform $(GOOS)/arm64 \
		-f $(DOCKERFILE) \
		-t $(IMAGE):$(VERSION)-arm64 \
		.

.PHONY: verify
verify:
	hack/verify.sh

.PHONY: test
test:
	go test ./pkg/...
