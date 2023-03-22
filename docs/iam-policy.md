# IAM policy of Huawei Cloud Kubernetes Cloud Provider

The following policy content is the minimum permissions used by Kubernetes CCM on HUAWEI CLOUD.
You can customize a policy and grant it to the account used by CCM.

```json
{
    "Version": "1.1",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ELB:*:*"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "ecs:cloudServers:listServerBlockDevices",
                "ecs:cloudServerFlavors:get",
                "ecs:diskConfigs:use",
                "ecs:cloudServers:showServerBlockDevice",
                "ecs:networks:list",
                "ecs:cloudServers:showServer",
                "ecs:servers:getMetadata",
                "ecs:cloudServers:showResetPasswordFlag",
                "ecs:cloudServers:get",
                "ecs:cloudServers:listServerInterfaces",
                "ecs:serverInterfaces:get",
                "ecs:cloudServerFpgaImages:getRelations",
                "ecs:servers:list",
                "ecs:cloudServers:getAutoRecovery",
                "ecs:serverKeypairs:get",
                "ecs:quotas:get",
                "ecs:cloudServers:showServerTags",
                "ecs:cloudServerQuotas:get",
                "ecs:cloudServers:listServerVolumeAttachments",
                "ecs:flavors:get",
                "ecs:cloudServers:list",
                "ecs:serverVolumeAttachments:get",
                "ecs:cloudServerFpgaImages:list",
                "ecs:serverKeypairs:list",
                "ecs:serverVolumes:use",
                "ecs:servers:getTags",
                "ecs:serverVolumeAttachments:list",
                "ecs:servers:listMetadata",
                "ecs:servers:get",
                "ecs:availabilityZones:list",
                "ecs:securityGroups:use"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "vpc:publicIps:update",
                "vpc:securityGroupRules:create",
                "vpc:securityGroups:create",
                "vpc:routes:list",
                "vpc:firewallRules:get",
                "vpc:securityGroupRules:delete",
                "vpc:peerings:get",
                "vpc:vpcTags:get",
                "vpc:vpcs:list",
                "vpc:firewalls:get",
                "vpc:networks:get",
                "vpc:publicIps:create",
                "vpc:floatingIps:get",
                "vpc:ports:get",
                "vpc:publicIps:remove",
                "vpc:firewalls:list",
                "vpc:privateIps:list",
                "vpc:quotas:list",
                "vpc:addressGroups:list",
                "vpc:privateIps:get",
                "vpc:routeTables:get",
                "vpc:subnetTags:get",
                "vpc:routes:get",
                "vpc:routeTables:list",
                "vpc:bandwidths:get",
                "vpc:publicIps:insert",
                "vpc:firewallPolicies:get",
                "vpc:addressGroups:get",
                "vpc:bandwidths:list",
                "vpc:securityGroupRules:get",
                "vpc:securityGroups:delete",
                "vpc:vpcs:get",
                "vpc:publicIps:delete",
                "vpc:subnets:get",
                "vpc:securityGroups:update",
                "vpc:subNetworkInterfaces:get",
                "vpc:routers:get",
                "vpc:securityGroupRules:update",
                "vpc:publicIps:list",
                "vpc:securityGroups:get",
                "vpc:firewallGroups:get",
                "vpc:publicIps:get",
                "vpc:publicipTags:get",
                "vpc:subNetworkInterfaces:list"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "evs:volumes:create",
                "evs:backups:get",
                "evs:volumes:get",
                "evs:snapshots:get",
                "evs:volumes:delete"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "ims:images:get"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "EIP:*:*"
            ]
        }
    ]
}
```
