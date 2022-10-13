module sigs.k8s.io/cloud-provider-huaweicloud

go 1.15

require (
	github.com/fsnotify/fsnotify v1.4.9
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/hashicorp/golang-lru v0.5.1
	github.com/huaweicloud/huaweicloud-sdk-go-v3 v0.0.6-beta
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.18.1
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	golang.org/x/net v0.0.0-20220722155237-a158d28d115b // indirect
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4 // indirect
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	k8s.io/api v0.19.14
	k8s.io/apimachinery v0.19.14
	k8s.io/client-go v0.19.14
	k8s.io/cloud-provider v0.19.14
	k8s.io/component-base v0.19.14
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.19.14
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
)

replace (
	k8s.io/api => k8s.io/api v0.19.14
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.14
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.14
	k8s.io/apiserver => k8s.io/apiserver v0.19.14
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.14
	k8s.io/client-go => k8s.io/client-go v0.19.14
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.14
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.14
	k8s.io/code-generator => k8s.io/code-generator v0.19.14
	k8s.io/component-base => k8s.io/component-base v0.19.14
	k8s.io/cri-api => k8s.io/cri-api v0.19.14
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.14
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.14
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.14
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.14
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.14
	k8s.io/kubectl => k8s.io/kubectl v0.19.14
	k8s.io/kubelet => k8s.io/kubelet v0.19.14
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.14
	k8s.io/metrics => k8s.io/metrics v0.19.14
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.14
)
