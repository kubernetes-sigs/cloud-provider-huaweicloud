/*
Copyright 2022 The Kubernetes Authors.
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
	"os"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"

	"sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/framework"
	helper2 "sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/helper"
)

var _ = ginkgo.Describe("DNAT loadbalancer service testing", func() {
	natID := os.Getenv("HC_NAT_ID")
	natIP := os.Getenv("HC_NAT_IP")
	var deployment *appsv1.Deployment
	var service *corev1.Service

	ginkgo.BeforeEach(func() {
		deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		deployment = helper2.NewDeployment(testNamespace, deploymentName)
		framework.CreateDeployment(kubeClient, deployment)
	})

	ginkgo.AfterEach(func() {
		framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		if service != nil {
			framework.RemoveService(kubeClient, service.Namespace, service.Name)
			ginkgo.By(fmt.Sprintf("Wait for the Service(%s/%s) to be deleted", testNamespace, service.Name), func() {
				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					_, err := kubeClient.CoreV1().Services(testNamespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
					if apierrors.IsNotFound(err) {
						return true, nil
					}
					if err != nil {
						return false, err
					}
					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		}
	})

	ginkgo.It("DNAT loadbalancer service testing", func() {
		if len(natID) == 0 || len(natIP) == 0 {
			ginkgo.Skip("not found HC_NAT_ID or HC_NAT_IP env, skip testing DNAT LoadBalance service")
			return
		}

		serviceName := serviceNamePrefix + rand.String(RandomStrLength)
		service = newDnatLoadbalancerService(testNamespace, serviceName, natID, natIP)
		framework.CreateService(kubeClient, service)

		var ingress string
		ginkgo.By("Check service status", func() {
			gomega.Eventually(func(g gomega.Gomega) bool {
				svc, err := kubeClient.CoreV1().Services(testNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
				g.Expect(err).ShouldNot(gomega.HaveOccurred())

				if len(svc.Status.LoadBalancer.Ingress) > 0 {
					ingress = svc.Status.LoadBalancer.Ingress[0].IP
					g.Expect(ingress).ShouldNot(gomega.BeEmpty())
					return true
				}

				return false
			}, pollTimeout, pollInterval).Should(gomega.Equal(true))
		})

		ginkgo.By("Check if ELB listener is available", func() {
			url := fmt.Sprintf("http://%s", ingress)
			statusCode, err := helper2.DoRequest(url)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			gomega.Expect(statusCode).Should(gomega.Equal(200))
		})
	})
})

// newDnatLoadbalancerService new a DNAT loadbalancer service
func newDnatLoadbalancerService(namespace, name, natID, natIP string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Annotations: map[string]string{
				"kubernetes.io/elb.class":     "dnat",
				"kubernetes.io/natgateway.id": natID,
			},
			Labels: map[string]string{"app": "nginx"},
		},
		Spec: corev1.ServiceSpec{
			LoadBalancerIP:        natIP,
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeCluster,
			Type:                  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.IntOrString{IntVal: 80},
				},
			},
			Selector: map[string]string{"app": "nginx"},
		},
	}
}
