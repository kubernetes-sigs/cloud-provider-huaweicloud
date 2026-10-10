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

package e2e

import (
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/wrapper"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/common"
	"sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/clients"
)

var _ = ginkgo.Describe("Loadbalancer health-check testing", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deployment = setupDeployment()
	})

	ginkgo.AfterEach(func() {
		cleanupDeploymentAndService(deployment, service)
	})

	ginkgo.DescribeTable("verify health-check-flag=off removes health monitor",
		func(elbClass string) {
			serviceName := serviceNamePrefix + rand.String(RandomStrLength)

			var annotations map[string]string
			if elbClass == "shared" {
				annotations = sharedTCPAnnotations("ROUND_ROBIN")
			} else {
				annotations = dedicatedTCPAnnotations("ROUND_ROBIN")
			}
			annotations[huaweicloud.ElbHealthCheckFlag] = "off"

			service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
			frameworkCreateService(service)

			ingress := waitIngressIP(serviceName)
			verifyHTTPAvailable(ingress)

			ginkgo.By(fmt.Sprintf("Verifying health monitor is absent on %s ELB (health-check-flag=off)", elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				if elbClass == "shared" {
					pools := clients.ListSharedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					hmID := pools[0].HealthmonitorId
					if hmID != "" {
						sharedClient := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
						_, err := sharedClient.GetHealthMonitor(hmID)
						gomega.Expect(common.IsNotFound(err)).Should(gomega.BeTrue(),
							"health monitor should not exist when health-check-flag=off")
					}
				} else {
					pools := clients.ListDedicatedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					hmID := pools[0].HealthmonitorId
					if hmID != "" {
						dedicatedClient := wrapper.DedicatedLoadBalanceClient{AuthOpts: authOpts}
						_, err := dedicatedClient.GetHealthMonitor(hmID)
						gomega.Expect(common.IsNotFound(err)).Should(gomega.BeTrue(),
							"health monitor should not exist when health-check-flag=off")
					}
				}
			})
		},
		ginkgo.Entry("shared ELB", "shared"),
		ginkgo.Entry("dedicated ELB", "dedicated"),
	)

	ginkgo.DescribeTable("verify health-check-flag=on creates health monitor with correct params",
		func(elbClass string) {
			serviceName := serviceNamePrefix + rand.String(RandomStrLength)

			var annotations map[string]string
			if elbClass == "shared" {
				annotations = sharedTCPAnnotations("ROUND_ROBIN")
				annotations[huaweicloud.ElbHealthCheckOptions] = `{"delay": 7, "timeout": 12, "max_retries": 5}`
			} else {
				annotations = dedicatedTCPAnnotations("ROUND_ROBIN")
				annotations[huaweicloud.ElbHealthCheckOptions] = `{"delay": 7, "timeout": 12, "max_retries": 5}`
			}

			service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
			frameworkCreateService(service)

			ingress := waitIngressIP(serviceName)
			verifyHTTPAvailable(ingress)

			ginkgo.By(fmt.Sprintf("Verifying health monitor params on %s ELB", elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				if elbClass == "shared" {
					pools := clients.ListSharedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					hmID := pools[0].HealthmonitorId
					gomega.Expect(hmID).ShouldNot(gomega.BeEmpty())
					hm := clients.GetSharedHealthMonitor(authOpts, hmID)
					gomega.Expect(hm.Delay).Should(gomega.Equal(int32(7)))
					gomega.Expect(hm.Timeout).Should(gomega.Equal(int32(12)))
					gomega.Expect(hm.MaxRetries).Should(gomega.Equal(int32(5)))
				} else {
					pools := clients.ListDedicatedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					hmID := pools[0].HealthmonitorId
					gomega.Expect(hmID).ShouldNot(gomega.BeEmpty())
					hm := clients.GetDedicatedHealthMonitor(authOpts, hmID)
					gomega.Expect(hm.Delay).Should(gomega.Equal(int32(7)))
					gomega.Expect(hm.Timeout).Should(gomega.Equal(int32(12)))
					gomega.Expect(hm.MaxRetries).Should(gomega.Equal(int32(5)))
				}
			})
		},
		ginkgo.Entry("shared ELB", "shared"),
		ginkgo.Entry("dedicated ELB", "dedicated"),
	)
})
