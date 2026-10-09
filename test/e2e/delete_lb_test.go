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

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/wrapper"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/common"
)

var _ = ginkgo.Describe("Loadbalancer delete verification testing", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deployment = setupDeployment()
	})

	ginkgo.AfterEach(func() {
		if service != nil {
			frameworkRemoveService(service.Namespace, service.Name)
			waitServiceDeletion(service.Namespace, service.Name)
		}
		if deployment != nil {
			frameworkRemoveDeployment(deployment.Namespace, deployment.Name)
		}
	})

	ginkgo.DescribeTable("verify ELB instance is cleaned up after service deletion",
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

			elbID := findELBID(serviceName, annotations)
			gomega.Expect(elbID).ShouldNot(gomega.BeEmpty())

			ginkgo.By(fmt.Sprintf("Deleting service and waiting for cleanup on %s ELB", elbClass), func() {
				frameworkRemoveService(service.Namespace, service.Name)
				waitServiceDeletion(service.Namespace, service.Name)
			})

			ginkgo.By("Verifying ELB instance is deleted from cloud", func() {
				gomega.Eventually(func(g gomega.Gomega) bool {
					if elbClass == "shared" {
						sharedClient := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
						_, err := sharedClient.GetInstance(elbID)
						return common.IsNotFound(err)
					}
					dedicatedClient := wrapper.DedicatedLoadBalanceClient{AuthOpts: authOpts}
					_, err := dedicatedClient.GetInstance(elbID)
					return common.IsNotFound(err)
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})

			service = nil
		},
		ginkgo.Entry("shared ELB auto-create deleted", "shared"),
		ginkgo.Entry("dedicated ELB auto-create deleted", "dedicated"),
	)
})
