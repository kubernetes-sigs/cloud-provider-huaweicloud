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
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud"
	"sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/clients"
	helper2 "sigs.k8s.io/cloud-provider-huaweicloud/test/e2e/helper"
)

// ---------- Service builders ----------

func newUDPLoadbalancerService(namespace, name string, port int32, annotations map[string]string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: annotations,
			Labels:      map[string]string{"app": "nginx"},
		},
		Spec: corev1.ServiceSpec{
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeCluster,
			Type:                  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{
				Name:       "udp",
				Protocol:   corev1.ProtocolUDP,
				Port:       port,
				TargetPort: intstr.IntOrString{IntVal: 80},
			}},
			Selector: map[string]string{"app": "nginx"},
		},
	}
}

func newMultiPortLoadbalancerService(namespace, name string, ports []corev1.ServicePort, annotations map[string]string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: annotations,
			Labels:      map[string]string{"app": "nginx"},
		},
		Spec: corev1.ServiceSpec{
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeCluster,
			Type:                  corev1.ServiceTypeLoadBalancer,
			Ports:                 ports,
			Selector:              map[string]string{"app": "nginx"},
		},
	}
}

func newLocalTrafficLoadbalancerService(namespace, name string, port int32, annotations map[string]string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Annotations: annotations,
			Labels:      map[string]string{"app": "nginx"},
		},
		Spec: corev1.ServiceSpec{
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			Type:                  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Protocol:   corev1.ProtocolTCP,
				Port:       port,
				TargetPort: intstr.IntOrString{IntVal: 80},
			}},
			Selector: map[string]string{"app": "nginx"},
		},
	}
}

// ---------- Deployment helpers ----------

