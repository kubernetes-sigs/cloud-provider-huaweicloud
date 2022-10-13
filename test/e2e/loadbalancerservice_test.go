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

var _ = ginkgo.Describe("loadbalancer service testing", func() {
	var deployment *appsv1.Deployment
	var service *corev1.Service
	var secret *corev1.Secret

	ginkgo.BeforeEach(func() {
		deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		deployment = helper2.NewDeployment(testNamespace, deploymentName)
		framework.CreateDeployment(kubeClient, deployment)

		// todo(chengxiangdong): This is a temporary solution, I will optimize it later.
		secret = newSecret(testNamespace)
		framework.CreateSecret(kubeClient, secret)
	})

	ginkgo.AfterEach(func() {
		framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		framework.RemoveSecret(kubeClient, secret.Namespace, secret.Name)
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

	ginkgo.It("service enhanced auto testing", func() {
		sessionAffinity := "SOURCE_IP"
		serviceName := serviceNamePrefix + rand.String(RandomStrLength)
		service = newLoadbalancerAutoService(testNamespace, serviceName, sessionAffinity)
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

// newLoadbalancerAutoService new a loadbalancer type service
func newLoadbalancerAutoService(namespace, name, sessionAffinity string) *corev1.Service {
	autoCreate := fmt.Sprintf(`{"type":"public","bandwidth_name":"cce-bandwidth-%s","bandwidth_chargemode":"bandwidth","bandwidth_size":5,"bandwidth_sharetype":"PER","eip_type":"5_bgp","name":"cce-eip-%s"}`,
		rand.String(RandomStrLength), rand.String(RandomStrLength))

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Annotations: map[string]string{
				"kubernetes.io/elb.class":             "union",
				"kubernetes.io/session-affinity-mode": sessionAffinity,
				"kubernetes.io/elb.autocreate":        autoCreate,
			},
			Labels: map[string]string{"app": "nginx"},
		},
		Spec: corev1.ServiceSpec{
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
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

func newSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "example.secret",
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"access": os.Getenv("HC_ACCESS_KEY"),
			"secret": os.Getenv("HC_SECRET_KEY"),
		},
	}
}
