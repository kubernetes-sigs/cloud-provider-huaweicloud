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
	"sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/clients"
)

var _ = ginkgo.Describe("Loadbalancer annotation update testing", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deployment = setupDeployment()
	})

	ginkgo.AfterEach(func() {
		cleanupDeploymentAndService(deployment, service)
	})

	ginkgo.DescribeTable("verify ELB config updates when annotations change",
		func(elbClass string) {
			serviceName := serviceNamePrefix + rand.String(RandomStrLength)

			var annotations map[string]string
			if elbClass == "shared" {
				annotations = sharedTCPAnnotations("ROUND_ROBIN")
			} else {
				annotations = dedicatedTCPAnnotations("ROUND_ROBIN")
			}

			service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
			frameworkCreateService(service)

			ingress := waitIngressIP(serviceName)
			verifyHTTPAvailable(ingress)

			ginkgo.By(fmt.Sprintf("Verifying initial lb_algorithm is ROUND_ROBIN on %s ELB", elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				if elbClass == "shared" {
					pools := clients.ListSharedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					gomega.Expect(pools[0].LbAlgorithm.Value()).Should(gomega.Equal("ROUND_ROBIN"))
				} else {
					pools := clients.ListDedicatedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					gomega.Expect(pools[0].LbAlgorithm).Should(gomega.Equal("ROUND_ROBIN"))
				}
			})

			ginkgo.By("Updating lb_algorithm annotation to LEAST_CONNECTIONS", func() {
				updatedAnnotations := annotations
				updatedAnnotations[huaweicloud.ElbAlgorithm] = "LEAST_CONNECTIONS"
				updateServiceAnnotations(testNamespace, serviceName, updatedAnnotations)
			})

			ginkgo.By(fmt.Sprintf("Verifying lb_algorithm changed to LEAST_CONNECTIONS on %s ELB", elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				gomega.Eventually(func(g gomega.Gomega) string {
					if elbClass == "shared" {
						pools := clients.ListSharedPools(authOpts, elbID)
						g.Expect(pools).ShouldNot(gomega.BeEmpty())
						return pools[0].LbAlgorithm.Value()
					}
					pools := clients.ListDedicatedPools(authOpts, elbID)
					g.Expect(pools).ShouldNot(gomega.BeEmpty())
					return pools[0].LbAlgorithm
				}, pollTimeout, pollInterval).Should(gomega.Equal("LEAST_CONNECTIONS"))
			})
		},
		ginkgo.Entry("shared ELB", "shared"),
		ginkgo.Entry("dedicated ELB", "dedicated"),
	)

	ginkgo.It("dedicated ELB health-check can be toggled off via annotation update", func() {
		serviceName := serviceNamePrefix + rand.String(RandomStrLength)

		annotations := dedicatedTCPAnnotations("ROUND_ROBIN")
		annotations[huaweicloud.ElbHealthCheckFlag] = "on"

		service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
		frameworkCreateService(service)

		ingress := waitIngressIP(serviceName)
		verifyHTTPAvailable(ingress)

		ginkgo.By("Verifying health monitor exists initially", func() {
			elbID := findDedicatedELBID(serviceName)
			pools := clients.ListDedicatedPools(authOpts, elbID)
			gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
			gomega.Expect(pools[0].HealthmonitorId).ShouldNot(gomega.BeEmpty())
		})

		ginkgo.By("Updating health-check-flag to off", func() {
			updatedAnnotations := annotations
			updatedAnnotations[huaweicloud.ElbHealthCheckFlag] = "off"
			updateServiceAnnotations(testNamespace, serviceName, updatedAnnotations)
		})

		ginkgo.By("Verifying health monitor is removed after annotation update", func() {
			elbID := findDedicatedELBID(serviceName)
			gomega.Eventually(func(g gomega.Gomega) bool {
				pools := clients.ListDedicatedPools(authOpts, elbID)
				g.Expect(pools).ShouldNot(gomega.BeEmpty())
				return pools[0].HealthmonitorId == ""
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())
		})
	})
})
