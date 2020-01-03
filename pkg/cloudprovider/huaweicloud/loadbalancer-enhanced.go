/*
Copyright 2020 The Kubernetes Authors.

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

package huaweicloud

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/golang-lru"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

type ALBCloud struct {
	config        *LoadBalancerOpts
	kubeClient    corev1.CoreV1Interface
	lrucache      *lru.Cache
	secret        *Secret
	eventRecorder record.EventRecorder
}

type tempALBServicePort struct {
	servicePort *v1.ServicePort
	listener    ALBListener
}

/*
 *    >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
 *    ALB implement of functions in cloud.go, including
 *               GetLoadBalancer()
 *               GetLoadBalancerName()
 *               EnsureLoadBalancer()
 *               UpdateLoadBalancer()
 *               EnsureLoadBalancerDeleted()
 *    >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
 */
func (alb *ALBCloud) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	status = &v1.LoadBalancerStatus{}
	albProvider, err := alb.getALBClient(service.Namespace)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}

		return nil, false, err
	}

	_, loadBalancerId, err := alb.getAlbInstanceInfo(service, albProvider, true)
	if err != nil {
		return nil, false, err
	}

	listeners, err := albProvider.findListenerOfService(loadBalancerId, service)
	if err != nil {
		return nil, false, err
	}
	if len(listeners) == 0 {
		return nil, false, nil
	}

	status.Ingress = append(status.Ingress, v1.LoadBalancerIngress{IP: service.Spec.LoadBalancerIP})
	return status, true, nil
}

/*
 *    Not implemented
 */
func (alb *ALBCloud) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	return ""
}

/*
 *    clusterName: discarded
 *    service: each service has its corresponding load balancer
 *    nodes: all nodes under ServiceController, i.e. all nodes under the k8s cluster
 */
