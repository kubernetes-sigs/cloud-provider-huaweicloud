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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"

	"sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/clients"
)

var _ = ginkgo.Describe("Loadbalancer multi-port service testing", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deployment = setupDeployment()
	})

	ginkgo.AfterEach(func() {
		cleanupDeploymentAndService(deployment, service)
	})

	ginkgo.DescribeTable("verify multiple listeners created for multi-port service",
		func(elbClass string) {
			serviceName := serviceNamePrefix + rand.String(RandomStrLength)

			var annotations map[string]string
			if elbClass == "shared" {
				annotations = sharedTCPAnnotations("ROUND_ROBIN")
			} else {
				annotations = dedicatedTCPAnnotations("ROUND_ROBIN")
			}

			ports := []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.IntOrString{IntVal: 80},
				},
				{
					Name:       "http2",
					Protocol:   corev1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.IntOrString{IntVal: 80},
				},
			}

			service = newMultiPortLoadbalancerService(testNamespace, serviceName, ports, annotations)
			frameworkCreateService(service)

			ingress := waitIngressIP(serviceName)

			ginkgo.By(fmt.Sprintf("Verifying 2 listeners on %s ELB for multi-port service", elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				gomega.Eventually(func(g gomega.Gomega) int {
					if elbClass == "shared" {
						listeners := clients.ListSharedListeners(authOpts, elbID)
						return len(listeners)
					}
					listeners := clients.ListDedicatedListeners(authOpts, elbID)
					return len(listeners)
				}, pollTimeout, pollInterval).Should(gomega.Equal(2))
			})

			ginkgo.By(fmt.Sprintf("Verifying 2 pools on %s ELB for multi-port service", elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				gomega.Eventually(func(g gomega.Gomega) int {
					if elbClass == "shared" {
						pools := clients.ListSharedPools(authOpts, elbID)
						return len(pools)
					}
					pools := clients.ListDedicatedPools(authOpts, elbID)
					return len(pools)
				}, pollTimeout, pollInterval).Should(gomega.Equal(2))
			})

			ginkgo.By("Verifying both ports are reachable", func() {
				verifyHTTPAvailable(fmt.Sprintf("%s", ingress))
				verifyHTTPAvailable(fmt.Sprintf("%s:8080", ingress))
			})
		},
		ginkgo.Entry("shared ELB", "shared"),
		ginkgo.Entry("dedicated ELB", "dedicated"),
	)
})
