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

	"sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/clients"
)

var _ = ginkgo.Describe("Loadbalancer reconcile (endpoint change) testing", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		deployment = newDeploymentWithReplicas(testNamespace, deploymentName, 1)
		frameworkCreateDeployment(deployment)
	})

	ginkgo.AfterEach(func() {
		cleanupDeploymentAndService(deployment, service)
	})

	ginkgo.DescribeTable("verify member list changes when deployment scales",
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

			ginkgo.By(fmt.Sprintf("Checking initial member count is 1 on %s ELB", elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				gomega.Eventually(func(g gomega.Gomega) int {
					if elbClass == "shared" {
						pools := clients.ListSharedPools(authOpts, elbID)
						g.Expect(pools).ShouldNot(gomega.BeEmpty())
						members := clients.ListSharedMembers(authOpts, pools[0].Id)
						return len(members)
					}
					pools := clients.ListDedicatedPools(authOpts, elbID)
					g.Expect(pools).ShouldNot(gomega.BeEmpty())
					members := clients.ListDedicatedMembers(authOpts, pools[0].Id)
					return len(members)
				}, pollTimeout, pollInterval).Should(gomega.Equal(1))
			})

			ginkgo.By("Scaling deployment to 3 replicas", func() {
				scaleDeployment(testNamespace, deployment.Name, 3)
			})

			ginkgo.By(fmt.Sprintf("Checking member count is 3 on %s ELB after scale up", elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				gomega.Eventually(func(g gomega.Gomega) int {
					if elbClass == "shared" {
						pools := clients.ListSharedPools(authOpts, elbID)
						g.Expect(pools).ShouldNot(gomega.BeEmpty())
						members := clients.ListSharedMembers(authOpts, pools[0].Id)
						return len(members)
					}
					pools := clients.ListDedicatedPools(authOpts, elbID)
					g.Expect(pools).ShouldNot(gomega.BeEmpty())
					members := clients.ListDedicatedMembers(authOpts, pools[0].Id)
					return len(members)
				}, pollTimeout, pollInterval).Should(gomega.Equal(3))
			})

			ginkgo.By("Scaling deployment back to 1 replica", func() {
				scaleDeployment(testNamespace, deployment.Name, 1)
			})

			ginkgo.By(fmt.Sprintf("Checking member count is 1 on %s ELB after scale down", elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				gomega.Eventually(func(g gomega.Gomega) int {
					if elbClass == "shared" {
						pools := clients.ListSharedPools(authOpts, elbID)
						g.Expect(pools).ShouldNot(gomega.BeEmpty())
						members := clients.ListSharedMembers(authOpts, pools[0].Id)
						return len(members)
					}
					pools := clients.ListDedicatedPools(authOpts, elbID)
					g.Expect(pools).ShouldNot(gomega.BeEmpty())
					members := clients.ListDedicatedMembers(authOpts, pools[0].Id)
					return len(members)
				}, pollTimeout, pollInterval).Should(gomega.Equal(1))
			})
		},
		ginkgo.Entry("shared ELB", "shared"),
		ginkgo.Entry("dedicated ELB", "dedicated"),
	)
})