func (alb *ALBCloud) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, hosts []*v1.Node) (*v1.LoadBalancerStatus, error) {
	klog.Infof("Begin to ensure loadbalancer configuration of service(%s/%s)", service.Namespace, service.Name)
	albProvider, err := alb.getALBClient(service.Namespace)
	if err != nil {
		return nil, err
	}

	_, loadBalancerId, err := alb.getAlbInstanceInfo(service, albProvider, true)
	if err != nil {
		return nil, err
	}

	listeners, err := albProvider.findListenerOfService(loadBalancerId, service)
	if err != nil {
		return nil, err
	}

	members, err := alb.generateMembers(service, albProvider)
	if err != nil {
		return nil, err
	}

	needsCreate, needsUpdate, needsDelete := alb.compare(service, listeners)
	ch := make(chan error, 3)

	go func() {
		ch <- alb.createLoadBalancer(albProvider, service, needsCreate, members)
	}()

	go func() {
		ch <- alb.updateLoadBalancer(albProvider, service, needsUpdate, members)
	}()

	go func() {
		ch <- alb.deleteLoadBalancer(albProvider, service, needsDelete)
	}()

	var errs []error
	for i := 3; i > 0; i-- {
		select {
		case err := <-ch:
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) != 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	status := &v1.LoadBalancerStatus{}
	status.Ingress = append(status.Ingress, v1.LoadBalancerIngress{IP: service.Spec.LoadBalancerIP})
	return status, nil
}

// update members in the service
//        (1) find out new members for the service according to service pods and the health status
//        (2) find out the pool
//        (3) get previous members under the pool
//        (4) compare the equality of two member sets, if not equal, update the pool members
func (alb *ALBCloud) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.Infof("Begin to update loadbalancer configuration of service(%s/%s)", service.Namespace, service.Name)
	albProvider, err := alb.getALBClient(service.Namespace)
	if err != nil {
		return err
	}

	members, err := alb.generateMembers(service, albProvider)
	if err != nil {
		return err
	}

	_, loadBalancerId, err := alb.getAlbInstanceInfo(service, albProvider, true)
	if err != nil {
		return err
	}

	listeners, err := albProvider.findListenerOfService(loadBalancerId, service)
	if err != nil {
		return err
	}
	sessionAffinity := GetSessionAffinity(service)
	var errs []error
	for _, listener := range listeners {
		pool, err := albProvider.findPoolOfListener(listener.Id)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Get pool of listener(%s) error: %v", listener.Id, err)
			alb.sendEvent("UpdateLoadBalancerFailed", msg, service)
			continue
		}
		if pool == nil {
			poolConf := ALBPool{
				LbAlgorithm:  ALBAlgorithmRR,
				Protocol:     listener.Protocol,
				ListenerId:   listener.Id,
				AdminStateUp: true,
			}
			if sessionAffinity {
				poolConf.SessionPersistence.Type = ALBSessionSource
			}
			pool, err = albProvider.CreatePool(&poolConf)
			if err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Create pool of listener(%s) error: %v", listener.Id, err)
				alb.sendEvent("UpdateLoadBalancerFailed", msg, service)
				continue
			}
		}

		var curPort int32
		for _, port := range service.Spec.Ports {
			if port.Port == listener.ProtocolPort {
				curPort = port.NodePort
				break
			}
		}
		if curPort == 0 {
			msg := fmt.Sprintf("Get backend port of pool(%s) error", pool.Id)
			alb.sendEvent("UpdateLoadBalancerFailed", msg, service)
			err = fmt.Errorf("%s", msg)
			errs = append(errs, err)
			continue
		}
		curMembers := make(map[string]*ALBMember)
		for _, member := range members {
			member.ProtocolPort = curPort
			curMembers[member.Address] = member
		}

		preListMembers, err := albProvider.ListMembers(pool.Id)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Get member of listener/pool(%s/%s) error: %v", listener.Id, pool.Id, err)
			alb.sendEvent("UpdateLoadBalancerFailed", msg, service)
			continue
		}
		preMembers := make(map[string]ALBMember)
		for _, member := range preListMembers.Members {
			preMembers[member.Address] = member
		}

		if err = alb.updateALbMembers(*albProvider, curMembers, preMembers, pool.Id); err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Update member of listener/pool(%s/%s) error: %v", listener.Id, pool.Id, err)
			alb.sendEvent("UpdateLoadBalancerFailed", msg, service)
		}
	}
	klog.Infof("Update loadbalancer of service(%s/%s) finish, total: %d, failed: %d",
		service.Namespace, service.Name, len(listeners), len(errs))
	return utilerrors.NewAggregate(errs)
}

// delete all resources under a service, including listener, pool, healthMonitor and member
//        (1) find out the listener of the service
//        (2) find out the pool of the listener
//        (3) get all members & healthMonitor under this pool
//        (4) delete these members & healthMonitors in the pool
//        (5) delete the pool & listener
func (alb *ALBCloud) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.Infof("Begin to delete loadbalancer configuration of service(%s/%s)", service.Namespace, service.Name)
	albProvider, err := alb.getALBClient(service.Namespace)
	if err != nil {
		return err
	}

	_, loadBalancerId, err := alb.getAlbInstanceInfo(service, albProvider, true)
	if err != nil {
		return err
	}

	listeners, err := albProvider.findListenerOfService(loadBalancerId, service)
	if err != nil {
		return err
	}
	if len(listeners) == 0 {
		return nil
	}

	var errs []error
	for _, listener := range listeners {
		pool, err := albProvider.findPoolOfListener(listener.Id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var poolID, healthMonitorID string
		if pool != nil {
			poolID = pool.Id
			healthMonitorID = pool.HealthMonitorId
		}
		if err = alb.deleteListener(albProvider, listener.Id, poolID, healthMonitorID); err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Delete listener(%s) error: %v", listener.Id, err)
			alb.sendEvent("DeleteLoadBalancerFailed", msg, service)
		}
	}
	klog.Infof("Delete listeners of service(%s/%s) finish, total: %d, failed: %d",
		service.Namespace, service.Name, len(listeners), len(errs))

	return utilerrors.NewAggregate(errs)
}

