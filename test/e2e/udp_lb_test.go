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

var _ = ginkgo.Describe("Loadbalancer UDP protocol testing", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deployment = setupDeployment()
	})

	ginkgo.AfterEach(func() {
		cleanupDeploymentAndService(deployment, service)
	})

	ginkgo.DescribeTable("verify UDP listener and health monitor via ELB API",
		func(elbClass string) {
			serviceName := serviceNamePrefix + rand.String(RandomStrLength)

			var annotations map[string]string
			if elbClass == "shared" {
				annotations = sharedTCPAnnotations("ROUND_ROBIN")
			} else {
				annotations = dedicatedTCPAnnotations("ROUND_ROBIN")
			}

			service = newUDPLoadbalancerService(testNamespace, serviceName, 8080, annotations)
			frameworkCreateService(service)

			ingress := waitIngressIP(serviceName)

			ginkgo.By(fmt.Sprintf("Waiting for UDP service on %s ELB (ingress: %s)", elbClass, ingress), func() {
				gomega.Expect(ingress).ShouldNot(gomega.BeEmpty())
			})

			ginkgo.By(fmt.Sprintf("Verifying listener protocol is UDP on %s ELB", elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				if elbClass == "shared" {
					listeners := clients.ListSharedListeners(authOpts, elbID)
					gomega.Expect(listeners).ShouldNot(gomega.BeEmpty())
					gomega.Expect(listeners[0].Protocol.Value()).Should(gomega.Equal("UDP"))
					gomega.Expect(listeners[0].ProtocolPort).Should(gomega.Equal(int32(8080)))
				} else {
					listeners := clients.ListDedicatedListeners(authOpts, elbID)
					gomega.Expect(listeners).ShouldNot(gomega.BeEmpty())
					gomega.Expect(listeners[0].Protocol).Should(gomega.Equal("UDP"))
					gomega.Expect(listeners[0].ProtocolPort).Should(gomega.Equal(int32(8080)))
				}
			})

			ginkgo.By(fmt.Sprintf("Verifying health monitor type is UDP_CONNECT on %s ELB", elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				if elbClass == "shared" {
					pools := clients.ListSharedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					hmID := pools[0].HealthmonitorId
					if hmID != "" {
						hm := clients.GetSharedHealthMonitor(authOpts, hmID)
						gomega.Expect(hm.Type.Value()).Should(gomega.Equal("UDP_CONNECT"))
					}
				} else {
					pools := clients.ListDedicatedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					hmID := pools[0].HealthmonitorId
					if hmID != "" {
						hm := clients.GetDedicatedHealthMonitor(authOpts, hmID)
						gomega.Expect(hm.Type).Should(gomega.Equal("UDP_CONNECT"))
					}
				}
			})
		},
		ginkgo.Entry("shared ELB UDP", "shared"),
		ginkgo.Entry("dedicated ELB UDP", "dedicated"),
	)
})
