# Signer
`Signer` used to verify if your authentication information is correct or suitable for your environment. 

The main process is retrieve a server's name, if the server's name can be retrieved means your authentication is 
correct and suitable for your environment.

## Quick Start

### Prepare Authentication
Suppose you have prepared a configuration file as per [configuration guide](../../docs/config/CloudControllerManagerConfiguration.md).

Set your configuration file path to environment, e.g.: 
```shell script
export CCMConfigPath=/root/provider/provider.conf
```

In addition, you need to provide a server ID, e.g.:
```shell script
export ServerID=a44af098-7548-4519-8243-a88ba3e5de4f
``` 

### Run
Just run following command under `cloud-provider-huaweicloud` home directory as follows:
```shell script
[root@ecs-d8b6 cloud-provider-huaweicloud]# go run ./test/signer/v4/main.go 
Congratulations! Your authentication information is suitable. server name: ecs-d8b6
```

If everything works fine, you will get a `Congratulations` as well as the server's name. 
Otherwise, you will see an error message.

## Signer Version
All request against cloud services should be signed, that usually processed by SDK.
Different cloud environment rely on specific signature algorithm version. 

Currently, we are using [huaweicloud-sdk-go-v3](https://github.com/huaweicloud/huaweicloud-sdk-go-v3) which rely on 
signature algorithm `v4`.