func (alb *ALBCloud) sendEvent(title, msg string, service *v1.Service) {
	klog.Errorf("[%s/%s]%s", service.Namespace, service.Name, msg)
	alb.eventRecorder.Event(service, v1.EventTypeWarning, title, fmt.Sprintf("Details: %s", msg))
}

func (alb *ALBCloud) deleteListener(albProvider *ALBClient, listenerID, poolID, healthMonitorId string) error {
	if poolID != "" {
		err := albProvider.DeleteMembers(poolID)
		if err != nil {
			return fmt.Errorf("Delete members of pool(%s) error: %v", poolID, err)
		}
	}

	if healthMonitorId != "" {
		err := albProvider.DeleteHealthMonitor(healthMonitorId)
		if err != nil {
			return fmt.Errorf("Delete healthMonitor of pool(%s) error: %v", poolID, err)
		}
	}

	if poolID != "" {
		err := albProvider.DeletePool(poolID)
		if err != nil {
			return fmt.Errorf("Delete pool(%s) error: %v", poolID, err)
		}
	}

	if listenerID != "" {
		err := albProvider.DeleteListener(listenerID)
		if err != nil {
			return fmt.Errorf("Delete listener(%s) error: %v", listenerID, err)
		}
	}

	return nil
}

/*
 *    >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
 *               Util function
 *    >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
 */
func (alb *ALBCloud) getALBClient(namespace string) (*ALBClient, error) {
	secret, err := alb.getSecret(namespace, alb.config.SecretName)
	if err != nil {
		return nil, err
	}

	return NewALBClient(
		alb.config.ALBEndpoint,
		alb.config.TenantId,
		secret.Data.AccessKey,
		secret.Data.SecretKey,
		alb.config.Region,
		alb.config.SignerType,
	), nil
}

