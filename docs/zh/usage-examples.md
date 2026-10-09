# 华为云 Kubernetes Cloud Provider 使用示例

本文档提供华为云 Cloud Controller Manager（CCM）的完整可执行示例，涵盖各类 LoadBalancer 服务创建场景。

> 有关安装说明，请参阅[快速入门](./getting-started.md)。

---

## 目录

- [前置条件](#前置条件)
- [通用准备：创建测试 Deployment](#通用准备创建测试-deployment)
- [示例 1：自动创建共享型 ELB 服务](#示例-1自动创建共享型-elb-服务)
- [示例 2：使用已有共享型 ELB 实例](#示例-2使用已有共享型-elb-实例)
- [示例 3：自动创建 EIP 并绑定到 ELB](#示例-3自动创建-eip-并绑定到-elb)
- [示例 4：共享型 ELB 会话保持](#示例-4共享型-elb-会话保持)
- [示例 5：共享型 ELB 自定义健康检查](#示例-5共享型-elb-自定义健康检查)
- [示例 6：共享型 ELB 保留客户端源 IP](#示例-6共享型-elb-保留客户端源-ip)
- [示例 7：创建独享型 ELB 服务](#示例-7创建独享型-elb-服务)
- [示例 8：独享型 ELB + ExternalTrafficPolicy=Local（Pod IP 后端）](#示例-8独享型-elb--externaltrafficpolicylocalpod-ip-后端)
- [示例 9：独享型 ELB 跨 VPC 后端](#示例-9独享型-elb-跨-vpc-后端)
- [示例 10：独享型 ELB 指定规格（Flavor）](#示例-10独享型-elb-指定规格flavor)
- [示例 11：独享型 ELB TLS 终止（HTTPS）](#示例-11独享型-elb-tls-终止https)
- [示例 12：HTTP/HTTPS 超时配置](#示例-12httphttps-超时配置)
- [示例 13：多端口 Service](#示例-13多端口-service)
- [清理资源](#清理资源)
- [全局负载均衡配置（ConfigMap）](#全局负载均衡配置configmap)

---

## 前置条件

在运行以下示例前，请确保已在集群中安装华为云 Cloud Controller Manager。
安装说明请参阅[快速入门](./getting-started.md)。

此外，请确保以下条件：

1. **kubectl 工具**：已安装并可连接到集群。
2. **IAM 权限**：CCM 使用的 AK/SK 需要具备 ELB、ECS、VPC、EVS、EIP、IMS 等相关权限，详见 [IAM 权限策略](./iam-policy.md)。
3. **安全组**：集群节点安全组需放行 ELB 健康检查网段 `100.125.0.0/16`。

> **注意**：以下所有 `kubectl` 命令均要求已配置好 kubeconfig，可以正常访问集群。

---

## 通用准备：创建测试 Deployment

以下示例 1~13 均依赖一个 Nginx Deployment，请先创建：

```shell
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: default
  name: deployment-ccm-test
spec:
  selector:
    matchLabels:
      app: nginx
  replicas: 2
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:1.23
          ports:
            - containerPort: 80
EOF
```

验证 Deployment：
```shell
kubectl get deployment deployment-ccm-test
```

---

## 示例 1：自动创建共享型 ELB 服务

**场景说明**：创建一个 `Type: LoadBalancer` 的 Service，CCM 会自动创建共享型 ELB 实例并将流量转发到后端 Pod。

**前置条件**：已完成 CCM 安装（参阅[快速入门](./getting-started.md)）。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: shared
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
  labels:
    app: nginx
  name: loadbalancer-service-demo-01
  namespace: default
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

验证服务状态（等待 `EXTERNAL-IP` 不再是 `<pending>`）：
```shell
kubectl get service loadbalancer-service-demo-01 -w
```

预期输出：
```
NAME                          TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)        AGE
loadbalancer-service-demo-01  LoadBalancer   10.1.130.216   192.168.0.113   80:30993/TCP   3m10s
```

测试访问：
```shell
curl http://192.168.0.113
```

---

## 示例 2：使用已有共享型 ELB 实例

**场景说明**：复用华为云上已创建好的共享型 ELB 实例，不自动创建新实例。

**前置条件**：
- 已在华为云 ELB 控制台创建了一个共享型 ELB 实例。
- 已获取该 ELB 实例的 ID。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: shared
    kubernetes.io/elb.id: <your-existing-elb-id>   # 替换为已有 ELB 实例 ID
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
  labels:
    app: nginx
  name: loadbalancer-service-demo-02
  namespace: default
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

验证：
```shell
kubectl get service loadbalancer-service-demo-02
```

---

## 示例 3：自动创建 EIP 并绑定到 ELB

**场景说明**：创建共享型 ELB 时，自动创建一个 BGP 类型的 EIP（独享带宽 5Mbit/s）并绑定到 ELB。

**前置条件**：账号下有可用的 EIP 配额。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: shared
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.keep-eip: "false"
    kubernetes.io/elb.eip-auto-create-option: >-
      {"ip_type": "5_bgp", "bandwidth_size": 5, "share_type": "PER"}
  labels:
    app: nginx
  name: loadbalancer-service-demo-03
  namespace: default
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

> **参数说明**：
> - `ip_type: 5_bgp` — 动态 BGP 类型 EIP。
> - `bandwidth_size: 5` — 带宽大小 5Mbit/s。
> - `share_type: PER` — 独享带宽。若使用共享带宽，需设为 `WHOLE` 并额外指定 `share_id`。

验证：
```shell
kubectl get service loadbalancer-service-demo-03
```

预期输出（`EXTERNAL-IP` 为公网 EIP 地址）：
```
NAME                           TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)        AGE
loadbalancer-service-demo-03   LoadBalancer   10.1.35.151   159.138.37.76   80:30080/TCP   41s
```

测试公网访问：
```shell
curl http://159.138.37.76
```

---

## 示例 4：共享型 ELB 会话保持

**场景说明**：为共享型 ELB 启用基于源 IP 的会话保持，同一客户端 IP 的请求会转发到同一后端服务器。

**前置条件**：已完成 CCM 安装（参阅[快速入门](./getting-started.md)）。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: shared
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.session-affinity-flag: 'on'
    kubernetes.io/elb.session-affinity-option: >-
      {"type": "SOURCE_IP", "persistence_timeout": 15}
  labels:
    app: nginx
  name: loadbalancer-service-demo-04
  namespace: default
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

> **参数说明**：
> - `type: SOURCE_IP` — 基于客户端 IP 做会话保持（TCP 协议仅支持此类型）。
> - `persistence_timeout: 15` — 会话保持超时时间 15 分钟（TCP/UDP 范围 1~60，HTTP/HTTPS 范围 1~1440）。

验证：
```shell
kubectl get service loadbalancer-service-demo-04
```

---

## 示例 5：共享型 ELB 自定义健康检查

**场景说明**：自定义 ELB 对后端服务器的健康检查参数，调整检查间隔、超时和重试次数。

**前置条件**：已完成 CCM 安装（参阅[快速入门](./getting-started.md)）。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: shared
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.health-check-flag: 'on'
    kubernetes.io/elb.health-check-option: >-
      {"delay": 5, "timeout": 10, "max_retries": 3}
  labels:
    app: nginx
  name: loadbalancer-service-demo-05
  namespace: default
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

> **参数说明**：
> - `delay: 5` — 每次健康检查间隔 5 秒（范围 1~50）。
> - `timeout: 10` — 健康检查超时 10 秒（范围 1~50）。
> - `max_retries: 3` — 最大重试次数 3 次（范围 1~10）。
> - 请确保节点安全组已放行 `100.125.0.0/16` 网段用于健康检查（CCM 不会自动添加此规则）。

验证：
```shell
kubectl get service loadbalancer-service-demo-05
```

---

## 示例 6：共享型 ELB 保留客户端源 IP

**场景说明**：启用透明客户端 IP 传递，后端服务器能获取到客户端的真实 IP 地址。

**前置条件**：
- 已完成 CCM 安装（参阅[快速入门](./getting-started.md)）。
- 节点安全组规则和访问控制策略已正确配置，允许 ELB 使用真实 IP 与后端通信。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: shared
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.enable-transparent-client-ip: 'true'
  labels:
    app: nginx
  name: loadbalancer-service-demo-06
  namespace: default
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

> **注意**：
> - 启用此功能后，ELB 会使用后端服务器的真实 IP 进行通信。
> - 启用后，一台服务器不能同时作为后端服务器和客户端。
> - 启用后，后端服务器规格不能更改。

验证：
```shell
kubectl get service loadbalancer-service-demo-06
```

---

## 示例 7：创建独享型 ELB 服务

**场景说明**：创建独享型（Dedicated）ELB 实例，独享型 ELB 提供更高的性能和更多高级功能。

**前置条件**：
- 已完成 CCM 安装（参阅[快速入门](./getting-started.md)）。
- 已获取集群所在可用区（AZ）名称，例如 `cn-north-4a`。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>   # 替换为实际可用区，例如 cn-north-4a
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
  labels:
    app: nginx
  name: loadbalancer-service-demo-07
  namespace: default
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

> **参数说明**：
> - `kubernetes.io/elb.class: dedicated` — 使用独享型 ELB。
> - `kubernetes.io/elb.availability-zones` — **必填**，指定 ELB 所在可用区，多个可用区以分号 `;` 分隔。

验证：
```shell
kubectl get service loadbalancer-service-demo-07 -w
```

---

## 示例 8：独享型 ELB + ExternalTrafficPolicy=Local（Pod IP 后端）

**场景说明**：设置 `externalTrafficPolicy: Local`，独享型 ELB 直接使用 Pod IP 作为后端成员（而非 Node IP），完美保留客户端源 IP。

**前置条件**：
- 已完成 CCM 安装（参阅[快速入门](./getting-started.md)）。
- 已获取可用区名称和后端子网 ID。
- 要提前将 ELB 与 Pod 网络打通，否则 ELB 监听器健康检查不通过。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>           # 替换为实际可用区
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.backend.ip-version: ipv4               # 指定后端 IP 版本
    kubernetes.io/elb.backend.subnet-id: <your-subnet-id>    # 替换为后端子网 ID
  labels:
    app: nginx
  name: loadbalancer-service-demo-08
  namespace: default
spec:
  externalTrafficPolicy: Local
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

> **参数说明**：
> - `externalTrafficPolicy: Local` — 仅将运行有匹配 Pod 的节点作为后端，使用 Pod IP 直接通信。
> - `kubernetes.io/elb.backend.ip-version: ipv4` — 指定后端 IP 版本（可选 `ipv4`/`v4` 或 `ipv6`/`v6`）。
> - `kubernetes.io/elb.backend.subnet-id` — 指定后端成员使用的子网 ID，适用于节点有多个网络接口的场景。

验证：
```shell
kubectl get service loadbalancer-service-demo-08 -w
```

---

## 示例 9：独享型 ELB 跨 VPC 后端

**场景说明**：启用独享型 ELB 的跨 VPC 后端功能，允许 ELB 将流量转发到其他 VPC 的后端服务器。

**前置条件**：
- 已完成 CCM 安装（参阅[快速入门](./getting-started.md)）。
- 已获取可用区名称。
- 已在华为云控制台或通过 API 配置好跨 VPC 网络连通（对等连接等）。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>         # 替换为实际可用区
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.enable-cross-vpc: 'true'
  labels:
    app: nginx
  name: loadbalancer-service-demo-09
  namespace: default
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

> **注意**：
> - `kubernetes.io/elb.enable-cross-vpc` 仅对独享型 ELB 有效。
> - 该值只能从 `false` 更新为 `true`，不支持反向操作。

验证：
```shell
kubectl get service loadbalancer-service-demo-09 -w
```

---

## 示例 10：独享型 ELB 指定规格（Flavor）

**场景说明**：创建独享型 ELB 时指定四层/七层规格（Flavor），控制 ELB 的性能等级。

**前置条件**：
- 已完成 CCM 安装（参阅[快速入门](./getting-started.md)）。
- 已获取可用区名称。
- 已在华为云 ELB 控制台查询到可用的 Flavor ID。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>           # 替换为实际可用区
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.l4-flavor-id: <your-l4-flavor-id>     # 替换为四层规格 ID
    # kubernetes.io/elb.l7-flavor-id: <your-l7-flavor-id>   # 如需七层规格，取消注释并填写
  labels:
    app: nginx
  name: loadbalancer-service-demo-10
  namespace: default
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

> **参数说明**：
> - `kubernetes.io/elb.l4-flavor-id` — 四层（TCP/UDP）规格 ID。
> - `kubernetes.io/elb.l7-flavor-id` — 七层（HTTP/HTTPS）规格 ID。
> - 如果两个都未指定，将使用默认规格。

验证：
```shell
kubectl get service loadbalancer-service-demo-10 -w
```

---

## 示例 11：独享型 ELB TLS 终止（HTTPS）

**场景说明**：创建独享型 ELB 并配置 TLS 终止监听器，ELB 负责 SSL/TLS 解密，后端使用 HTTP。

**前置条件**：
- 已完成 CCM 安装（参阅[快速入门](./getting-started.md)）。
- 已获取可用区名称。
- 已在华为云 ELB 控制台创建服务器证书，并获取证书 ID。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>                    # 替换为实际可用区
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.default-tls-container-ref: <your-cert-id>       # 替换为服务器证书 ID
  labels:
    app: nginx
  name: loadbalancer-service-demo-11
  namespace: default
spec:
  ports:
    - port: 443
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

> **参数说明**：
> - `kubernetes.io/elb.default-tls-container-ref` — 指定服务器证书 ID，设置后 CCM 会创建 `TERMINATED_HTTPS` 类型的监听器。
> - ELB 在 443 端口接收 HTTPS 请求，解密后以 HTTP 转发到后端 Pod 的 80 端口。

验证（使用 HTTPS 访问）：
```shell
kubectl get service loadbalancer-service-demo-11

# 获取到 EXTERNAL-IP 后
curl -k https://<EXTERNAL-IP>
```

---

## 示例 12：HTTP/HTTPS 超时配置

**场景说明**：为 ELB 监听器配置空闲超时、请求超时和响应超时，适用于 HTTP/HTTPS 协议。

**前置条件**：已完成 CCM 安装（参阅[快速入门](./getting-started.md)）。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>           # 替换为实际可用区
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.idle-timeout: '60'
    kubernetes.io/elb.request-timeout: '60'
    kubernetes.io/elb.response-timeout: '60'
    kubernetes.io/elb.x-forwarded-host: 'true'
  labels:
    app: nginx
  name: loadbalancer-service-demo-12
  namespace: default
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

> **参数说明**：
> - `kubernetes.io/elb.idle-timeout` — 空闲超时，范围 0~4000 秒。
> - `kubernetes.io/elb.request-timeout` — 请求超时，范围 1~300 秒（仅 HTTP/HTTPS 有效）。
> - `kubernetes.io/elb.response-timeout` — 响应超时，范围 1~300 秒（仅 HTTP/HTTPS 有效）。
> - `kubernetes.io/elb.x-forwarded-host: 'true'` — 启用 `X-Forwarded-Host` 头重写。启用后 ELB 监听器协议会自动设为 HTTP（即使 Service spec 中指定了 `protocol: TCP`）。

验证：
```shell
kubectl get service loadbalancer-service-demo-12
```

---

## 示例 13：多端口 Service

**场景说明**：创建一个暴露多个端口的 LoadBalancer Service，ELB 会为每个端口创建对应的监听器。

**前置条件**：已完成 CCM 安装（参阅[快速入门](./getting-started.md)）。

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: shared
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
  labels:
    app: nginx
  name: loadbalancer-service-demo-13
  namespace: default
spec:
  ports:
    - name: http
      port: 80
      protocol: TCP
      targetPort: 80
    - name: tcp-443
      port: 443
      protocol: TCP
      targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
EOF
```

验证：
```shell
kubectl get service loadbalancer-service-demo-13
```

预期输出：
```
NAME                           TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)                      AGE
loadbalancer-service-demo-13   LoadBalancer   10.1.130.100   192.168.0.200   80:30993/TCP,443:31443/TCP   2m
```

测试两个端口均可访问：
```shell
curl http://192.168.0.200:80
curl http://192.168.0.200:443
```

---

## 清理资源

按需删除示例中创建的资源：

```shell
# 删除所有示例 Service
kubectl delete service loadbalancer-service-demo-01 loadbalancer-service-demo-02 \
  loadbalancer-service-demo-03 loadbalancer-service-demo-04 loadbalancer-service-demo-05 \
  loadbalancer-service-demo-06 loadbalancer-service-demo-07 loadbalancer-service-demo-08 \
  loadbalancer-service-demo-09 loadbalancer-service-demo-10 loadbalancer-service-demo-11 \
  loadbalancer-service-demo-12 loadbalancer-service-demo-13

# 删除测试 Deployment
kubectl delete deployment deployment-ccm-test

# （可选）卸载 CCM
kubectl delete -f https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/master/manifests/huawei-cloud-controller-manager-deployment.yaml
kubectl delete -f https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/master/manifests/rbac-huawei-cloud-controller-manager.yaml
kubectl delete secret cloud-config -n kube-system
```

> **注意**：删除 Service 后，CCM 会自动删除自动创建的 ELB 实例和 EIP（除非设置了 `keep-eip: true`）。通过 `kubernetes.io/elb.id` 引用的已有 ELB 实例不会被删除。
>
> 卸载 CCM 时，请确保 URL 中的分支（上例中的 `master`）与安装时使用的分支或 release tag 一致。

---

## 全局负载均衡配置（ConfigMap）

除了在 Service Annotation 中逐个配置参数外，还可以通过 `loadbalancer-config` ConfigMap 设置全局默认值。

有关支持的完整参数列表和 ConfigMap 示例，请参阅 CCM 配置说明中的[负载均衡器配置](./huawei-cloud-controller-manager-configuration.md#负载均衡器配置)部分。

> **说明**：
> - ConfigMap 中的配置为全局默认值。如果 Service Annotation 中设置了相同参数，则以 Annotation 为准。
> - 修改 ConfigMap 后需要重启 CCM Pod 才能生效：
>   ```shell
>   kubectl rollout restart deployment huawei-cloud-controller-manager -n kube-system
>   ```
