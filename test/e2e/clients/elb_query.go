/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clients

import (
	eipmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eip/v2/model"
	v2model "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v2/model"
	v3model "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v3/model"
	"github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/wrapper"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
)

// ========== Shared ELB (v2 API) query helpers ==========

func ListSharedListeners(authOpts *config.AuthOptions, elbID string) []v2model.ListenerResp {
	client := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
	listeners, err := client.ListListeners(&v2model.ListListenersRequest{
		LoadbalancerId: pointer.String(elbID),
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return listeners
}

func ListSharedPools(authOpts *config.AuthOptions, elbID string) []v2model.PoolResp {
	client := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
	pools, err := client.ListPools(&v2model.ListPoolsRequest{
		LoadbalancerId: pointer.String(elbID),
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return pools
}

func ListSharedMembers(authOpts *config.AuthOptions, poolID string) []v2model.MemberResp {
	client := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
	members, err := client.ListMembers(&v2model.ListMembersRequest{
		PoolId: poolID,
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return members
}

func GetSharedHealthMonitor(authOpts *config.AuthOptions, id string) *v2model.HealthmonitorResp {
	client := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
	hm, err := client.GetHealthMonitor(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return hm
}

func GetSharedLoadBalancer(authOpts *config.AuthOptions, id string) *v2model.LoadbalancerResp {
	client := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
	instance, err := client.GetInstance(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return instance
}

func GetSharedListener(authOpts *config.AuthOptions, id string) *v2model.ListenerResp {
	client := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
	listener, err := client.GetListener(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return listener
}

func GetSharedPool(authOpts *config.AuthOptions, id string) *v2model.PoolResp {
	client := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
	pool, err := client.GetPool(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return pool
}

// ========== Dedicated ELB (v3 API) query helpers ==========

func ListDedicatedListeners(authOpts *config.AuthOptions, elbID string) []v3model.Listener {
	client := wrapper.DedicatedLoadBalanceClient{AuthOpts: authOpts}
	ids := []string{elbID}
	listeners, err := client.ListListeners(&v3model.ListListenersRequest{
		LoadbalancerId: &ids,
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return listeners
}

func ListDedicatedPools(authOpts *config.AuthOptions, elbID string) []v3model.Pool {
	client := wrapper.DedicatedLoadBalanceClient{AuthOpts: authOpts}
	ids := []string{elbID}
	pools, err := client.ListPools(&v3model.ListPoolsRequest{
		LoadbalancerId: &ids,
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return pools
}

func ListDedicatedMembers(authOpts *config.AuthOptions, poolID string) []v3model.Member {
	client := wrapper.DedicatedLoadBalanceClient{AuthOpts: authOpts}
	members, err := client.ListMembers(&v3model.ListMembersRequest{
		PoolId: poolID,
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return members
}

func GetDedicatedHealthMonitor(authOpts *config.AuthOptions, id string) *v3model.HealthMonitor {
	client := wrapper.DedicatedLoadBalanceClient{AuthOpts: authOpts}
	hm, err := client.GetHealthMonitor(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return hm
}

func GetDedicatedLoadBalancer(authOpts *config.AuthOptions, id string) *v3model.LoadBalancer {
	client := wrapper.DedicatedLoadBalanceClient{AuthOpts: authOpts}
	instance, err := client.GetInstance(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return instance
}

func GetDedicatedListener(authOpts *config.AuthOptions, id string) *v3model.Listener {
	client := wrapper.DedicatedLoadBalanceClient{AuthOpts: authOpts}
	listener, err := client.GetListener(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return listener
}

func GetDedicatedPool(authOpts *config.AuthOptions, id string) *v3model.Pool {
	client := wrapper.DedicatedLoadBalanceClient{AuthOpts: authOpts}
	pool, err := client.GetPool(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return pool
}

// ========== EIP query helper ==========

func GetEip(authOpts *config.AuthOptions, id string) *eipmodel.PublicipShowResp {
	eipClient := wrapper.EIpClient{AuthOpts: authOpts}
	eip, err := eipClient.Get(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return eip
}