func (alb *ALBCloud) getSecret(namespace, secretName string) (*Secret, error) {
	var kubeSecret *v1.Secret

	key := namespace + "/" + secretName
	obj, ok := alb.lrucache.Get(key)
	if ok {
		kubeSecret = obj.(*v1.Secret)
	} else {
		secret, err := alb.kubeClient.Secrets(namespace).Get(secretName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		kubeSecret = secret
	}

	bytes, err := json.Marshal(kubeSecret)
	if err != nil {
		return nil, err
	}

	var secret *Secret
	err = json.Unmarshal(bytes, &secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func (alb *ALBCloud) getPods(name, namespace string) (*v1.PodList, error) {
	service, err := alb.kubeClient.Services(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if len(service.Spec.Selector) == 0 {
		return nil, fmt.Errorf("The service %s has no selector to associate the pods.", name)
	}

	set := labels.Set{}
	set = service.Spec.Selector
	labelSelector := set.AsSelector()
	opts := metav1.ListOptions{LabelSelector: labelSelector.String()}
	return alb.kubeClient.Pods(namespace).List(opts)
}

// make sure the listener is created under specific load balancer, if not, let's create one
// albProvider: alb client
// name: listener name
// albAlgorithm: load balance algorithm
// port: the port in "EIP:port"
// lbIp: load balancer IP, i.e. EIP/VipAddress
func (alb *ALBCloud) ensureCreateListener(albProvider ALBClient, name string, port v1.ServicePort, albId string) (string, error) {
	listenerConf := &ALBListener{
		Name:           name,
		AdminStateUp:   true,
		Protocol:       ALBProtocol(port.Protocol),
		ProtocolPort:   port.Port,
		LoadbalancerId: albId,
		Description:    ListenerDescription,
	}

	//check if the port in use, or has exist
	params := map[string]string{"loadbalancer_id": albId}
	listenerList, err := albProvider.ListListeners(params)
	if err != nil {
		return "", err
	}
	for _, lsn := range listenerList.Listeners {
		if lsn.ProtocolPort == port.Port {
			klog.Infof("EnsureCreateListener %v: Listener port has exist ", name)
			return "", nil
		}
	}

	listener, err := albProvider.CreateListener(listenerConf)
	if err != nil {
		return "", fmt.Errorf("EnsureCreateListener: Failed to CreateListener : %v", err)
	}
	return listener.Id, nil
}

func (alb *ALBCloud) updateALbMembers(albProvider ALBClient, newMembers map[string]*ALBMember, preMembers map[string]ALBMember, poolId string) error {
	deleteMemberIds := make(map[string]bool)
	for memberAddress, premember := range preMembers {
		if newmember, ok := newMembers[memberAddress]; !ok || premember.ProtocolPort != newmember.ProtocolPort {
			deleteMemberIds[premember.Id] = true
		} else {
			delete(newMembers, memberAddress)
		}
	}
	for memberAddress := range newMembers {
		_, err := albProvider.AddMember(poolId, newMembers[memberAddress])
		if err != nil {
			return err
		}
	}

	for deleteMemberId := range deleteMemberIds {
		err := albProvider.DeleteMember(poolId, deleteMemberId)
		if err != nil {
			return err
		}
	}
	return nil
}

func (alb *ALBCloud) getAlbInstanceInfo(service *v1.Service, albProvider *ALBClient, onlyGetLoadBalanceID bool) (*ALB, string, error) {
	var albInstanceInfo *ALB
	var err error
	loadBalancerId := service.Annotations[ELBIDAnnotation]
	if loadBalancerId != "" {
		if onlyGetLoadBalanceID {
			return nil, loadBalancerId, nil
		}

		albInstanceInfo, err = albProvider.GetLoadBalancer(loadBalancerId)
		if err != nil {
			return nil, "", err
		}
	} else {
		params := map[string]string{"vip_address": service.Spec.LoadBalancerIP}
		albInstances, err := albProvider.ListLoadBalancers(params)
		if err != nil {
			return nil, "", err
		}
		if len(albInstances.Loadbalancers) == 0 {
			return nil, "", fmt.Errorf("There has no LoadBalancer found")
		}
		albInstanceInfo = &albInstances.Loadbalancers[0]
	}
	return albInstanceInfo, albInstanceInfo.Id, nil
}

func (alb *ALBCloud) compare(
	service *v1.Service,
	listeners map[string]ALBListener) ([]v1.ServicePort, map[string]tempALBServicePort, []ALBListener) {
	var needsCreate []v1.ServicePort
	needsUpdate := make(map[string]tempALBServicePort)
	var needsDelete []ALBListener
	for i := range service.Spec.Ports {
		port := service.Spec.Ports[i]
		create := true
		for _, listener := range listeners {
			if port.Port == listener.ProtocolPort {
				create = false
				needsUpdate[listener.Id] = tempALBServicePort{
					servicePort: &port,
					listener:    listener,
				}
				break
			}
		}
		if create {
			needsCreate = append(needsCreate, port)
		}
	}

	for _, listener := range listeners {
		if _, ok := needsUpdate[listener.Id]; ok {
			continue
		}
		needsDelete = append(needsDelete, listener)
	}
	return needsCreate, needsUpdate, needsDelete
}

func (alb *ALBCloud) generateMembers(service *v1.Service, albProvider *ALBClient) ([]*ALBMember, error) {
	podList, err := alb.getPods(service.Name, service.Namespace)
	if err != nil {
		return nil, err
	}

	albInstanceInfo, _, err := alb.getAlbInstanceInfo(service, albProvider, false)
	if err != nil {
		return nil, err
	}

	var members []*ALBMember
	hasNodeExist := map[string]bool{}
	for _, item := range podList.Items {
		if item.Status.HostIP == "" {
			klog.Errorf("pod(%s/%s) has not been scheduled", item.Namespace, item.Name)
			continue
		}

		if hasNodeExist[item.Status.HostIP] {
			continue
		}

		hasNodeExist[item.Status.HostIP] = true
		member := ALBMember{
			Address:      item.Status.HostIP,
			SubnetId:     albInstanceInfo.VipSubnetId,
			AdminStateUp: true,
		}
		members = append(members, &member)
	}

	// TODO: consider the pods have been deleted.
	if len(members) == 0 {
		return nil, fmt.Errorf("Have no node to bind.")
	}

	return members, nil
}

func (alb *ALBCloud) createLoadBalancer(
	albProvider *ALBClient,
	service *v1.Service,
	needsCreate []v1.ServicePort,
	members []*ALBMember) error {
	var (
		errs []error
	)
	_, loadBalancerId, err := alb.getAlbInstanceInfo(service, albProvider, true)
	if err != nil {
		return err
	}
	lsName := GetListenerName(service)
	sessionAffinity := GetSessionAffinity(service)
	for _, port := range needsCreate {
		// Step 1. create listener if needed
		listenerId, err := alb.ensureCreateListener(
			*albProvider,
			lsName,
			port,
			loadBalancerId)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Create listener(port:%d) error: %v", port.Port, err)
			alb.sendEvent("CreateLoadBalancerFailed", msg, service)
			continue
		}

		if listenerId == "" {
			msg := fmt.Sprintf("The listener(port:%d) has already exist", port.Port)
			alb.sendEvent("CreateLoadBalancerFailed", msg, service)
			continue
		}

		klog.Infof("Create listener(%s/%d) of loadbalancer(%s) success.", lsName, port.Port, service.Spec.LoadBalancerIP)

		// Step 2. Create pool
		poolConf := ALBPool{
			LbAlgorithm:  ALBAlgorithmRR,
			Protocol:     ALBProtocol(port.Protocol),
			ListenerId:   listenerId,
			AdminStateUp: true,
		}
		if sessionAffinity {
			poolConf.SessionPersistence.Type = ALBSessionSource
		}
		pool, err := albProvider.CreatePool(&poolConf)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Create pool of listener(%s) error: %v", listenerId, err)
			alb.sendEvent("CreateLoadBalancerFailed", msg, service)
			continue
		}

		klog.Infof("Create pool(%s) of loadbalancer/listener(%s:%s) success.", pool.Id, service.Spec.LoadBalancerIP, listenerId)
		// step 3 create health monitor
		healthMonitorConf := ALBHealthMonitor{
			Type:         ALBProtocol(port.Protocol),
			Delay:        5,
			Timeout:      10,
			MaxRetries:   3,
			PoolId:       pool.Id,
			AdminStateUp: true,
		}

		_, err = albProvider.CreateHealthMonitor(&healthMonitorConf)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Create health Monitor of pool(%s) error: %v", pool.Id, err)
			alb.sendEvent("CreateLoadBalancerFailed", msg, service)
			continue
		}
		klog.Infof("Create health monitor of loadbalancer/pool(%s:%s) success.", service.Spec.LoadBalancerIP, pool.Id)

		// Step 4. add backend hosts
		for _, member := range members {
			member.ProtocolPort = port.NodePort
			_, err = albProvider.AddMember(pool.Id, member)
			if err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Create members of listener(%s) error: %v", listenerId, err)
				alb.sendEvent("CreateLoadBalancerFailed", msg, service)
				continue
			}
		}
		klog.Infof("Create members loadbalancer/pool(%s:%s) success.", service.Spec.LoadBalancerIP, pool.Id)
	}

	klog.Infof("Create listeners of service(%s/%s) finish, total: %d, failed: %d",
		service.Namespace, service.Name, len(needsCreate), len(errs))

	return utilerrors.NewAggregate(errs)
}

