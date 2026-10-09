# 使用指南

> [English](../usage-guide.md) | [中文](./usage-guide.md)

本页面提供 Service Annotations 的参数说明。

在运行示例之前，请确保已在 Kubernetes 集群中安装了 `huawei-cloud-controller-manager`，
参阅[快速入门](./getting-started.md)。

如果 Service 中的 annotation 为空，
将使用[负载均衡器配置](./huawei-cloud-controller-manager-configuration.md#负载均衡器配置)中的值，
否则使用设置的值。

## Service Annotations

* `kubernetes.io/elb.class` 必填。指定使用的 ELB 服务类型。取值为：

  **shared**：使用共享型负载均衡服务。

  **dedicated**：使用独享型负载均衡服务。

* `kubernetes.io/elb.availability-zones` 可选。指定创建负载均衡器的可用区，多个可用区以分号（;）分隔。
  此注解适用于独享型负载均衡器（`kubernetes.io/elb.class: dedicated`），
  创建独享型负载均衡器服务时为必填。

* `kubernetes.io/elb.id` 可选。指定使用已有的 ELB 服务。
  如果为空，将自动创建新的 ELB 服务。

* `kubernetes.io/elb.subnet-id` 可选。指定负载均衡器所在的 IPv4 子网 ID。
  如果为空，将使用 `cloud-config` Secret 中的 `subnet-id`。
  如果两者都为空，则查询节点所在的子网。
  仅支持 IPv4 子网。

* `kubernetes.io/elb.eip-id` 可选。指定 ELB 服务使用的 EIP。
  使用已有 ELB 服务时此字段无效。

* `kubernetes.io/elb.keep-eip` 可选。指定删除 ELB 服务时是否保留 EIP。
  取值为 `'true'` 和 `'false'`，默认为 `'false'`。

* `kubernetes.io/elb.eip-auto-create-option` 可选。指定是否为 ELB 服务自动创建 EIP。
  这是一个 JSON 字符串，例如 `{"ip_type": "5_bgp", "bandwidth_size": 5, "share_type": "PER"}`。

  详细说明：

  * `share_type` 必填。指定带宽类型。取值为：

    **PER**：独享带宽。
    **WHOLE**：共享带宽。

    如果设置为 **WHOLE**，则必须指定 `share_id`。

  * `ip_type` 可选（当 `share_type` 为 `PER` 时必填）。指定 EIP 类型。取值为 `5_bgp`（动态 BGP）或 `5_sbgp`（静态 BGP）。

    各区域支持的 `ip_type`，请
    参阅[分配 EIP](https://support.huaweicloud.com/intl/en-us/api-eip/eip_api_0001.html) "表4 publicIP字段说明"。

  * `bandwidth_size` 可选（当 `share_type` 为 `PER` 时必填）。指定带宽大小。

  * `charge_mode` 可选（当 `share_type` 为 `PER` 时必填）。指定带宽按流量还是按带宽大小计费。
    默认为 `traffic`，取值为：

    **bandwidth**：按带宽大小计费。

    **traffic**：按流量计费。

  * `share_id` 可选。指定带宽 ID。分配 EIP 时可以指定已有的共享带宽。

    当 `share_type` 为 `WHOLE` 时必填。

* `kubernetes.io/elb.lb-algorithm` 可选。指定后端服务器组的负载均衡算法。
  根据后端服务器组的协议，取值范围不同：

  **ROUND_ROBIN**：加权轮询算法。

  **LEAST_CONNECTIONS**：加权最少连接算法。

  **SOURCE_IP**：源 IP 哈希算法。
  当值为 **SOURCE_IP** 时，后端服务器组中后端服务器的权重无效。

* `kubernetes.io/elb.session-affinity-flag` 可选。指定是否开启会话保持。
  取值为 `'on'` 和 `'off'`，默认为 `'off'`。

* `kubernetes.io/elb.session-affinity-option` 指定会话保持超时时间，单位为分钟。
  当 `kubernetes.io/elb.session-affinity-flag` 为 `'on'` 或全局 `session-affinity-flag` 为 `on` 时，此参数必填。
  这是一个 JSON 字符串，例如 `{"type": "SOURCE_IP", "persistence_timeout": 15}`。
  详细说明：

  * `type` 必填。指定会话保持类型。
    根据后端服务器组的协议，取值范围不同：

    **SOURCE_IP**：根据客户端 IP 地址分发请求。
    来自同一 IP 地址的请求会被发送到同一后端服务器。

    **HTTP_COOKIE**：客户端首次发送请求时，负载均衡器自动生成 cookie 并插入响应消息中。后续请求将发送到处理第一个请求的后端服务器。

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

* `kubernetes.io/elb.health-check-flag` 可选。指定是否为后端服务器组开启健康检查。
  取值为 `on` 和 `off`，默认为 `on`。

  > 开启健康检查时，请确保节点安全组已放行来自 `100.125.0.0/16` 的入站流量。
  > CCM 不会自动添加此规则。`100.125.0.0/16` 是 ELB 用于检查后端服务器健康状态的内部 IP 地址。

* `kubernetes.io/elb.health-check-option` 可选。指定健康检查参数。
  当 `health-check` 为 `on` 时，此参数必填。
  这是一个 JSON 字符串，例如 `{"delay": 5, "timeout": 3, "max_retries": 3}`。
  详细说明：

  * `delay` 必填。指定健康检查的最大间隔时间，单位为秒。
    取值范围为 `1` ~ `50`。默认为 `5`。

  * `max_retries` 必填。指定最大重试次数。
    取值范围为 `1` ~ `10`。默认为 `3`。

  * `timeout` 必填。指定健康检查超时时间，单位为秒。
    取值范围为 `1` ~ `50`。默认为 `3`。

* `kubernetes.io/elb.enable-transparent-client-ip` 可选。指定是否将客户端的源 IP 地址传递给后端服务器。
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

* `kubernetes.io/elb.x-forwarded-host` 可选。指定是否重写 `X-Forwarded-Host` 头。
  启用后，`X-Forwarded-Host` 会根据请求中的 Host 重写并发送到后端服务器。

  取值为 `'true'` 和 `'false'`，默认为 `'false'`。

* `kubernetes.io/elb.default-tls-container-ref` 可选。指定监听器使用的服务器证书 ID。
  设置此选项后，云提供商将创建 `TERMINATED_HTTPS` 类型的监听器，用于 TLS 终止负载均衡。

* `kubernetes.io/elb.idle-timeout` 可选。指定监听器的空闲超时时间。取值范围：`0` ~ `4000`。
  单位：秒。

* `kubernetes.io/elb.request-timeout` 可选。指定监听器的请求超时时间。取值范围：`1` ~ `300`。
  单位：秒。此参数在协议设置为 *HTTP* 或 *HTTPS* 时有效。

* `kubernetes.io/elb.response-timeout` 可选。指定监听器的响应超时时间。取值范围：`1` ~ `300`。
  单位：秒。此参数在协议设置为 *HTTP* 或 *HTTPS* 时有效。

* `kubernetes.io/elb.enable-cross-vpc` 可选。指定是否启用跨 VPC 后端。
  取值为 `true`（启用）或 `false`（禁用）。
  该值只能更新为 `true`。
  仅独享型负载均衡器服务（`kubernetes.io/elb.class: dedicated`）使用此注解。

* `kubernetes.io/elb.l4-flavor-id` 可选。指定四层规格 ID。
  如果 `kubernetes.io/elb.l4-flavor-id` 和 `kubernetes.io/elb.l7-flavor-id` 均未指定，
  则使用默认规格。
  仅独享型负载均衡器服务（`kubernetes.io/elb.class: dedicated`）使用此注解。

* `kubernetes.io/elb.l7-flavor-id` 可选。指定七层规格 ID。
  如果 `kubernetes.io/elb.l4-flavor-id` 和 `kubernetes.io/elb.l7-flavor-id` 均未指定，
  则使用默认规格。
  仅独享型负载均衡器服务（`kubernetes.io/elb.class: dedicated`）使用此注解。

* `kubernetes.io/elb.backend.subnet-id` 可选。指定后端服务器的子网 ID。
  设置此注解后，后端成员将使用指定子网添加到池中。
  当节点有多个网络接口且你想为后端成员指定特定子网时非常有用。
  仅独享型负载均衡器服务（`kubernetes.io/elb.class: dedicated`）使用此注解。

* `kubernetes.io/elb.backend.ip-version` 可选。指定使用 Pod IP 作为后端时
  （如 `ExternalTrafficPolicy=Local` 或 `AllocateLoadBalancerNodePorts=false`）的后端服务器 IP 版本。
  取值为 `ipv4`（或 `v4`）和 `ipv6`（或 `v6`）。
  如果未设置此注解，则使用默认 Pod IP。
  仅独享型负载均衡器服务（`kubernetes.io/elb.class: dedicated`）使用此注解。

## 创建 LoadBalancer 类型的 Service

如需上述所有注解的完整可执行示例及更多场景
（自动创建共享型/独享型 ELB、EIP 绑定、会话保持、健康检查、TLS 终止、跨 VPC 后端、多端口服务等），
请参阅[使用示例](./usage-examples.md)。
