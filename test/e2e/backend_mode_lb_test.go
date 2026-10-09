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
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	"sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/clients"
)

var _ = ginkgo.Describe("Loadbalancer backend mode (ExternalTrafficPolicy) testing", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deployment = setupDeployment()
	})

	ginkgo.AfterEach(func() {
		cleanupDeploymentAndService(deployment, service)
	})

	ginkgo.DescribeTable("verify member type matches ExternalTrafficPolicy",
		func(elbClass, trafficPolicy string) {
			serviceName := serviceNamePrefix + rand.String(RandomStrLength)

			var annotations map[string]string
			if elbClass == "shared" {
				annotations = sharedTCPAnnotations("ROUND_ROBIN")
			} else {
				annotations = dedicatedTCPAnnotations("ROUND_ROBIN")
			}

			if trafficPolicy == "Local" {
				service = newLocalTrafficLoadbalancerService(testNamespace, serviceName, 80, annotations)
			} else {
				service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
			}
			frameworkCreateService(service)

			ingress := waitIngressIP(serviceName)

			if trafficPolicy == "Cluster" {
				verifyHTTPAvailable(ingress)
			}

			ginkgo.By(fmt.Sprintf("Verifying member addresses for %s ELB with ExternalTrafficPolicy=%s",
				elbClass, trafficPolicy), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				pods, err := kubeClient.CoreV1().Pods(testNamespace).List(
					context.TODO(), metav1.ListOptions{
						LabelSelector: "app=nginx",
					})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				podIPs := make(map[string]bool)
				podHostIPs := make(map[string]bool)
				for _, pod := range pods.Items {
					podIPs[pod.Status.PodIP] = true
					podHostIPs[pod.Status.HostIP] = true
				}

				gomega.Eventually(func(g gomega.Gomega) {
					var memberAddresses []string
					if elbClass == "shared" {
						pools := clients.ListSharedPools(authOpts, elbID)
						g.Expect(pools).ShouldNot(gomega.BeEmpty())
						members := clients.ListSharedMembers(authOpts, pools[0].Id)
						for _, m := range members {
							memberAddresses = append(memberAddresses, m.Address)
						}
					} else {
						pools := clients.ListDedicatedPools(authOpts, elbID)
						g.Expect(pools).ShouldNot(gomega.BeEmpty())
						members := clients.ListDedicatedMembers(authOpts, pools[0].Id)
						for _, m := range members {
							memberAddresses = append(memberAddresses, m.Address)
						}
					}

					if trafficPolicy == "Local" {
						// Dedicated ELB uses Pod IP (IpTargetEnable); Shared ELB uses Node IP but only for pod-bearing nodes.
						if elbClass == "dedicated" {
							for _, addr := range memberAddresses {
								g.Expect(podIPs[addr]).Should(gomega.BeTrue(),
									fmt.Sprintf("member address %s should be a Pod IP (Local mode)", addr))
							}
						} else {
							for _, addr := range memberAddresses {
								g.Expect(podHostIPs[addr]).Should(gomega.BeTrue(),
									fmt.Sprintf("member address %s should be a Node IP with a pod (Local mode)", addr))
							}
						}
					} else {
						for _, addr := range memberAddresses {
							g.Expect(podIPs[addr]).Should(gomega.BeFalse(),
								"member address should be a Node IP, not a Pod IP (Cluster mode)")
						}
					}
				}, pollTimeout, pollInterval).Should(gomega.Succeed())
			})
		},
		ginkgo.Entry("shared ELB with Local traffic policy", "shared", "Local"),
		ginkgo.Entry("dedicated ELB with Local traffic policy", "dedicated", "Local"),
		ginkgo.Entry("shared ELB with Cluster traffic policy", "shared", "Cluster"),
		ginkgo.Entry("dedicated ELB with Cluster traffic policy", "dedicated", "Cluster"),
	)
})
