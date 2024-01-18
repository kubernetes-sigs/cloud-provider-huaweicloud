# Kubernetes Cloud Provider for Huawei Cloud

The Huawei Cloud Controller Manager provides the interface between a Kubernetes cluster and Huawei Cloud service APIs. 
This project allows a Kubernetes cluster to provision, monitor, and remove Huawei Cloud resources necessary for the operation of the cluster.

See [Cloud Controller Manager Administration](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/)
for more about Kubernetes cloud controller manager.

## Implementation Details

Currently `huawei-cloud-controller-manager` implements:

* servicecontroller - responsible for creating LoadBalancers when a service of `Type: LoadBalancer` is created in Kubernetes.
* nodecontroller - updates nodes with cloud provider specific labels and addresses.

## Compatibility with Kubernetes

| Kubernetes Version | Latest Huawei Cloud Controller Manager Version |
| ------------------ |------------------------------------------------|
| v1.20              | v0.20.4                                        |
| v1.21              | v0.21.4                                        |
| v1.22              | v0.22.4                                        |
| v1.23              | v0.23.4                                        |
| v1.24              | v0.24.4                                        |
| v1.25              | v0.25.4                                        |
| v1.26              | v0.26.6                                        |

## Quick Start

- [Running on an Existing Kubernetes Cluster on Huawei Cloud](/docs/getting-started.md)
- [Huawei Cloud Controller Manager Configurations](/docs/huawei-cloud-controller-manager-configuration.md)
- [Usage Guide](/docs/usage-guide.md)
- [IAM Policy](/docs/iam-policy.md) for Kubernetes Cloud Provider on Huawei Cloud.

## More About Cloud Controller Manager

- [Concepts Underlying the Cloud Controller Manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/)
- [Running cloud controller manager](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/#running-cloud-controller-manager)
- [Developing Cloud Controller Manager](https://kubernetes.io/docs/tasks/administer-cluster/developing-cloud-controller-manager/)

## Support

Any questions feel free to [submit an issue](https://github.com/kubernetes-sigs/cloud-provider-huaweicloud/issues/new).
