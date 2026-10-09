# Kubernetes Cloud Provider for Huawei Cloud

> [English](./README.md) | [中文](./README_zh.md)

The Huawei Cloud Controller Manager provides the interface between a Kubernetes cluster and Huawei Cloud service APIs. 
This project allows a Kubernetes cluster to provision, monitor, and remove Huawei Cloud resources necessary for the operation of the cluster.

See [Cloud Controller Manager Administration](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/)
for more about Kubernetes cloud controller manager.

## Implementation Details

Currently `huawei-cloud-controller-manager` implements:

* **servicecontroller** — responsible for creating LoadBalancers when a service of `Type: LoadBalancer` is created in Kubernetes.
* **nodecontroller** — updates nodes with cloud provider specific labels and addresses.

## Compatibility with Kubernetes

| Kubernetes Version | Latest Huawei Cloud Controller Manager Version |
|--------------------|------------------------------------------------|
| v1.20              | v0.20.4                                        |
| v1.21              | v0.21.4                                        |
| v1.22              | v0.22.4                                        |
| v1.23              | v0.23.4                                        |
| v1.24              | v0.24.4                                        |
| v1.25              | v0.25.4                                        |
| v1.26              | v0.26.9                                        |
| v1.35              | v0.26.10                                       |

## Documentation Index

### Installation

- [Getting Started](./docs/getting-started.md) — Install CCM on an existing Kubernetes cluster on Huawei Cloud.

### Configuration

- [cloud-config (Global & Vpc)](./docs/huawei-cloud-controller-manager-configuration.md#huawei-cloud-configuration) — Huawei Cloud authentication and network configuration stored in the `cloud-config` Secret.
- [loadbalancer-config](./docs/huawei-cloud-controller-manager-configuration.md#loadbalancer-configuration) — Global ELB defaults stored in the `loadbalancer-config` ConfigMap.
- [IAM Policy](./docs/iam-policy.md) — Minimum IAM permissions required by CCM.

### Usage

- [Service Annotations](./docs/usage-guide.md) — Reference for all supported Service annotations.
- [Usage Examples](./docs/usage-examples.md) — Complete executable examples (shared/dedicated ELB, EIP, session affinity, health checks, TLS, cross-VPC, multi-port, and more).

## More About Cloud Controller Manager

- [Concepts Underlying the Cloud Controller Manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/)
- [Running cloud controller manager](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/#running-cloud-controller-manager)
- [Developing Cloud Controller Manager](https://kubernetes.io/docs/tasks/administer-cluster/developing-cloud-controller-manager/)

## Support

Any questions feel free to [submit an issue](https://github.com/kubernetes-sigs/cloud-provider-huaweicloud/issues/new).