func scaleDeployment(namespace, name string, replicas int32) {
	ginkgo.By(fmt.Sprintf("Scaling deployment %s/%s to %d replicas", namespace, name, replicas), func() {
		deploy, err := kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		deploy.Spec.Replicas = pointer.Int32(replicas)
		_, err = kubeClient.AppsV1().Deployments(namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

		gomega.Eventually(func(g gomega.Gomega) bool {
			d, err := kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
			g.Expect(err).ShouldNot(gomega.HaveOccurred())
			return d.Status.ReadyReplicas == replicas
		}, pollTimeout, pollInterval).Should(gomega.Equal(true))
	})
}

func newDeploymentWithReplicas(namespace, name string, replicas int32) *appsv1.Deployment {
	deploy := helper2.NewDeployment(namespace, name)
	deploy.Spec.Replicas = pointer.Int32(replicas)
	return deploy
}

// ---------- Service update helpers ----------

func updateServiceAnnotations(namespace, name string, annotations map[string]string) {
	ginkgo.By(fmt.Sprintf("Updating annotations on Service %s/%s", namespace, name), func() {
		svc, err := kubeClient.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		svc.Annotations = annotations
		_, err = kubeClient.CoreV1().Services(namespace).Update(context.TODO(), svc, metav1.UpdateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// ---------- Ingress / cleanup helpers ----------

func waitIngressIP(serviceName string) string {
	var ingress string
	ginkgo.By(fmt.Sprintf("Waiting for ingress IP on Service %s", serviceName), func() {
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
	return ingress
}

func waitServiceDeletion(namespace, name string) {
	ginkgo.By(fmt.Sprintf("Waiting for Service %s/%s to be deleted", namespace, name), func() {
		gomega.Eventually(func(g gomega.Gomega) (bool, error) {
			_, err := kubeClient.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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

// ---------- ELB ID helpers ----------

func findSharedELBID(serviceName string) string {
	return clients.FindSharedELBID(authOpts, serviceName)
}

func findDedicatedELBID(serviceName string) string {
	return clients.FindDedicatedELBID(authOpts, serviceName)
}

// findELBID determines ELB class from annotations and returns the ELB ID
func findELBID(serviceName string, annotations map[string]string) string {
	class := annotations[huaweicloud.ElbClass]
	switch class {
	case "dedicated":
		return findDedicatedELBID(serviceName)
	default:
		return findSharedELBID(serviceName)
	}
}

// ---------- Standard BeforeEach / AfterEach for new test files ----------

func setupDeployment() *appsv1.Deployment {
	deploymentName := deploymentNamePrefix + rand.String(RandomStrLength)
	deployment := helper2.NewDeployment(testNamespace, deploymentName)
	frameworkCreateDeployment(deployment)
	return deployment
}

func frameworkCreateDeployment(deployment *appsv1.Deployment) {
	ginkgo.By(fmt.Sprintf("Creating Deployment(%s/%s)", deployment.Namespace, deployment.Name), func() {
		_, err := kubeClient.AppsV1().Deployments(deployment.Namespace).Create(
			context.TODO(), deployment, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

func cleanupDeploymentAndService(deployment *appsv1.Deployment, service *corev1.Service) {
	if deployment != nil {
		frameworkRemoveDeployment(deployment.Namespace, deployment.Name)
	}
	if service != nil {
		frameworkRemoveService(service.Namespace, service.Name)
		waitServiceDeletion(service.Namespace, service.Name)
	}
}

func frameworkRemoveDeployment(namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing Deployment(%s/%s)", namespace, name), func() {
		err := kubeClient.AppsV1().Deployments(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

func frameworkRemoveService(namespace, name string) {
	ginkgo.By(fmt.Sprintf("Removing Service(%s/%s)", namespace, name), func() {
		err := kubeClient.CoreV1().Services(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		}
	})
}

func frameworkCreateService(service *corev1.Service) {
	ginkgo.By(fmt.Sprintf("Creating Service(%s/%s)", service.Namespace, service.Name), func() {
		_, err := kubeClient.CoreV1().Services(service.Namespace).Create(
			context.TODO(), service, metav1.CreateOptions{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	})
}

// ---------- Annotation builders ----------

func sharedTCPAnnotations(algorithm string) map[string]string {
	annotations := map[string]string{
		huaweicloud.ElbClass:                 "shared",
		huaweicloud.ElbAlgorithm:             algorithm,
		huaweicloud.ElbSessionAffinityFlag:   "on",
		huaweicloud.ElbSessionAffinityOption: `{"type":"SOURCE_IP", "persistence_timeout": 3}`,
		huaweicloud.ElbHealthCheckFlag:       "on",
		huaweicloud.ElbHealthCheckOptions:    `{"delay": 3, "timeout": 15, "max_retries": 3}`,
		huaweicloud.AutoCreateEipOptions:     `{"ip_type": "5_bgp", "bandwidth_size": 5, "share_type": "PER", "charge_mode": "bandwidth"}`,
	}
	return annotations
}

func sharedHTTPAnnotations(algorithm string) map[string]string {
	annotations := map[string]string{
		huaweicloud.ElbClass:                 "shared",
		huaweicloud.ElbAlgorithm:             algorithm,
		huaweicloud.ElbSessionAffinityFlag:   "on",
		huaweicloud.ElbSessionAffinityOption: `{"type":"SOURCE_IP", "persistence_timeout": 3}`,
		huaweicloud.ElbHealthCheckFlag:       "on",
		huaweicloud.ElbHealthCheckOptions:    `{"delay": 3, "timeout": 15, "max_retries": 3}`,
		huaweicloud.ElbXForwardedHost:        "true",
		huaweicloud.AutoCreateEipOptions:     `{"ip_type": "5_bgp", "bandwidth_size": 5, "share_type": "PER", "charge_mode": "bandwidth"}`,
	}
	return annotations
}

func dedicatedTCPAnnotations(algorithm string) map[string]string {
	annotations := map[string]string{
		huaweicloud.ElbClass:                 "dedicated",
		huaweicloud.ElbAvailabilityZones:     getAZ(),
		huaweicloud.ElbAlgorithm:             algorithm,
		huaweicloud.ElbSessionAffinityFlag:   "on",
		huaweicloud.ElbSessionAffinityOption: `{"type":"SOURCE_IP", "persistence_timeout": 20}`,
		huaweicloud.ElbHealthCheckFlag:       "on",
		huaweicloud.ElbHealthCheckOptions:    `{"delay": 4, "timeout": 16, "max_retries": 4}`,
		huaweicloud.ElbIdleTimeout:           "15",
		huaweicloud.AutoCreateEipOptions:     `{"ip_type": "5_bgp", "bandwidth_size": 5, "share_type": "PER", "charge_mode": "bandwidth"}`,
	}
	return annotations
}

func dedicatedHTTPAnnotations(algorithm string) map[string]string {
	annotations := map[string]string{
		huaweicloud.ElbClass:                 "dedicated",
		huaweicloud.ElbAvailabilityZones:     getAZ(),
		huaweicloud.ElbAlgorithm:             algorithm,
		huaweicloud.ElbSessionAffinityFlag:   "on",
		huaweicloud.ElbSessionAffinityOption: `{"type":"HTTP_COOKIE", "persistence_timeout": 20}`,
		huaweicloud.ElbHealthCheckFlag:       "on",
		huaweicloud.ElbHealthCheckOptions:    `{"delay": 4, "timeout": 16, "max_retries": 4}`,
		huaweicloud.ElbXForwardedHost:        "true",
		huaweicloud.ElbIdleTimeout:           "290",
		huaweicloud.ElbRequestTimeout:        "290",
		huaweicloud.ElbResponseTimeout:       "290",
		huaweicloud.AutoCreateEipOptions:     `{"ip_type": "5_bgp", "bandwidth_size": 5, "share_type": "PER", "charge_mode": "bandwidth"}`,
	}
	return annotations
}

func verifyHTTPAvailable(ingress string) {
	url := fmt.Sprintf("http://%s", ingress)
	gomega.Eventually(func(g gomega.Gomega) {
		statusCode, err := helper2.DoRequest(url)
		g.Expect(err).ShouldNot(gomega.HaveOccurred())
		g.Expect(statusCode).Should(gomega.Equal(200))
	}, pollTimeout, pollInterval).Should(gomega.Succeed())
}