func (alb *ALBCloud) deleteLoadBalancer(
	albProvider *ALBClient,
	service *v1.Service,
	needsDelete []ALBListener) error {
	if len(needsDelete) == 0 {
		return nil
	}
	klog.Infof("Begin to delete useless listeners of service(%s/%s)", service.Namespace, service.Name)
	var errs []error
	for _, listener := range needsDelete {
		pool, err := albProvider.findPoolOfListener(listener.Id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if pool != nil {
			if err := alb.deleteListener(albProvider, listener.Id, pool.Id, pool.HealthMonitorId); err != nil {
				errs = append(errs, err)
				alb.sendEvent("DeleteLoadBalancerFailed", err.Error(), service)
			}
		}
	}

	klog.Infof("Delete useless listeners of service(%s/%s) finish, total: %d, failed: %d",
		service.Namespace, service.Name, len(needsDelete), len(errs))

	return utilerrors.NewAggregate(errs)
}

func (alb *ALBCloud) updateLoadBalancer(
	albProvider *ALBClient,
	service *v1.Service,
	needsUpdate map[string]tempALBServicePort,
	members []*ALBMember) error {
	var errs []error
	sessionAffinity := GetSessionAffinity(service)
	for _, tempPort := range needsUpdate {
		// Step 1: check protocol which is not allowed to update
		if ALBProtocol(tempPort.servicePort.Protocol) != tempPort.listener.Protocol {
			msg := fmt.Sprintf("The protocol of listener(%s) can not be modified", tempPort.listener.Id)
			alb.sendEvent("UpdateLoadBalancerFailed", msg, service)
			err := fmt.Errorf("%s", msg)
			errs = append(errs, err)
			continue
		}
		pool, err := albProvider.findPoolOfListener(tempPort.listener.Id)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Get pool of listener(%s) error: %v", tempPort.listener.Id, err)
			alb.sendEvent("UpdateLoadBalancerFailed", msg, service)
			continue
		}
		if pool == nil {
			poolConf := ALBPool{
				LbAlgorithm:  ALBAlgorithmRR,
				Protocol:     ALBProtocol(tempPort.servicePort.Protocol),
				ListenerId:   tempPort.listener.Id,
				AdminStateUp: true,
			}
			if sessionAffinity {
				poolConf.SessionPersistence.Type = ALBSessionSource
			}
			pool, err = albProvider.CreatePool(&poolConf)
			if err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Create pool of listener(%s) error: %v", tempPort.listener.Id, err)
				alb.sendEvent("UpdateLoadBalancerFailed", msg, service)
				continue
			}
		}
		// Step 2: update pool's session affinity
		if sessionAffinity == true && pool.SessionPersistence.Type != ALBSessionSource ||
			sessionAffinity == false && pool.SessionPersistence.Type == ALBSessionSource {
			tempPool := &ALBPool{}
			if sessionAffinity {
				tempPool.SessionPersistence.Type = ALBSessionSource
			}
			if _, err = albProvider.UpdatePool(tempPool, pool.Id); err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Update pool of listener(%s) error: %v", tempPort.listener.Id, err)
				alb.sendEvent("UpdateLoadBalancerFailed", msg, service)
				continue
			}
		}
		//Step 3: update members
		curMembers := make(map[string]*ALBMember)
		for _, member := range members {
			member.ProtocolPort = tempPort.servicePort.NodePort
			curMembers[member.Address] = member
		}

		preListMembers, err := albProvider.ListMembers(pool.Id)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Get member of listener/pool(%s/%s) error: %v", tempPort.listener.Id, pool.Id, err)
			alb.sendEvent("UpdateLoadBalancerFailed", msg, service)
			continue
		}
		preMembers := make(map[string]ALBMember)
		for _, member := range preListMembers.Members {
			preMembers[member.Address] = member
		}

		if err = alb.updateALbMembers(*albProvider, curMembers, preMembers, pool.Id); err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Update member of listener/pool(%s/%s) error: %v", tempPort.listener.Id, pool.Id, err)
			alb.sendEvent("UpdateLoadBalancerFailed", msg, service)
		}
	}

	klog.Infof("Update listeners of service(%s/%s) finish, total: %d, failed: %d",
		service.Namespace, service.Name, len(needsUpdate), len(errs))
	return utilerrors.NewAggregate(errs)
}
