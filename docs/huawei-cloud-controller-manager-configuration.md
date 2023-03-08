# Huawei Cloud Controller Manager Configurations

There are 2 sets of configurations, as follows:

* `Huawei Cloud Configuration` - the configuration information of Huawei Cloud.

* `Loadbalancer Configuration` - global configuration of the ELB service.

## Huawei Cloud Configuration

The configuration is stored in `cloud-config`(namespace: `kube-system`) secret.
See [create the cloud-config secret](./create-cloud-config-secret.md) to creating secret in Kubernetes cluster.

The cloud-config structure is as follows:

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
```

The following arguments are supported:

### Global

This section provides Huawei Cloud IAM configuration and authentication information.

* `region` Required. This is the Huawei Cloud region.

  **Note**: The `region` must be the same as the ECSes of the Kubernetes cluster.

* `access-key` Required. The access key of the Huawei Cloud.

* `secret-key` Required. The secret key of the Huawei Cloud.

* `project-id` Optional. The Project ID of the Huawei Cloud. 
  See [Obtaining a Project ID](https://support.huaweicloud.com/intl/en-us/api-evs/evs_04_0046.html).
  
  **Note**: The `project-id` must be the same as the ECSes of the Kubernetes cluster.

* `cloud` Optional. The endpoint of the cloud provider. Defaults to `myhuaweicloud.com`'`.

* `auth-url` Optional. The Identity authentication URL. Defaults to `https://iam.{cloud}:443/v3/`.

### Vpc

This section contains network configuration information.

* `id` Optional. Specifies the VPC used by ECSes of the Kubernetes cluster.

* `subnet-id` Optional. Specifies the IPv4 subnet ID used by ECSes of the Kubernetes cluster.

## Loadbalancer Configuration

These arguments will be applied when the annotation in the service is empty.
It needs to be stored in the `loadbalancer-config` ConfigMap under the `huawei-cloud-provider` namespace.

Here's an example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: huawei-cloud-provider
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
       "health-check-flag": "on",
       "health-check-option": {
         "delay": 5,
         "timeout": 15,
         "max_retries": 5
       }
    }
```

The following arguments are supported:

### Load Balancer Options

* `lb-algorithm` Specifies the load balancing algorithm of the backend server group.

  The value range varies depending on the protocol of the backend server group:

  **ROUND_ROBIN**: indicates the weighted round-robin algorithm.

  **LEAST_CONNECTIONS**: indicates the weighted least connections algorithm.

  **SOURCE_IP**: indicates the source IP hash algorithm.

  When the value is **SOURCE_IP**, the weights of backend servers in the server group are invalid.

* `session-affinity-flag` Specifies whether to enable session affinity.
  Valid values are `on` and `off`, defaults to `off`.

* `session-affinity-option` Specifies the sticky session timeout duration in minutes. 

  This parameter is mandatory when the `session-affinity-flag` is `on`.

  This is a json string, such as `{"type": "SOURCE_IP", "persistence_timeout": 15}`.

  For details:
  
  * `type` Required. Specifies the sticky session type.
  
    The value range varies depending on the protocol of the backend server group:
  
    **SOURCE_IP**: Requests are distributed based on the client's IP address. 
  
    Requests from the same IP address are sent to the same backend server.
  
    **HTTP_COOKIE**: When the client sends a request for the first time, the load balancer automatically generates 
    a cookie and inserts the cookie into the response message. 
  
    Subsequent requests are sent to the backend server that processes the first request.
  
    **APP_COOKIE**: When the client sends a request for the first time, the backend server that receives the request
    generates a cookie and inserts the cookie into the response message. 
  
    Subsequent requests are sent to this backend server.
  
    When the protocol of the backend server group is `TCP`, only **SOURCE_IP** takes effect.
  
    When the protocol of the backend server group is `HTTP`, only **HTTP_COOKIE** or **APP_COOKIE** takes effect.

  * `cookie_name` Optional. Specifies the cookie name.
  
    This parameter is mandatory when the sticky session type is **APP_COOKIE**.
  
  * `persistence_timeout` Optional. Specifies the sticky session timeout duration in minutes.
  
    This parameter is invalid when `type` is set to **APP_COOKIE**.
  
    The value range varies depending on the protocol of the backend server group:
  
    When the protocol of the backend server group is `TCP` or `UDP`, the value ranges from `1` to `60`.
  
    When the protocol of the backend server group is `HTTP` or `HTTPS`, the value ranges from `1` to `1440`.

  See [Adding a Backend Server Group](https://support.huaweicloud.com/intl/en-us/api-elb/elb_qy_hz_0001.html) for detailed usage.

* `keep-eip` Specifies whether to retain the EIP when deleting a ELB service.
  Valid values are `true` and `false`, defaults to `false`.

* `health-check-flag` Specifies whether to enable health check for a backend server group.
  Valid values are `on` and `off`, defaults to `on`.

* `health-check-option` Specifies the health check.

  This parameter is mandatory when the `health-check` is `on`.

  This is a json string, defaults to `{"delay": 5, "timeout": 3, "max_retries": 3}`.

  For details:

  * `delay` Required. Specifies the maximum time between health checks in the unit of second.
    The value ranges from `1` to `50`. Defaults to `5`.

  * `max_retries` Required. Specifies the maximum number of retries.
    The value ranges from `1` to `10`. Defaults to `3`.

  * `timeout` Required. Specifies the health check timeout duration in the unit of second.
    The value ranges from `1` to `50`. Defaults to `3`.

* `enable-cross-vpc` Optional. Specifies whether to enable cross-VPC backend.
  The value can be `true` (enable cross-VPC backend) or `false` (disable cross-VPC backend).
  The value can only be updated to `true`.
  Only dedicated load balancer service will use this annotation.

* `l4-flavor-id` Optional. Specifies the ID of a flavor at Layer 4.
  Only dedicated load balancer service will use this annotation.

* `l7-flavor-id` Optional. Specifies the ID of a flavor at Layer 7.
  Only dedicated load balancer service will use this annotation.
