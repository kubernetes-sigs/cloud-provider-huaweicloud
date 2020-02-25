# Kubernetes Cloud Provider for HUAWEI CLOUD
This repository contains the [Kubernetes cloud-controller-manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/) for HUAWEICLOUD.

Successfully running cloud-controller-manager requires some changes to your cluster configuration.
- `kube-apiserver` and `kube-controller-manager` MUST NOT specify the `--cloud-provider` flag
(or specify `--cloud-provider=external`). This ensures that it does not run any cloud specific loops that would be run by cloud controller manager. 
- `kubelet` must run with `--cloud-provider=external`. This is to ensure that the kubelet is aware that it must be initialized by the cloud controller manager before it is scheduled any work.

## Quick Start
- [Run with local cluster](./docs/quick-start-with-local-cluster.md)

## Development

#### Dependency management
Go version should be 1.13+, and the dependencies are managed using [Go modules](https://github.com/golang/go/wiki/Modules).
To keep it simple, you can use vendor as well.

#### build locally
```
$ git clone https://github.com/huawei-cloudnative/cloud-provider-huaweicloud.git 
$ make huawei-cloud-controller-manager
```

## More about CCM
- [Concepts Underlying the Cloud Controller Manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/)
- [Running cloud controller manager](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/#running-cloud-controller-manager)
- [Developing Cloud Controller Manager](https://kubernetes.io/docs/tasks/administer-cluster/developing-cloud-controller-manager/)

## Support
Any questions feel free to [send an issue](https://github.com/huawei-cloudnative/cloud-provider-huaweicloud/issues/new).  