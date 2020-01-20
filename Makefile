
GOOS ?= $(shell go env GOOS)
SOURCES := $(shell find . -type f  -name '*.go')
LDFLAGS := ""

huawei-cloud-controller-manager: $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGS) \
		-o huawei-cloud-controller-manager \
		cmd/cloud-controller-manager/cloud-controller-manager.go

clean:
	rm -rf huawei-cloud-controller-manager

verify:
	hack/verify.sh