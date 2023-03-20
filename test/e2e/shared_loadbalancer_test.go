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

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/wrapper"
	"sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/clients"
	"sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/framework"
	helper2 "sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/helper"
)

var _ = ginkgo.Describe("Shared loadbalancer(TCP) service testing", func() {
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

	ginkgo.It("Auto create service testing", func() {
		serviceName := serviceNamePrefix + rand.String(RandomStrLength)

		annotations := map[string]string{}
		annotations[huaweicloud.ElbClass] = "shared"
		annotations[huaweicloud.ElbAlgorithm] = "ROUND_ROBIN"
		annotations[huaweicloud.AutoCreateEipOptions] = `{"ip_type": "5_bgp", "bandwidth_size": 5, "share_type": "PER", "charge_mode": "bandwidth"}`
		annotations[huaweicloud.ElbSessionAffinityFlag] = "on"
		annotations[huaweicloud.ElbSessionAffinityOption] = `{"type":"SOURCE_IP", "persistence_timeout": 3}`
		annotations[huaweicloud.ElbHealthCheckFlag] = "on"
		annotations[huaweicloud.ElbHealthCheckOptions] = `{"delay": 3, "timeout": 15, "max_retries": 3}`

		service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
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

var _ = ginkgo.Describe("Shared loadbalancer(HTTP) service testing", func() {
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

	ginkgo.It("Auto create service testing", func() {
		serviceName := serviceNamePrefix + rand.String(RandomStrLength)

		annotations := map[string]string{}
		annotations[huaweicloud.ElbClass] = "shared"
		annotations[huaweicloud.ElbAlgorithm] = "ROUND_ROBIN"
		annotations[huaweicloud.ElbSessionAffinityFlag] = "on"
		annotations[huaweicloud.ElbSessionAffinityOption] = `{"type":"SOURCE_IP", "persistence_timeout": 3}`
		annotations[huaweicloud.ElbHealthCheckFlag] = "on"
		annotations[huaweicloud.ElbHealthCheckOptions] = `{"delay": 3, "timeout": 15, "max_retries": 3}`
		annotations[huaweicloud.ElbXForwardedHost] = "true"
		annotations[huaweicloud.ElbIdleTimeout] = "120"
		annotations[huaweicloud.ElbRequestTimeout] = "120"
		annotations[huaweicloud.ElbResponseTimeout] = "120"

		service = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
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

var _ = ginkgo.Describe("Use an existing shared load balancer (TCP) service testing", func() {
	var deployment *appsv1.Deployment
	var service1 *corev1.Service
	var service2 *corev1.Service
	var elbID *string
	var eipID *string

	ginkgo.BeforeEach(func() {
		if vpcOpts.SubnetID == "" {
			return
		}
		name := fmt.Sprintf("e2e_test_%s", rand.String(RandomStrLength))
		instanceID := clients.CreateSharedELBInstance(authOpts, vpcOpts.SubnetID, name)
		elbID = &instanceID
		eip := clients.CreateEip(authOpts)
		eipID = eip.Id

		deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
		deployment = helper2.NewDeployment(testNamespace, deploymentName)
		framework.CreateDeployment(kubeClient, deployment)

	})

	ginkgo.AfterEach(func() {
		if vpcOpts.SubnetID == "" {
			return
		}
		framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
		if service1 != nil {
			framework.RemoveService(kubeClient, service1.Namespace, service1.Name)
			framework.WaitServiceDisappearOnCluster(kubeClient, service1.Namespace, service1.Name)
		}
		if service2 != nil {
			framework.RemoveService(kubeClient, service2.Namespace, service2.Name)
			framework.WaitServiceDisappearOnCluster(kubeClient, service2.Namespace, service2.Name)
		}

		sharedElbClient := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
		_, err := sharedElbClient.GetInstance(*elbID)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		clients.DeleteSharedELBInstance(authOpts, *elbID)

		eipClient := wrapper.EIpClient{AuthOpts: authOpts}
		_, err = eipClient.Get(*eipID)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		clients.DeleteEip(authOpts, *eipID)
	})

	ginkgo.It("Create multiple listeners", func() {
		if vpcOpts.SubnetID == "" {
			ginkgo.Skip("not found HC_SUBNET_ID env, skip testing")
			return
		}

		serviceName := serviceNamePrefix + rand.String(RandomStrLength)
		annotations := map[string]string{}
		annotations[huaweicloud.ElbClass] = "shared"
		annotations[huaweicloud.ElbAlgorithm] = "ROUND_ROBIN"
		annotations[huaweicloud.ElbSessionAffinityFlag] = "on"
		annotations[huaweicloud.ElbSessionAffinityOption] = `{"type":"SOURCE_IP", "persistence_timeout": 5}`
		annotations[huaweicloud.ElbHealthCheckOptions] = `{"delay": 3, "timeout": 15, "max_retries": 3}`
		annotations[huaweicloud.ElbXForwardedHost] = "true"
		annotations[huaweicloud.ElbID] = *elbID
		annotations[huaweicloud.ElbEipID] = *eipID
		annotations[huaweicloud.ELBKeepEip] = "true"

		service1 = newLoadbalancerAutoService(testNamespace, serviceName, 80, annotations)
		framework.CreateService(kubeClient, service1)
		serviceName2 := serviceNamePrefix + rand.String(RandomStrLength)
		service2 = newLoadbalancerAutoService(testNamespace, serviceName2, 82, annotations)
		framework.CreateService(kubeClient, service2)

		var ingress string
		ginkgo.By("Check service1 status", func() {
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
func newLoadbalancerAutoService(namespace, name string, port int32, annotations map[string]string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: annotations,
			Labels:      map[string]string{"app": "nginx"},
		},
		Spec: corev1.ServiceSpec{
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeCluster,
			Type:                  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       port,
					TargetPort: intstr.IntOrString{IntVal: 80},
				},
			},
			Selector: map[string]string{"app": "nginx"},
		},
	}
}
