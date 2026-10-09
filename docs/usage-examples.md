# Huawei Cloud Kubernetes Cloud Provider Examples

> [English](./usage-examples.md) | [中文](./zh/usage-examples.md)

This document provides complete executable examples for Huawei Cloud Cloud Controller Manager (CCM), covering various LoadBalancer service creation scenarios.

> For installation instructions, see [Getting Started](./getting-started.md).

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Common Preparation: Create a Test Deployment](#common-preparation-create-a-test-deployment)
- [Example 1: Auto-create a Shared ELB Service](#example-1-auto-create-a-shared-elb-service)
- [Example 2: Use an Existing Shared ELB Instance](#example-2-use-an-existing-shared-elb-instance)
- [Example 3: Auto-create an EIP and Bind It to the ELB](#example-3-auto-create-an-eip-and-bind-it-to-the-elb)
- [Example 4: Shared ELB Session Affinity](#example-4-shared-elb-session-affinity)
- [Example 5: Shared ELB Custom Health Check](#example-5-shared-elb-custom-health-check)
- [Example 6: Shared ELB Preserve Client Source IP](#example-6-shared-elb-preserve-client-source-ip)
- [Example 7: Create a Dedicated ELB Service](#example-7-create-a-dedicated-elb-service)
- [Example 8: Dedicated ELB + ExternalTrafficPolicy=Local (Pod IP Backend)](#example-8-dedicated-elb--externaltrafficpolicylocal-pod-ip-backend)
- [Example 9: Dedicated ELB Cross-VPC Backend](#example-9-dedicated-elb-cross-vpc-backend)
- [Example 10: Dedicated ELB with Specified Flavor](#example-10-dedicated-elb-with-specified-flavor)
- [Example 11: Dedicated ELB TLS Termination (HTTPS)](#example-11-dedicated-elb-tls-termination-https)
- [Example 12: HTTP/HTTPS Timeout Configuration](#example-12-httphttps-timeout-configuration)
- [Example 13: Multi-Port Service](#example-13-multi-port-service)
- [Clean Up Resources](#clean-up-resources)
- [Global Load Balancer Configuration (ConfigMap)](#global-load-balancer-configuration-configmap)

---

## Prerequisites

Before running the examples below, ensure that the Huawei Cloud Controller Manager has been installed in your cluster.
See the [Getting Started Guide](./getting-started.md) for installation instructions.

Additionally, ensure the following:

1. **kubectl Tool**: Installed and able to connect to the cluster.
2. **IAM Permissions**: The AK/SK used by CCM must have permissions for ELB, ECS, VPC, EVS, EIP, IMS, etc. See the [IAM Policy documentation](./iam-policy.md).
3. **Security Group**: The cluster node security group must allow the ELB health check CIDR `100.125.0.0/16`.

> **Note**: All `kubectl` commands below require a properly configured kubeconfig with normal access to the cluster.

---

## Common Preparation: Create a Test Deployment

Examples 1–13 all depend on an Nginx Deployment. Create it first:

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

Verify the Deployment:
```shell
kubectl get deployment deployment-ccm-test
```

---

## Example 1: Auto-create a Shared ELB Service

**Scenario**: Create a `Type: LoadBalancer` Service. CCM will automatically create a shared ELB instance and forward traffic to the backend Pods.

**Prerequisites**: CCM has been installed (see [Getting Started](./getting-started.md)).

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

Verify the service status (wait until `EXTERNAL-IP` is no longer `<pending>`):
```shell
kubectl get service loadbalancer-service-demo-01 -w
```

Expected output:
```
NAME                          TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)        AGE
loadbalancer-service-demo-01  LoadBalancer   10.1.130.216   192.168.0.113   80:30993/TCP   3m10s
```

Test access:
```shell
curl http://192.168.0.113
```

---

## Example 2: Use an Existing Shared ELB Instance

**Scenario**: Reuse an existing shared ELB instance on Huawei Cloud instead of automatically creating a new one.

**Prerequisites**:
- A shared ELB instance has already been created in the Huawei Cloud ELB console.
- The ID of that ELB instance has been obtained.

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: shared
    kubernetes.io/elb.id: <your-existing-elb-id>   # Replace with the existing ELB instance ID
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

Verify:
```shell
kubectl get service loadbalancer-service-demo-02
```

---

## Example 3: Auto-create an EIP and Bind It to the ELB

**Scenario**: When creating a shared ELB, automatically create a BGP-type EIP (dedicated bandwidth 5 Mbit/s) and bind it to the ELB.

**Prerequisites**: The account has available EIP quota.

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

> **Parameter Description**:
> - `ip_type: 5_bgp` — Dynamic BGP type EIP.
> - `bandwidth_size: 5` — Bandwidth size 5 Mbit/s.
> - `share_type: PER` — Dedicated bandwidth. To use shared bandwidth, set this to `WHOLE` and additionally specify `share_id`.

Verify:
```shell
kubectl get service loadbalancer-service-demo-03
```

Expected output (`EXTERNAL-IP` is the public EIP address):
```
NAME                           TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)        AGE
loadbalancer-service-demo-03   LoadBalancer   10.1.35.151   159.138.37.76   80:30080/TCP   41s
```

Test public access:
```shell
curl http://159.138.37.76
```

---

## Example 4: Shared ELB Session Affinity

**Scenario**: Enable source IP-based session affinity for a shared ELB. Requests from the same client IP will be forwarded to the same backend server.

**Prerequisites**: CCM has been installed (see [Getting Started](./getting-started.md)).

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

> **Parameter Description**:
> - `type: SOURCE_IP` — Session affinity based on client IP (TCP protocol only supports this type).
> - `persistence_timeout: 15` — Session affinity timeout 15 minutes (TCP/UDP range 1–60, HTTP/HTTPS range 1–1440).

Verify:
```shell
kubectl get service loadbalancer-service-demo-04
```

---

## Example 5: Shared ELB Custom Health Check

**Scenario**: Customize the ELB health check parameters for backend servers, adjusting the check interval, timeout, and retry count.

**Prerequisites**: CCM has been installed (see [Getting Started](./getting-started.md)).

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

> **Parameter Description**:
> - `delay: 5` — Health check interval 5 seconds (range 1–50).
> - `timeout: 10` — Health check timeout 10 seconds (range 1–50).
> - `max_retries: 3` — Maximum retry count 3 (range 1–10).
> - Ensure the node security group allows the `100.125.0.0/16` CIDR for health checks (CCM does not add this rule automatically).

Verify:
```shell
kubectl get service loadbalancer-service-demo-05
```

---

## Example 6: Shared ELB Preserve Client Source IP

**Scenario**: Enable transparent client IP pass-through so that backend servers can obtain the client's real IP address.

**Prerequisites**:
- CCM has been installed (see [Getting Started](./getting-started.md)).
- Node security group rules and access control policies are correctly configured to allow the ELB to communicate with backends using the real IP.

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

> **Note**:
> - When this feature is enabled, the ELB will communicate with backend servers using their real IP addresses.
> - Once enabled, a server cannot serve as both a backend server and a client simultaneously.
> - Once enabled, the backend server specifications cannot be changed.

Verify:
```shell
kubectl get service loadbalancer-service-demo-06
```

---

## Example 7: Create a Dedicated ELB Service

**Scenario**: Create a dedicated ELB instance. Dedicated ELBs provide higher performance and more advanced features.

**Prerequisites**:
- CCM has been installed (see [Getting Started](./getting-started.md)).
- The availability zone (AZ) name where the cluster is located has been obtained, e.g., `cn-north-4a`.

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>   # Replace with the actual AZ, e.g., cn-north-4a
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

> **Parameter Description**:
> - `kubernetes.io/elb.class: dedicated` — Use a dedicated ELB.
> - `kubernetes.io/elb.availability-zones` — **Required**. Specifies the AZ for the ELB. Multiple AZs are separated by semicolons `;`.

Verify:
```shell
kubectl get service loadbalancer-service-demo-07 -w
```

---

## Example 8: Dedicated ELB + ExternalTrafficPolicy=Local (Pod IP Backend)

**Scenario**: Set `externalTrafficPolicy: Local` so that the dedicated ELB uses Pod IPs directly as backend members (instead of Node IPs), perfectly preserving the client source IP.

**Prerequisites**:
- CCM has been installed (see [Getting Started](./getting-started.md)).
- The AZ name and backend subnet ID have been obtained.
- The ELB and Pod network must be connected beforehand; otherwise, the ELB listener health check will fail.

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>           # Replace with the actual AZ
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.backend.ip-version: ipv4               # Specify the backend IP version
    kubernetes.io/elb.backend.subnet-id: <your-subnet-id>    # Replace with the backend subnet ID
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

> **Parameter Description**:
> - `externalTrafficPolicy: Local` — Only nodes running matching Pods are used as backends, communicating directly via Pod IP.
> - `kubernetes.io/elb.backend.ip-version: ipv4` — Specifies the backend IP version (options: `ipv4`/`v4` or `ipv6`/`v6`).
> - `kubernetes.io/elb.backend.subnet-id` — Specifies the subnet ID used by backend members, applicable when nodes have multiple network interfaces.

Verify:
```shell
kubectl get service loadbalancer-service-demo-08 -w
```

---

## Example 9: Dedicated ELB Cross-VPC Backend

**Scenario**: Enable the cross-VPC backend feature for a dedicated ELB, allowing the ELB to forward traffic to backend servers in other VPCs.

**Prerequisites**:
- CCM has been installed (see [Getting Started](./getting-started.md)).
- The AZ name has been obtained.
- Cross-VPC network connectivity (peering, etc.) has been configured in the Huawei Cloud console or via API.

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>         # Replace with the actual AZ
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

> **Note**:
> - `kubernetes.io/elb.enable-cross-vpc` is only valid for dedicated ELBs.
> - This value can only be updated from `false` to `true`; the reverse is not supported.

Verify:
```shell
kubectl get service loadbalancer-service-demo-09 -w
```

---

## Example 10: Dedicated ELB with Specified Flavor

**Scenario**: Specify a Layer 4/Layer 7 flavor when creating a dedicated ELB to control the ELB's performance tier.

**Prerequisites**:
- CCM has been installed (see [Getting Started](./getting-started.md)).
- The AZ name has been obtained.
- Available Flavor IDs have been queried in the Huawei Cloud ELB console.

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>           # Replace with the actual AZ
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.l4-flavor-id: <your-l4-flavor-id>     # Replace with the Layer 4 flavor ID
    # kubernetes.io/elb.l7-flavor-id: <your-l7-flavor-id>   # Uncomment and fill in for Layer 7 flavor
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

> **Parameter Description**:
> - `kubernetes.io/elb.l4-flavor-id` — Layer 4 (TCP/UDP) flavor ID.
> - `kubernetes.io/elb.l7-flavor-id` — Layer 7 (HTTP/HTTPS) flavor ID.
> - If neither is specified, the default flavor will be used.

Verify:
```shell
kubectl get service loadbalancer-service-demo-10 -w
```

---

## Example 11: Dedicated ELB TLS Termination (HTTPS)

**Scenario**: Create a dedicated ELB with a TLS termination listener. The ELB handles SSL/TLS decryption, and the backend uses HTTP.

**Prerequisites**:
- CCM has been installed (see [Getting Started](./getting-started.md)).
- The AZ name has been obtained.
- A server certificate has been created in the Huawei Cloud ELB console, and the certificate ID has been obtained.

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>                    # Replace with the actual AZ
    kubernetes.io/elb.lb-algorithm: ROUND_ROBIN
    kubernetes.io/elb.default-tls-container-ref: <your-cert-id>       # Replace with the server certificate ID
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

> **Parameter Description**:
> - `kubernetes.io/elb.default-tls-container-ref` — Specifies the server certificate ID. When set, CCM will create a `TERMINATED_HTTPS` type listener.
> - The ELB receives HTTPS requests on port 443, decrypts them, and forwards them via HTTP to the backend Pod's port 80.

Verify (access via HTTPS):
```shell
kubectl get service loadbalancer-service-demo-11

# After obtaining the EXTERNAL-IP
curl -k https://<EXTERNAL-IP>
```

---

## Example 12: HTTP/HTTPS Timeout Configuration

**Scenario**: Configure idle timeout, request timeout, and response timeout for an ELB listener, applicable to HTTP/HTTPS protocols.

**Prerequisites**: CCM has been installed (see [Getting Started](./getting-started.md)).

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    kubernetes.io/elb.class: dedicated
    kubernetes.io/elb.availability-zones: <your-az>           # Replace with the actual AZ
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

> **Parameter Description**:
> - `kubernetes.io/elb.idle-timeout` — Idle timeout, range 0–4000 seconds.
> - `kubernetes.io/elb.request-timeout` — Request timeout, range 1–300 seconds (only valid for HTTP/HTTPS).
> - `kubernetes.io/elb.response-timeout` — Response timeout, range 1–300 seconds (only valid for HTTP/HTTPS).
> - `kubernetes.io/elb.x-forwarded-host: 'true'` — Enable `X-Forwarded-Host` header rewrite. When enabled, the ELB listener protocol is automatically set to HTTP (even if `protocol: TCP` is specified in the Service spec).

Verify:
```shell
kubectl get service loadbalancer-service-demo-12
```

---

## Example 13: Multi-Port Service

**Scenario**: Create a LoadBalancer Service that exposes multiple ports. The ELB will create a corresponding listener for each port.

**Prerequisites**: CCM has been installed (see [Getting Started](./getting-started.md)).

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

Verify:
```shell
kubectl get service loadbalancer-service-demo-13
```

Expected output:
```
NAME                           TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)                      AGE
loadbalancer-service-demo-13   LoadBalancer   10.1.130.100   192.168.0.200   80:30993/TCP,443:31443/TCP   2m
```

Test that both ports are accessible:
```shell
curl http://192.168.0.200:80
curl http://192.168.0.200:443
```

---

## Clean Up Resources

Delete the resources created in the examples as needed:

```shell
# Delete all example Services
kubectl delete service loadbalancer-service-demo-01 loadbalancer-service-demo-02 \
  loadbalancer-service-demo-03 loadbalancer-service-demo-04 loadbalancer-service-demo-05 \
  loadbalancer-service-demo-06 loadbalancer-service-demo-07 loadbalancer-service-demo-08 \
  loadbalancer-service-demo-09 loadbalancer-service-demo-10 loadbalancer-service-demo-11 \
  loadbalancer-service-demo-12 loadbalancer-service-demo-13

# Delete the test Deployment
kubectl delete deployment deployment-ccm-test

# (Optional) Uninstall CCM
kubectl delete -f https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/master/manifests/huawei-cloud-controller-manager-deployment.yaml
kubectl delete -f https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/master/manifests/rbac-huawei-cloud-controller-manager.yaml
kubectl delete secret cloud-config -n kube-system
```

> **Note**: After deleting a Service, CCM will automatically delete the auto-created ELB instance and EIP (unless `keep-eip: true` is set). Existing ELB instances referenced via `kubernetes.io/elb.id` will not be deleted.
>
> When uninstalling CCM, ensure the URL branch (`master` in the example above) matches the branch or release tag used during installation.

---

## Global Load Balancer Configuration (ConfigMap)

In addition to configuring parameters individually via Service Annotations, you can set global defaults through the `loadbalancer-config` ConfigMap.

For the full list of supported parameters and a ConfigMap example, see the [Loadbalancer Configuration](./huawei-cloud-controller-manager-configuration.md#loadbalancer-configuration) section of the CCM Configurations document.

> **Note**:
> - The configuration in the ConfigMap serves as global defaults. If the same parameter is set in a Service Annotation, the Annotation takes precedence.
> - After modifying the ConfigMap, you need to restart the CCM Pod for the changes to take effect:
>   ```shell
>   kubectl rollout restart deployment huawei-cloud-controller-manager -n kube-system
>   ```
