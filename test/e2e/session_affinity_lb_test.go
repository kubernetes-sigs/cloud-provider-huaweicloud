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

var _ = ginkgo.Describe("Loadbalancer session-affinity testing", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deployment = setupDeployment()
	})

	ginkgo.AfterEach(func() {
		cleanupDeploymentAndService(deployment, service)
	})

	ginkgo.DescribeTable("verify session affinity type via ELB API",
		func(elbClass, sessionType string) {
			serviceName := serviceNamePrefix + rand.String(RandomStrLength)

			// cookie_name is required when type is APP_COOKIE; persistence_timeout is invalid for APP_COOKIE.
			var sessionAffinityOption string
			if sessionType == "APP_COOKIE" {
				sessionAffinityOption = fmt.Sprintf(`{"type":"%s","cookie_name":"mycookie"}`, sessionType)
			} else {
				sessionAffinityOption = fmt.Sprintf(`{"type":"%s","persistence_timeout":20}`, sessionType)
			}

			annotations := map[string]string{
				huaweicloud.ElbSessionAffinityFlag:   "on",
				huaweicloud.ElbSessionAffinityOption: sessionAffinityOption,
				huaweicloud.AutoCreateEipOptions:     `{"ip_type": "5_bgp", "bandwidth_size": 5, "share_type": "PER", "charge_mode": "bandwidth"}`,
			}

			if elbClass == "shared" {
				annotations[huaweicloud.ElbClass] = "shared"
				annotations[huaweicloud.ElbAlgorithm] = "ROUND_ROBIN"
				annotations[huaweicloud.ElbHealthCheckFlag] = "on"
				annotations[huaweicloud.ElbHealthCheckOptions] = `{"delay": 3, "timeout": 15, "max_retries": 3}`
			} else {
				annotations[huaweicloud.ElbClass] = "dedicated"
				annotations[huaweicloud.ElbAvailabilityZones] = getAZ()
				annotations[huaweicloud.ElbAlgorithm] = "ROUND_ROBIN"
				annotations[huaweicloud.ElbHealthCheckFlag] = "on"
				annotations[huaweicloud.ElbHealthCheckOptions] = `{"delay": 4, "timeout": 16, "max_retries": 4}`
				annotations[huaweicloud.ElbXForwardedHost] = "true"
			}

			service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
			frameworkCreateService(service)

			ingress := waitIngressIP(serviceName)
			verifyHTTPAvailable(ingress)

			ginkgo.By(fmt.Sprintf("Verifying session affinity type is %s on %s ELB", sessionType, elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				if elbClass == "shared" {
					pools := clients.ListSharedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					gomega.Expect(pools[0].SessionPersistence).ShouldNot(gomega.BeNil())
					gomega.Expect(pools[0].SessionPersistence.Type.Value()).Should(gomega.Equal(sessionType))
					// cookie_name is only valid when type is APP_COOKIE; in other cases it must be nil.
					if sessionType != "APP_COOKIE" {
						gomega.Expect(pools[0].SessionPersistence.CookieName).Should(gomega.BeNil())
					}
				} else {
					pools := clients.ListDedicatedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					gomega.Expect(pools[0].SessionPersistence).ShouldNot(gomega.BeNil())
					gomega.Expect(pools[0].SessionPersistence.Type).Should(gomega.Equal(sessionType))
					// cookie_name is only valid when type is APP_COOKIE; in other cases it must be nil.
					if sessionType != "APP_COOKIE" {
						gomega.Expect(pools[0].SessionPersistence.CookieName).Should(gomega.BeNil())
					}
				}
			})
		},
		ginkgo.Entry("shared ELB with SOURCE_IP", "shared", "SOURCE_IP"),
		ginkgo.Entry("shared ELB with HTTP_COOKIE", "shared", "HTTP_COOKIE"),
		ginkgo.Entry("dedicated ELB with HTTP_COOKIE", "dedicated", "HTTP_COOKIE"),
		ginkgo.Entry("dedicated ELB with APP_COOKIE", "dedicated", "APP_COOKIE"),
	)

	ginkgo.It("dedicated ELB with APP_COOKIE should have cookie_name set", func() {
		serviceName := serviceNamePrefix + rand.String(RandomStrLength)
		cookieName := "mycookie"

		annotations := dedicatedHTTPAnnotations("ROUND_ROBIN")
		annotations[huaweicloud.ElbSessionAffinityOption] =
			fmt.Sprintf(`{"type":"APP_COOKIE","cookie_name":"%s"}`, cookieName)

		service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
		frameworkCreateService(service)

		ingress := waitIngressIP(serviceName)
		verifyHTTPAvailable(ingress)

		ginkgo.By("Verifying APP_COOKIE cookie_name is set on dedicated ELB", func() {
			elbID := findDedicatedELBID(serviceName)
			gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

			pools := clients.ListDedicatedPools(authOpts, elbID)
			gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
			gomega.Expect(pools[0].SessionPersistence).ShouldNot(gomega.BeNil())
			gomega.Expect(pools[0].SessionPersistence.Type).Should(gomega.Equal("APP_COOKIE"))
			gomega.Expect(pools[0].SessionPersistence.CookieName).ShouldNot(gomega.BeNil())
			gomega.Expect(*pools[0].SessionPersistence.CookieName).Should(gomega.Equal(cookieName))
		})
	})
})
