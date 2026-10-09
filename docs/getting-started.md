# Running on an Existing Kubernetes Cluster on Huawei Cloud

> [English](./getting-started.md) | [中文](./zh/getting-started.md)

## Prerequisites

- A Kubernetes cluster running on Huawei Cloud (self-managed or CCE).
- Huawei Cloud account credentials have been obtained:
  - AK (Access Key) / SK (Secret Key), obtainable from the "My Credentials" page in the Huawei Cloud console.
  - The region where the cluster is located (e.g., `cn-north-4`).
  - Project ID, obtainable from the "My Credentials" page in the Huawei Cloud console.
  - VPC ID and Subnet ID, viewable in the VPC console.
- IAM Permissions: The AK/SK used by CCM must have permissions for ELB, ECS, VPC, EVS, EIP, IMS, etc. See the [IAM Policy](./iam-policy.md) documentation.
- Security Group: The cluster node security group must allow the ELB health check CIDR `100.125.0.0/16`.

## Update Cluster Configurations

- Add `--cloud-provider=external` to kube-controller-manager and kube-apiserver config

`kube-apiserver` and `kube-controller-manager` **MUST NOT** specify the `--cloud-provider` flag
(or specify `--cloud-provider=external`). This ensures that it does not run any cloud specific loops that would be run
by cloud controller manager.

- Add `--cloud-provider=external` to kubelet on each node

`kubelet` MUST run with `--cloud-provider=external`. This is to ensure that the kubelet is aware that it must be
initialized by the cloud controller manager before it is scheduled any work.

## Step 1: Create the cloud-config Secret

### 1.1 Create the cloud-config file

Create the `cloud-config` file on the control node (replace the values in angle brackets with your actual values):

```shell
cat <<EOF > ./cloud-config
[Global]
region=<your-region>                    # e.g.: cn-north-4
access-key=<your-access-key>            # Huawei Cloud AK
secret-key=<your-secret-key>            # Huawei Cloud SK
project-id=<your-project-id>            # Project ID (optional, but recommended)

[Vpc]
id=<your-vpc-id>                        # VPC ID where the cluster is located
subnet-id=<your-subnet-id>              # Subnet ID (IPv4) where the cluster is located
security-group-id=<your-security-group-id>  # Security group ID (optional)
EOF

# Check if the file exists.
ls -l | grep cloud-config
```

> See [Huawei Cloud Controller Manager Configurations](./huawei-cloud-controller-manager-configuration.md)
> for a full description of supported arguments.

### 1.2 Create the Secret in Kubernetes

```shell
kubectl create secret -n kube-system generic cloud-config --from-file=./cloud-config

# Verify the Secret was created
kubectl get secret cloud-config -n kube-system

# Detailed view
kubectl describe secret cloud-config -n kube-system
```

Expected output:
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

### 1.3 Delete the local cloud-config file

```shell
rm -rf ./cloud-config
```

> **Note**: After modifying the cloud-config Secret, CCM needs to be restarted to load the new configuration.

## Step 2: Create RBAC Resources

```shell
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/master/manifests/rbac-huawei-cloud-controller-manager.yaml
```

## Step 3: Deploy CCM

**Option A: Using Deployment (Recommended)**

```shell
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/master/manifests/huawei-cloud-controller-manager-deployment.yaml
```

**Option B: Using DaemonSet**

```shell
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-huaweicloud/master/manifests/huawei-cloud-controller-manager-daemonset.yaml
```

## Step 4: Verify Installation

```shell
kubectl get pod -n kube-system | grep huawei-cloud-controller-manager
```

Expected output (Pod status `Running` indicates successful installation):
```
huawei-cloud-controller-manager-5f4b7995fc-s6b7p   1/1     Running   0          2m36s
```

## What's Next

- [Service Annotations](./usage-guide.md) — Reference for all supported Service annotations.
- [Usage Examples](./usage-examples.md) — Complete executable examples from basic to advanced ELB scenarios.
- [CCM Configurations](./huawei-cloud-controller-manager-configuration.md) — Full reference for cloud-config and loadbalancer-config.
- [IAM Policy](./iam-policy.md) — Minimum IAM permissions required by CCM.
