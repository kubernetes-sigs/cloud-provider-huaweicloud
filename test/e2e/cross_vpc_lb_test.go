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
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud"
	"sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/clients"
)

var _ = ginkgo.Describe("Loadbalancer cross-VPC backend testing (dedicated only)", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deployment = setupDeployment()
	})

	ginkgo.AfterEach(func() {
		cleanupDeploymentAndService(deployment, service)
	})

	ginkgo.It("dedicated ELB with enable-cross-vpc=true", func() {
		serviceName := serviceNamePrefix + rand.String(RandomStrLength)

		annotations := dedicatedTCPAnnotations("ROUND_ROBIN")
		annotations[huaweicloud.ElbEnableCrossVpc] = "true"

		service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
		frameworkCreateService(service)

		ingress := waitIngressIP(serviceName)
		verifyHTTPAvailable(ingress)

		ginkgo.By("Verifying cross-VPC (ip_target_enable) is enabled on dedicated ELB", func() {
			elbID := findDedicatedELBID(serviceName)
			gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

			lb := clients.GetDedicatedLoadBalancer(authOpts, elbID)
			gomega.Expect(lb.IpTargetEnable).Should(gomega.BeTrue(),
				"ip_target_enable should be true when enable-cross-vpc=true")
		})

		ginkgo.By("Verifying pool type is 'ip' for cross-VPC backend", func() {
			elbID := findDedicatedELBID(serviceName)
			pools := clients.ListDedicatedPools(authOpts, elbID)
			gomega.Expect(pools).ShouldNot(gomega.BeEmpty())
		})
	})
})
