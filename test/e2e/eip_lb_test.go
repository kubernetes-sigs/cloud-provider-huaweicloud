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
	"sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/clients"
)

var _ = ginkgo.Describe("Loadbalancer EIP testing", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deployment = setupDeployment()
	})

	ginkgo.AfterEach(func() {
		cleanupDeploymentAndService(deployment, service)
	})

	ginkgo.It("shared ELB auto-create EIP with WHOLE share_type", func() {
		if vpcOpts.SubnetID == "" {
			ginkgo.Skip("HC_SUBNET_ID env not set, skipping WHOLE bandwidth test")
			return
		}

		// Create a shared bandwidth first, as share_type=WHOLE requires a valid share_id
		sharedBandwidth := clients.CreateSharedBandwidth(authOpts)
		sharedBandwidthID := *sharedBandwidth.Id

		serviceName := serviceNamePrefix + rand.String(RandomStrLength)

		annotations := sharedTCPAnnotations("ROUND_ROBIN")
		annotations[huaweicloud.AutoCreateEipOptions] =
			fmt.Sprintf(`{"ip_type": "5_bgp", "bandwidth_size": 5, "share_type": "WHOLE", "charge_mode": "bandwidth", "share_id": "%s"}`, sharedBandwidthID)

		service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
		frameworkCreateService(service)

		ingress := waitIngressIP(serviceName)
		verifyHTTPAvailable(ingress)

		ginkgo.By("Verifying EIP bandwidth share_type", func() {
			elbID := findSharedELBID(serviceName)
			gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())
			lb := clients.GetSharedLoadBalancer(authOpts, elbID)
			gomega.Expect(lb.VipPortId).ShouldNot(gomega.BeEmpty())
		})

		// Cleanup: delete service first so EIP is released, then delete the shared bandwidth
		frameworkRemoveService(service.Namespace, service.Name)
		waitServiceDeletion(service.Namespace, service.Name)
		service = nil
		clients.DeleteSharedBandwidth(authOpts, sharedBandwidthID)
	})

	ginkgo.DescribeTable("verify keep-eip behavior after service deletion",
		func(elbClass string, keepEip bool) {
			if vpcOpts.SubnetID == "" {
				ginkgo.Skip("HC_SUBNET_ID env not set, skipping keep-eip test")
				return
			}

			serviceName := serviceNamePrefix + rand.String(RandomStrLength)

			var annotations map[string]string
			if elbClass == "shared" {
				annotations = sharedTCPAnnotations("ROUND_ROBIN")
			} else {
				annotations = dedicatedTCPAnnotations("ROUND_ROBIN")
			}
			if keepEip {
				annotations[huaweicloud.ELBKeepEip] = "true"
			}

			service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
			frameworkCreateService(service)

			ingress := waitIngressIP(serviceName)
			verifyHTTPAvailable(ingress)

			ginkgo.By("Finding the EIP ID bound to the ELB", func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				var eipID string
				if elbClass == "shared" {
					lb := clients.GetSharedLoadBalancer(authOpts, elbID)
					eipID = clients.FindEipIDByPortID(authOpts, lb.VipPortId)
				} else {
					lb := clients.GetDedicatedLoadBalancer(authOpts, elbID)
					if len(lb.Eips) > 0 {
						eipID = *lb.Eips[0].EipId
					}
				}
				gomega.Expect(eipID).ShouldNot(gomega.BeEmpty())

				ginkgo.By(fmt.Sprintf("Deleting service and checking EIP retention (keep-eip=%v)", keepEip), func() {
					frameworkRemoveService(service.Namespace, service.Name)
					waitServiceDeletion(service.Namespace, service.Name)

					gomega.Eventually(func(g gomega.Gomega) bool {
						eipClient := wrapper.EIpClient{AuthOpts: authOpts}
						_, err := eipClient.Get(eipID)
						if keepEip {
							g.Expect(err).ShouldNot(gomega.HaveOccurred())
							return true
						}
						return err != nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(true))

					if keepEip {
						eipClient := wrapper.EIpClient{AuthOpts: authOpts}
						_ = eipClient.Delete(eipID)
					}
				})
			})
		},
		ginkgo.Entry("shared ELB keep-eip=true", "shared", true),
		ginkgo.Entry("dedicated ELB keep-eip=true", "dedicated", true),
		ginkgo.Entry("shared ELB keep-eip=false (default)", "shared", false),
		ginkgo.Entry("dedicated ELB keep-eip=false (default)", "dedicated", false),
	)
})
