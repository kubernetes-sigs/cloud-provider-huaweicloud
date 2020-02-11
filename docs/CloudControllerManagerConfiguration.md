# Cloud Controller Manager Configuration
This document describes all of the configuration options available to the `HUAWEICLOUD` cloud controller manager.

## Configuration Example
We recommend you fetch the [example configuration](../examples/CloudControllerManagerConfigurationExample.json) to start 
your configuration and the details description about the configuration please see below.

## Description of configuration

#### Auth Options
In AK/SK-based authentication, AK/SK is used to sign requests and the signature is then added to the requests for 
authentication.

- AK: access key ID, which is a unique identifier used in conjunction with a secret access key to sign requests cryptographically.
- SK: secret access key used in conjunction with an AK to sign requests cryptographically. It identifies a request sender and prevents the request from being modified.

- SecretName：
    The `kubernetes.Secret` name in which stores the AK/SK pair. 
    This is an optional parameter unless you'd like to manage AK/SK dynamically.
    And you can get an example Secret from [here](./loadbalancers/secret.yaml).
- AccessKey: 
    The access key and you can get according to [Obtaining an AK/SK](https://support.huaweicloud.com/en-us/devg-apisign/api-sign-provide.html).
- SecretKey:
    The secret key and you can get according to [Obtaining an AK/SK](https://support.huaweicloud.com/en-us/devg-apisign/api-sign-provide.html).
- IAMEndpoint:
    IAMEndpoint is the IAM(Identity and Access Management) service's endpoint.
    Get it from https://developer.huaweicloud.com/en-us/endpoint according to your region.
- DomainID：
    DomainID is the account ID. You can get yours according to [How Can I Obtain domain_name, project_name, and project_id?](https://support.huaweicloud.com/en-us/devg-sdk/en-us_topic_0070637164.html).
- ProjectID：
    ProjectID is the project ID your workload working on.
    Get yours according to [How Can I Obtain domain_name, project_name, and project_id?](https://support.huaweicloud.com/en-us/devg-sdk/en-us_topic_0070637164.html).
- Region：
    Region is the region name.
- Cloud：
    Cloud is the cloud platform domain name.

#### LoadBalancer Options

The options under `LoadBalancer` section is planed to refactor later. And the description will be updated then.
