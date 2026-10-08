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

var _ = ginkgo.Describe("Loadbalancer lb-algorithm testing", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deployment = setupDeployment()
	})

	ginkgo.AfterEach(func() {
		cleanupDeploymentAndService(deployment, service)
	})

	ginkgo.DescribeTable("verify lb-algorithm via ELB API",
		func(elbClass, algorithm string) {
			serviceName := serviceNamePrefix + rand.String(RandomStrLength)

			var annotations map[string]string
			if elbClass == "shared" {
				annotations = sharedTCPAnnotations(algorithm)
			} else {
				annotations = dedicatedTCPAnnotations(algorithm)
			}

			service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
			frameworkCreateService(service)

			ingress := waitIngressIP(serviceName)
			verifyHTTPAvailable(ingress)

			ginkgo.By(fmt.Sprintf("Verifying pool lb_algorithm is %s on %s ELB", algorithm, elbClass), func() {
				elbID := findELBID(serviceName, annotations)
				gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

				var poolAlgorithm string
				if elbClass == "shared" {
					pools := clients.ListSharedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					poolAlgorithm = pools[0].LbAlgorithm.Value()
				} else {
					pools := clients.ListDedicatedPools(authOpts, elbID)
					gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
					poolAlgorithm = pools[0].LbAlgorithm
				}
				gomega.Expect(poolAlgorithm).Should(gomega.Equal(algorithm))
			})
		},
		ginkgo.Entry("shared ELB with LEAST_CONNECTIONS", "shared", "LEAST_CONNECTIONS"),
		ginkgo.Entry("shared ELB with SOURCE_IP", "shared", "SOURCE_IP"),
		ginkgo.Entry("dedicated ELB with LEAST_CONNECTIONS", "dedicated", "LEAST_CONNECTIONS"),
		ginkgo.Entry("dedicated ELB with SOURCE_IP", "dedicated", "SOURCE_IP"),
	)
})
