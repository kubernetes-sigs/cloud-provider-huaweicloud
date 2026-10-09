# 华为云 Cloud Controller Manager 配置说明

> [English](../huawei-cloud-controller-manager-configuration.md) | [中文](./huawei-cloud-controller-manager-configuration.md)

有两套配置，如下：

* `华为云配置` — 华为云的配置信息。

* `负载均衡器配置` — ELB 服务的全局配置。

## 华为云配置

配置存储在 `cloud-config`（命名空间：`kube-system`）Secret 中。
有关在 Kubernetes 集群中创建 Secret 的步骤，请参阅[快速入门](./getting-started.md)。

cloud-config 结构如下：

```yaml
[Global]
region=
access-key=
secret-key=
project-id=
cloud=
auth-url=

[Vpc]
id=
subnet-id=
security-group-id=
```

> 修改后需要重启 CCM 才能加载新数据。

支持的参数如下：

### Global

此部分提供华为云 IAM 配置和认证信息。

* `region` 必填。华为云区域。

  **注意**：`region` 必须与 Kubernetes 集群的 ECS 所在区域一致。

* `access-key` 必填。华为云的 Access Key。

* `secret-key` 必填。华为云的 Secret Key。

* `project-id` 可选。华为云的项目 ID。
  请参阅[获取项目 ID](https://support.huaweicloud.com/intl/en-us/api-evs/evs_04_0046.html)。
  
  **注意**：`project-id` 必须与 Kubernetes 集群的 ECS 所在项目一致。

* `cloud` 可选。云提供商的 endpoint。默认为 `myhuaweicloud.com`。

* `auth-url` 可选。身份认证 URL。默认为 `https://iam.{cloud}:443/v3/`。

* `insecure` 可选。信任自签名 SSL 证书。需设置为字面字符串 `"true"` 才能启用（例如 `insecure=true`），其他值均视为禁用。

* `instance-version` 可选。选择 CCM 实例管理 API：`v1` 使用旧版 Instances 接口，`v2` 使用优化后的 InstancesV2 接口。
  默认为 `v2`。

### Vpc

此部分包含网络配置信息。

* `id` 可选。指定 Kubernetes 集群的 ECS 所使用的 VPC。

* `subnet-id` 可选。指定 Kubernetes 集群的 ECS 所使用的 IPv4 子网 ID。

* `security-group-id` 可选。用于关联节点的安全组 ID。
  当新节点加入集群时，会自动关联该安全组。
  当节点移除时，会自动解除关联。

## 负载均衡器配置

这些参数在 Service 的 annotation 为空时生效。
需要存储在 `kube-system` 命名空间的 `loadbalancer-config` ConfigMap 中。

> 推荐使用 `kube-system` 命名空间。为向后兼容，CCM 会先检查 `huawei-cloud-provider` 命名空间，
> 然后回退到 `kube-system`。建议迁移到 `kube-system`。

示例：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: kube-system
  name: loadbalancer-config
data:
  loadBalancerOption: |-
    {
       "lb-algorithm": "ROUND_ROBIN",
       "keep-eip": false,
       "session-affinity-flag": "on",
       "session-affinity-option": {
         "type": "SOURCE_IP",
         "persistence_timeout": 15
       },
       "disable-create-security-group": false,
       "health-check-flag": "on",
        "health-check-option": {
          "delay": 5,
          "timeout": 3,
          "max_retries": 3
        }
    }
```

> 修改后需要重启 CCM 才能加载新数据。

支持的参数如下：

### 负载均衡器选项

* `lb-algorithm` 指定后端服务器组的负载均衡算法。

  根据后端服务器组的协议，取值范围不同：

  **ROUND_ROBIN**：加权轮询算法。

  **LEAST_CONNECTIONS**：加权最少连接算法。

  **SOURCE_IP**：源 IP 哈希算法。

  当值为 **SOURCE_IP** 时，后端服务器组中后端服务器的权重无效。

* `session-affinity-flag` 指定是否开启会话保持。
  取值为 `on` 和 `off`，默认为 `off`。

* `session-affinity-option` 指定会话保持超时时间，单位为分钟。

  当 `session-affinity-flag` 为 `on` 时，此参数必填。

  这是一个 JSON 字符串，例如 `{"type": "SOURCE_IP", "persistence_timeout": 15}`。

  详细说明：
  
  * `type` 必填。指定会话保持类型。
  
    根据后端服务器组的协议，取值范围不同：
  
    **SOURCE_IP**：根据客户端 IP 地址分发请求。
  
    来自同一 IP 地址的请求会被发送到同一后端服务器。
  
    **HTTP_COOKIE**：客户端首次发送请求时，负载均衡器自动生成 cookie 并插入响应消息中。
  
    后续请求将发送到处理第一个请求的后端服务器。
  
    **APP_COOKIE**：客户端首次发送请求时，接收请求的后端服务器生成 cookie 并插入响应消息中。
  
    后续请求将发送到该后端服务器。
  
    当后端服务器组的协议为 `TCP` 时，仅 **SOURCE_IP** 生效。
  
    当后端服务器组的协议为 `HTTP` 时，仅 **HTTP_COOKIE** 或 **APP_COOKIE** 生效。

  * `cookie_name` 可选。指定 cookie 名称。
  
    当会话保持类型为 **APP_COOKIE** 时，此参数必填。
  
  * `persistence_timeout` 可选。指定会话保持超时时间，单位为分钟。
  
    当 `type` 设置为 **APP_COOKIE** 时，此参数无效。
  
    根据后端服务器组的协议，取值范围不同：
  
    当后端服务器组的协议为 `TCP` 或 `UDP` 时，取值范围为 `1` ~ `60`。
  
    当后端服务器组的协议为 `HTTP` 或 `HTTPS` 时，取值范围为 `1` ~ `1440`。

  详见[添加后端服务器组](https://support.huaweicloud.com/intl/en-us/api-elb/elb_qy_hz_0001.html)。

* `keep-eip` 指定删除 ELB 服务时是否保留 EIP。
  取值为 `true` 和 `false`，默认为 `false`。

* `health-check-flag` 指定是否为后端服务器组开启健康检查。
  取值为 `on` 和 `off`，默认为 `on`。

  > 开启健康检查时，请确保节点安全组已放行来自 `100.125.0.0/16` 的入站流量。
  > CCM 不会自动添加此规则。`100.125.0.0/16` 是 ELB 用于检查后端服务器健康状态的内部 IP 地址。

* `health-check-option` 指定健康检查参数。

  当 `health-check` 为 `on` 时，此参数必填。

  这是一个 JSON 字符串，默认为 `{"delay": 5, "timeout": 3, "max_retries": 3}`。

  详细说明：

  * `delay` 必填。指定健康检查的最大间隔时间，单位为秒。
    取值范围为 `1` ~ `50`。默认为 `5`。

  * `max_retries` 必填。指定最大重试次数。
    取值范围为 `1` ~ `10`。默认为 `3`。

  * `timeout` 必填。指定健康检查超时时间，单位为秒。
    取值范围为 `1` ~ `50`。默认为 `3`。

  * `protocol` 可选。指定健康检查协议。保留字段；协议由 Service 端口协议自动推导。

  * `path` 可选。指定 HTTP/HTTPS 健康检查路径。保留字段；CCM 当前未使用此字段。

* `enable-transparent-client-ip` 指定是否将客户端的源 IP 地址传递给后端服务器。
  取值为 `'true'` 和 `'false'`。

  共享型负载均衡器的 TCP 或 UDP 监听器：
  取值为 **true** 或 **false**，未传递此注解时默认为 **false**。

  共享型负载均衡器的 HTTP 或 HTTPS 监听器：
  取值只能为 **true**，未传递此注解时默认为 **true**。

  独享型负载均衡器的所有监听器：
  取值只能为 **true**，未传递此注解时默认为 **true**。

  > 注意：
  >
  > 启用此功能后，负载均衡器会使用后端服务器的真实 IP 进行通信。
  > 请确保安全组规则和访问控制策略已正确配置。
  >
  > 启用后，一台服务器不能同时作为后端服务器和客户端。
  >
  > 启用后，后端服务器规格不能更改。

* `enable-cross-vpc` 可选。指定是否启用跨 VPC 后端。
  取值为 `true`（启用）或 `false`（禁用）。
  该值只能更新为 `true`。
  仅独享型负载均衡器服务使用此注解。

* `l4-flavor-id` 可选。指定四层规格 ID。
  仅独享型负载均衡器服务使用此注解。

* `l7-flavor-id` 可选。指定七层规格 ID。
  仅独享型负载均衡器服务使用此注解。

* `disable-create-security-group` 可选。CCM 当前未实现此功能。ELB 健康检查的安全组规则（`100.125.0.0/16`）需手动配置。

* `business-name` 可选。业务名称或业务标识，用于组合华为云 ELB 实例名称。
  为防止在多个 K8s 集群中使用同一租户账号时创建同名 ELB 实例，
  最终导致 CCM 故障。
  例如，`business-name: order-pass`，Kubernetes 负载均衡服务命名空间/名称为 `default/order-service`，
  则 ELB 实例名称为：`k8s_service_order-pass_default_order-service`。

  > 注意：
  > 更改此参数会创建新的 ELB 实例，旧的 ELB 实例不会被删除且不再维护。

* `loadbalancer-class` 可选。设置后，只有 `spec.loadBalancerClass` 等于 `huaweicloud.com/elb` 的 Service 才会被 CCM 处理。
  适用于多集群场景中多个 CCM 实例共享同一集群时。
  如不设置，CCM 将处理所有 `LoadBalancer` 类型的 Service。

* `primary-nic` 可选。设置为 `force` 时，CCM 将使用节点主网卡的 IP 作为 ELB 后端。
  如不设置（默认），CCM 使用 Pod 的 HostIP 作为后端。
