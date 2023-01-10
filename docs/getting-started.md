# Running on an Existing Kubernetes Cluster on Huawei Cloud

## Prerequisites

- Kubernetes cluster on Huawei Cloud, version range v1.20 ~ v1.23.

## Update cluster configurations

- Add `--cloud-provider=external` to kube-controller-manager and kube-apiserver config

`kube-apiserver` and `kube-controller-manager` **MUST NOT** specify the `--cloud-provider` flag
(or specify `--cloud-provider=external`). This ensures that it does not run any cloud specific loops that would be run
by cloud controller manager.

- Add `--cloud-provider=external` to kubelet on each node

`kubelet` MUST run with `--cloud-provider=external`. This is to ensure that the kubelet is aware that it must be
initialized by the cloud controller manager before it is scheduled any work.

## Install Cloud Provider for Huawei Cloud

- Create the `cloud-config` secret in Kubernetes cluster

Create the `cloud-config` file according to [cloud-config](../manifests/cloud-config) in master node or control-plane,
see [Huawei Cloud Controller Manager Configurations](./huawei-cloud-controller-manager-configuration.md)
for configurations description.

Use the following command create `cloud-config` secret:

```shell
kubectl create secret -n kube-system generic cloud-config --from-file=./cloud-config
```

- Create RBAC resources

```shell
kubectl apply -f  https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/release-1.20/manifests/rbac-huawei-cloud-controller-manager.yaml
```

- Install the Huawei Cloud Provider Manager

```shell
kubectl apply -f  https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/release-1.20/manifests/huawei-cloud-controller-manager.yaml
```

- Check the running status

```shell
# kubectl get pod -n kube-system | grep huawei-cloud-controller-manager
huawei-cloud-controller-manager-5f4b7995fc-s6b7p   1/1     Running   0          2m36s

```

When the status of Pod `huawei-cloud-controller-manager` is `running`, the installation is successful.

## What's next

Refer to [Usage Guide](./usage-guide.md) for usage examples.
