# Create the cloud-config secret in Kubernetes cluster

Step 1: Create the `cloud-config` file in your master node or control-plane.

```shell
cat <<EOF >  ./cloud-config
[Global]
region=ap-southeast-1
access-key=assCNKG3CHF1skH6rJGIk
secret-key=xxxO8ZE8BGCUDUCbvZCrGDRixO52323q8Aict6
project-id=62fac0038c52b5eb0970dd9c1a0026a

[Vpc]
# The VPC where your cluster resides
id=0b876fcb-6a0b-47a3-8067-80fe9245a3da
subnet-id=d2aa75cc-e356-4dc3-abb0-87078923a168
security-group-id=48288809-1234-5678-abcd-eb30b2d16cd4

EOF

# Check if the file exists.
$ ls -l | grep cloud-config
-rw-r--r-- 1 root root   290 Dec 29 20:15 cloud-config
```

Step 2: Create the `cloud-config` secret.

```shell
$ kubectl create secret -n kube-system generic cloud-config --from-file=./cloud-config

secret/cloud-config created
```

Check whether the secret is created successfully.

```shell
$ kubectl get secret cloud-config  -n kube-system
NAME           TYPE     DATA   AGE
cloud-config   Opaque   1      13s

$ kubectl describe secret cloud-config  -n kube-system
Name:         cloud-config
Namespace:    kube-system
Labels:       <none>
Annotations:  <none>

Type:  Opaque

Data
====
cloud-config:  290 bytes
```

Step 3: Delete the cloud-config file.

```shell
$ rm -rf ./cloud-config
```

See [huawei-cloud-controller-manager-configuration](./huawei-cloud-controller-manager-configuration.md)
for supported arguments.
