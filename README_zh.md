# 华为云 Kubernetes Cloud Provider

> [English](./README.md) | [中文](./README_zh.md)

华为云 Cloud Controller Manager 提供 Kubernetes 集群与华为云服务 API 之间的接口。
本项目允许 Kubernetes 集群创建、监控和删除运行集群所需的华为云资源。

有关 Kubernetes Cloud Controller Manager 的更多信息，请参阅 [Cloud Controller Manager 管理](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/)。

## 实现详情

目前 `huawei-cloud-controller-manager` 实现了：

* **servicecontroller** — 当 Kubernetes 中创建 `Type: LoadBalancer` 类型的 Service 时，负责创建负载均衡器。
* **nodecontroller** — 使用云提供商特定的标签和地址更新节点信息。

## Kubernetes 兼容性

| Kubernetes 版本 | 华为云 Cloud Controller Manager 最新版本 |
|--------------------|------------------------------------------------|
| v1.20              | v0.20.4                                        |
| v1.21              | v0.21.4                                        |
| v1.22              | v0.22.4                                        |
| v1.23              | v0.23.4                                        |
| v1.24              | v0.24.4                                        |
| v1.25              | v0.25.4                                        |
| v1.26              | v0.26.9                                        |
| v1.35              | v0.26.10                                       |

## 文档索引

### 安装

- [快速入门](./docs/zh/getting-started.md) — 在华为云上已有的 Kubernetes 集群中安装 CCM。

### 配置

- [cloud-config（Global & Vpc）](./docs/zh/huawei-cloud-controller-manager-configuration.md#华为云配置) — 存储在 `cloud-config` Secret 中的华为云认证和网络配置。
- [loadbalancer-config](./docs/zh/huawei-cloud-controller-manager-configuration.md#负载均衡器配置) — 存储在 `loadbalancer-config` ConfigMap 中的全局 ELB 默认配置。
- [IAM 权限策略](./docs/zh/iam-policy.md) — CCM 所需的最小 IAM 权限。

### 使用

- [Service Annotations](./docs/zh/usage-guide.md) — 所有支持的 Service 注解参考。
- [使用示例](./docs/zh/usage-examples.md) — 完整的可执行示例（共享型/独享型 ELB、EIP、会话保持、健康检查、TLS、跨 VPC、多端口等）。

## 更多关于 Cloud Controller Manager

- [Cloud Controller Manager 概念](https://kubernetes.io/docs/concepts/architecture/cloud-controller/)
- [运行 Cloud Controller Manager](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/#running-cloud-controller-manager)
- [开发 Cloud Controller Manager](https://kubernetes.io/docs/tasks/administer-cluster/developing-cloud-controller-manager/)

## 支持

如有任何问题，欢迎[提交 Issue](https://github.com/kubernetes-sigs/cloud-provider-huaweicloud/issues/new)。
