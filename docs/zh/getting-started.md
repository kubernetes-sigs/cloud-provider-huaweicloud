# 在华为云上已有的 Kubernetes 集群中运行

> [English](../getting-started.md) | [中文](./getting-started.md)

## 前置条件

- 已在华为云上创建的 Kubernetes 集群（自建或 CCE）。
- 已获取华为云账号凭证：
  - AK（Access Key）/ SK（Secret Key），可在华为云控制台「我的凭证」页面获取。
  - 集群所在区域（Region），例如 `cn-north-4`。
  - 项目 ID（Project ID），可在华为云控制台「我的凭证」页面获取。
  - VPC ID、子网 ID（Subnet ID），可在 VPC 控制台查看。
- IAM 权限：CCM 使用的 AK/SK 需要具备 ELB、ECS、VPC、EVS、EIP、IMS 等相关权限，详见 [IAM 权限策略](./iam-policy.md)。
- 安全组：集群节点安全组需放行 ELB 健康检查网段 `100.125.0.0/16`。

## 更新集群配置

- 为 kube-controller-manager 和 kube-apiserver 添加 `--cloud-provider=external` 配置

`kube-apiserver` 和 `kube-controller-manager` **不能**指定 `--cloud-provider` 标志
（或指定 `--cloud-provider=external`）。这确保它们不会运行应由 Cloud Controller Manager 运行的云特定循环。

- 为每个节点的 kubelet 添加 `--cloud-provider=external` 配置

`kubelet` 必须以 `--cloud-provider=external` 运行。这确保 kubelet 知道它必须由 Cloud Controller Manager 初始化后才能被调度工作。

## 步骤 1：创建 cloud-config Secret

### 1.1 创建 cloud-config 文件

在控制节点创建 `cloud-config` 文件（请替换尖括号中的内容为你的实际值）：

```shell
cat <<EOF > ./cloud-config
[Global]
region=<your-region>                    # 例如: cn-north-4
access-key=<your-access-key>            # 华为云 AK
secret-key=<your-secret-key>            # 华为云 SK
project-id=<your-project-id>            # 项目 ID（可选，但建议填写）

[Vpc]
id=<your-vpc-id>                        # 集群所在 VPC ID
subnet-id=<your-subnet-id>              # 集群所在子网 ID（IPv4）
security-group-id=<your-security-group-id>  # 安全组 ID（可选）
EOF

# 检查文件是否存在
ls -l | grep cloud-config
```

> 有关支持的全部参数说明，请参阅 [华为云 Cloud Controller Manager 配置说明](./huawei-cloud-controller-manager-configuration.md)。

### 1.2 在 Kubernetes 中创建 Secret

```shell
kubectl create secret -n kube-system generic cloud-config --from-file=./cloud-config

# 验证 Secret 已创建
kubectl get secret cloud-config -n kube-system

# 查看详细信息
kubectl describe secret cloud-config -n kube-system
```

预期输出：
```
NAME           TYPE     DATA   AGE
cloud-config   Opaque   1      5s
```

```
Name:         cloud-config
Namespace:    kube-system
Labels:       <none>
Annotations:  <none>

Type:  Opaque

Data
====
cloud-config:  290 bytes
```

### 1.3 删除本地 cloud-config 文件

```shell
rm -rf ./cloud-config
```

> **注意**：修改 cloud-config Secret 后，需要重启 CCM 才能加载新配置。

## 步骤 2：创建 RBAC 资源

```shell
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/master/manifests/rbac-huawei-cloud-controller-manager.yaml
```

## 步骤 3：部署 CCM

**方式 A：使用 Deployment（推荐）**

```shell
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/master/manifests/huawei-cloud-controller-manager-deployment.yaml
```

**方式 B：使用 DaemonSet**

```shell
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/master/manifests/huawei-cloud-controller-manager-daemonset.yaml
```

## 步骤 4：验证安装

```shell
kubectl get pod -n kube-system | grep huawei-cloud-controller-manager
```

预期输出（Pod 状态为 `Running` 表示安装成功）：
```
huawei-cloud-controller-manager-5f4b7995fc-s6b7p   1/1     Running   0          2m36s
```

## 下一步

- [Service Annotations](./usage-guide.md) — 所有支持的 Service 注解参考。
- [使用示例](./usage-examples.md) — 从基础到高级 ELB 场景的完整可执行示例。
- [CCM 配置说明](./huawei-cloud-controller-manager-configuration.md) — cloud-config 和 loadbalancer-config 的完整参考。
- [IAM 权限策略](./iam-policy.md) — CCM 所需的最小 IAM 权限。
