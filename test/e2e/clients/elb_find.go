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
	"strings"

	v2model "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v2/model"
	v3model "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v3/model"
	"github.com/onsi/gomega"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/wrapper"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
)

// FindSharedELBID lists all shared ELB instances and returns the ID of the one
// whose name contains the given serviceName.
func FindSharedELBID(authOpts *config.AuthOptions, serviceName string) string {
	client := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
	instances, err := client.ListInstances(&v2model.ListLoadbalancersRequest{})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	for _, ins := range instances {
		if strings.Contains(ins.Name, serviceName) {
			return ins.Id
		}
	}
	gomega.ExpectWithOffset(1, false).To(gomega.BeTrue(),
		"should find shared ELB instance for service: "+serviceName)
	return ""
}

// FindDedicatedELBID lists all dedicated ELB instances and returns the ID of
// the one whose name contains the given serviceName.
func FindDedicatedELBID(authOpts *config.AuthOptions, serviceName string) string {
	client := wrapper.DedicatedLoadBalanceClient{AuthOpts: authOpts}
	guaranteed := true
	instances, err := client.ListInstances(&v3model.ListLoadBalancersRequest{
		Guaranteed: &guaranteed,
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	for _, ins := range instances {
		if strings.Contains(ins.Name, serviceName) {
			return ins.Id
		}
	}
	gomega.ExpectWithOffset(1, false).To(gomega.BeTrue(),
		"should find dedicated ELB instance for service: "+serviceName)
	return ""
}
