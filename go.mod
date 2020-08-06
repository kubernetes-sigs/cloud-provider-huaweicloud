module sigs.k8s.io/cloud-provider-huaweicloud

go 1.13

require (
	github.com/RainbowMango/huaweicloud-sdk-go v0.0.0
	github.com/fsnotify/fsnotify v1.4.7
	github.com/hashicorp/golang-lru v0.5.1
	github.com/huaweicloud/golangsdk v0.0.0-20200728004128-63646e8b1ea7
	github.com/mitchellh/mapstructure v1.1.2
	k8s.io/api v0.17.5
	k8s.io/apimachinery v0.17.5
	k8s.io/client-go v0.17.5
	k8s.io/cloud-provider v0.17.5
	k8s.io/component-base v0.17.5
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.17.5
)

replace (
	github.com/RainbowMango/huaweicloud-sdk-go => github.com/RainbowMango/huaweicloud-sdk-go v1.0.21-0.20200208034621-214008016257
	k8s.io/api => k8s.io/api v0.17.5
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.5
	k8s.io/apiserver => k8s.io/apiserver v0.17.5
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.17.5
	k8s.io/client-go => k8s.io/client-go v0.17.5
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.17.5
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.17.5
	k8s.io/code-generator => k8s.io/code-generator v0.17.5
	k8s.io/component-base => k8s.io/component-base v0.17.5
	k8s.io/cri-api => k8s.io/cri-api v0.17.5
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.17.5
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.17.5
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.17.5
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.17.5
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.17.5
	k8s.io/kubectl => k8s.io/kubectl v0.17.5
	k8s.io/kubelet => k8s.io/kubelet v0.17.5
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.17.5
	k8s.io/metrics => k8s.io/metrics v0.17.5
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.17.5
)
