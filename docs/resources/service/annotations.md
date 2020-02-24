There are several annotations you may used for your `Service` with `LoadBalancer` type.

## Specify load balancer class 
`kubernetes.io/elb.class` used to specify the load balancer classes which you want to use.
 Select an appropriate type based on application scenarios and your business needs:
- `kubernetes.io/elb.class: elasticity`: classic load balancer.
- `kubernetes.io/elb.class: union`: enhanced load balancer(instance is shared with others).
- `kubernetes.io/elb.class: performance`: enhanced load balancer(instance is not shared).
- `kubernetes.io/elb.class: dnat`: network address translation.

More docs:
- [Differences Between Classic and Enhanced Load Balancers](https://support.huaweicloud.com/en-us/productdesc-elb/en-us_elb_01_0007.html)
- [What Is NAT Gateway](https://support.huaweicloud.com/en-us/productdesc-natgateway/en-us_topic_0086739762.html) 

## Specify session affinity
These are optional annotations, it depends on if you want a session affinity feature.

#### Specify session affinity mode
`kubernetes.io/elb.session-affinity-mode` used to specify the session affinity mode. 
The available options are as follows:
- `kubernetes.io/elb.session-affinity-mode: SOURCE_IP`
- `kubernetes.io/elb.session-affinity-mode: ""`: disable affinity explicitly.

#### Specify session affinity options
`kubernetes.io/elb.session-affinity-option` used to specify the session affinity options if you 
decide to use session affinity feature.
Now, you can specify session affinity persistence timeout in range `[1, 60]` seconds, 
and the default value is `60s`. 

For example: 
- `kubernetes.io/elb.session-affinity-option: persistence_timeout=30`

## Specify load balancer metadata

You need to specify load balancer metadata which you have created before.

`kubernetes.io/elb.id` used to specify the load balancer ID and `kubernetes.io/elb.subnet-id` used to specify the load balancer subnet ID.

For example:
```
metadata:
  annotations:
    kubernetes.io/elb.id: 5884760e-ccf8-44b2-95e2-b850f111258c
    kubernetes.io/elb.subnet-id: 88d5ad7d-e20c-4ba8-9f00-10fd15201ccb
```