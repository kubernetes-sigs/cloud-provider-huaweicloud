This document describes how to run `huawei-cloud-controller-manager` with local cluster which is setup by [kubernetes local-up-cluster](https://github.com/kubernetes/kubernetes/blob/95504c32fe1fcd1ee97879cb508f3a57fe90ca81/hack/local-up-cluster.sh).

## Precondition

#### 1. Kubernetes source code
You need to clone [kubernetes](https://github.com/kubernetes/kubernetes) first.
You can do this by following command:
```
# git clone https://github.com/kubernetes/kubernetes.git
```

#### 2. local cluster runs well without `huawei-cloud-controller-manager`
Before start, it is recommend that run your local cluster without `huawei-cloud-controller-manager` first,
It will help to make sure everything is OK. If there is something wrong, please dig it out and then continue the follows. 

The way to run a local cluster is extremely simple. 
What you need to do is run `hack/local-up-cluster.sh` in the `kubernetes root` directory:
```
# hack/local-up-cluster.sh
``` 
`hack/local-up-cluster.sh` will compile all kubernetes components first and run them one by one.
If there is something going wrong and you can't dig out by yourself, please consider to [file an issue](https://github.com/kubernetes/kubernetes/issues/new/choose).

## Run
#### Build `huawei-cloud-controller-manager`

Build the `huawei-cloud-controller-manager` binary as per [Building Cloud Controller Manager](../README.md#https://github.com/huawei-cloudnative/cloud-provider-huaweicloud#building-cloud-controller-manager).

And then, copy the binary to your workspace, like `/root/provider/`:
```
# mkdir /root/provider
# cp huawei-cloud-controller-manager /root/provider
``` 

#### Prepare configuration
You should prepare a appropriate configuration as per [Cloud Controller Manager Configuration](./config/CloudControllerManagerConfiguration.md).

You can put your configuration to your workspace, e.g. `/root/provider/provider.conf`.

#### Set environment
You should set a bunch of environments for `hack/local-up-cluster.sh`: 
```
export EXTERNAL_CLOUD_PROVIDER=true
export EXTERNAL_CLOUD_PROVIDER_BINARY=/root/provider/huawei-cloud-controller-manager
export CLOUD_PROVIDER="huaweicloud"
export CLOUD_CONFIG=/root/provider/provider.conf

# Why: add elb health check member who must not be 127.0.0.1.
export HOSTNAME_OVERRIDE="192.168.1.122"

# Why: default --provider-id is hostname for kubelet. We should change it to ecs uuid. 
export KUBELET_PROVIDER_ID="a44af098-7548-4519-8243-a88ba3e5de4f"

# Run conformance testing 
export ALLOW_PRIVILEGED=true
export ALLOW_SECURITY_CONTEXT=true
export ENABLE_CRI=false
#export ENABLE_DAEMON=true
export ENABLE_HOSTPATH_PROVISIONER=true
export ENABLE_SINGLE_CA_SIGNER=true
export HOSTNAME_OVERRIDE=$(ip route get 1.1.1.1 | awk '{print $7}')
export KUBE_ENABLE_CLUSTER_DASHBOARD=true
export KUBE_ENABLE_CLUSTER_DNS=true
export LOG_LEVEL=10
export KUBELET_HOST="0.0.0.0"
export API_HOST_IP="172.17.0.1"
export API_HOST="172.17.0.1"
```

#### Startup cluster
Then, what you need to do is run `hack/local-up-cluster.sh` in the `kubernetes root` directory:
```
# hack/local-up-cluster.sh
```  

## Debug
You can get the `huawei-cloud-controller-manager` logs from `/tmp/cloud-controller-manager.log`.
Anything going wrong, please feel free to report an issue.