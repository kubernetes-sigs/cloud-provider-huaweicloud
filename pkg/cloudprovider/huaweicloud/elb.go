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

// nolint:golint // stop check lint issues as this file will be refactored
package huaweicloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"
)

type ELBCloud struct {
	Basic
}

// temp async job info
// used for add members
type tempJobInfo struct {
	jobID  string
	detail string
}

type tempServicePort struct {
	servicePort *v1.ServicePort
	listener    *ListenerDetail
}

// getELBClient
func (elb *ELBCloud) ELBClient() (*ELBClient, error) {
	authOpts := elb.cloudConfig.AuthOpts
	return NewELBClient(authOpts.Cloud, authOpts.Region, authOpts.ProjectID, authOpts.AccessKey, authOpts.SecretKey), nil
}

// GetLoadBalancer gets loadbalancer for service.
func (elb *ELBCloud) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	status = &v1.LoadBalancerStatus{}
	// get the apigateway client
	listeners, err := elb.getListenersByService(service)
	if err != nil {
		return nil, false, err
	}
	if len(listeners) == 0 {
		return nil, false, nil
	}
	status.Ingress = append(status.Ingress, v1.LoadBalancerIngress{IP: service.Spec.LoadBalancerIP})
	return status, true, nil
}

// asyncWaitJobs means we just wait add/delete members backends,
// do not block the main process.
// tryAgain means we need to update service to requeue,
// bacause we need to make sure at least one member available in
// the process of upgrading, the delete member may be trigger this
// way
func (elb *ELBCloud) asyncWaitJobs(
	elbProvider *ELBClient,
	service *v1.Service,
	jobs []tempJobInfo,
	listenerID string,
	newMembers []*Member,
	tryAgain bool) {
	if len(jobs) == 0 && !tryAgain {
		return
	}

	go func() {
		var errs []error
		if len(jobs) != 0 {
			klog.Infof("Begin to wait jobs of service(%s/%s) finish...", service.Namespace, service.Name)
		}
		for _, job := range jobs {
			err := elbProvider.WaitJobComplete(job.jobID)
			if err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Job(%s) is abnormal: %v", job.detail, err)
				elb.sendEvent("CreateLoadBalancerFailed", msg, service)
				continue
			}
			klog.Infof("Job(%s) is success.", job.detail)
		}

		if len(errs) != 0 {
			klog.Warning("There have some abnormal jobs, needs to try again")
			elb.updateServiceStatus(elb.kubeClient, service)
		} else {
			if len(newMembers) != 0 {
				err := elbProvider.WaitMemberComplete(listenerID, newMembers)
				if err != nil {
					klog.Warningf("AsyncWaitJobs(%s/%s) wait member complete error: %v",
						service.Namespace, service.Name, err)
				}
			}

			updateServiceMarkIfNeeded(elb.kubeClient, service, tryAgain)
		}

		if len(jobs) != 0 {
			klog.Infof("Jobs of service(%s/%s) have finished: total: %d, failed: %d",
				service.Namespace, service.Name, len(jobs), len(errs))
		}
	}()
}

func (elb *ELBCloud) ensureCreateListener(
	elbProvider *ELBClient,
	name string,
	elbAlgorithm ELBAlgorithm,
	port v1.ServicePort,
	loadBalancerID string,
	sessionAffinity string,
	sessionAffinityOpts map[string]string) (listenerID string, err error) {
	listenerConf := &Listener{
		LoadbalancerID:  "",
		Protocol:        ELBProtocol(port.Protocol),
		Port:            int(port.Port),
		BackendProtocol: ELBProtocol(port.Protocol),
		BackendPort:     int(port.NodePort),
		LBAlgorithm:     elbAlgorithm,
	}

	listenerConf.Name = name
	listenerConf.AdminStateUp = true
	listenerConf.Description = Attention

	switch sessionAffinity {
	case ELBSessionSourceIP:
		listenerConf.SessionSticky = true
		listenerConf.TCPTimeout, _ = strconv.Atoi(sessionAffinityOpts[ELBPersistenceTimeout])
	case ELBSessionNone:
		//do nothing
	}

	listenerConf.LoadbalancerID = loadBalancerID
	klog.Infof("Begin to create listener(%s/%d) of loadbalancer(%s)...", name, port.Port, loadBalancerID)
	listener, errRsp, err := elbProvider.CreateListener(listenerConf)
	if err != nil {
		// if the listener already exit
		if errRsp != nil && errRsp.Error.Code == ElbError6101 {
			return "", nil
		}
		return "", err
	}
	return listener.ID, nil
}

func (elb *ELBCloud) getPods(name, namespace string) (*v1.PodList, error) {
	service, err := elb.kubeClient.Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if len(service.Spec.Selector) == 0 {
		return nil, fmt.Errorf("the service %s has no selector to associate the pods", name)
	}

	set := labels.Set(service.Spec.Selector)
	labelSelector := labels.SelectorFromSet(set)

	opts := metav1.ListOptions{LabelSelector: labelSelector.String()}
	return elb.kubeClient.Pods(namespace).List(context.TODO(), opts)
}

// Not implemented
func (elb *ELBCloud) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	return ""
}

// EnsureTCPLoadBalancer is an implementation of TCPLoadBalancer.EnsureTCPLoadBalancer.
// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
func (elb *ELBCloud) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, hosts []*v1.Node) (*v1.LoadBalancerStatus, error) {
	// func (elb *ELBCloud) EnsureLoadBalancer(name, region string, loadBalancerIP net.IP, ports []*v1.ServicePort, hosts []string, servicename types.NamespacedName, affinityType v1.ServiceAffinity, annotations map[string]string) (*v1.LoadBalancerStatus, error) {
	klog.Infof("Begin to ensure loadbalancer configuration of service(%s/%s)", service.Namespace, service.Name)
	elbProvider, err := elb.ELBClient()
	if err != nil {
		return nil, err
	}

	healthCheckPort := GetHealthCheckPort(service)
	listeners, err := elb.getListenersByService(service)
	if err != nil {
		return nil, err
	}

	members, err := elb.generateMembers(service)
	if err != nil {
		return nil, err
	}

	params := make(map[string]string)
	if elb.cloudConfig.VpcOpts.ID != "" {
		params["vpc_id"] = elb.cloudConfig.VpcOpts.ID
	}
	if service.Spec.LoadBalancerIP != "" {
		params["vip_address"] = service.Spec.LoadBalancerIP
	}
	lbList, err := elbProvider.ListLoadBalancers(params)
	if err != nil {
		return nil, err
	}

	if len(lbList.Loadbalancers) > 1 {
		return nil, fmt.Errorf("find more than one Loadbalancer(service:%s/%s)", service.Namespace, service.Name)
	} else if len(lbList.Loadbalancers) == 0 {
		return nil, fmt.Errorf("can't find matched Loadbalancer")
	}

	loadBalancerID := lbList.Loadbalancers[0].LoadbalancerId
	needsCreate, needsUpdate, needsDelete := elb.compare(loadBalancerID, service, listeners)
	ch := make(chan error, 3)

	go func() {
		ch <- elb.createLoadBalancer(elbProvider, loadBalancerID, service, needsCreate, healthCheckPort, members)
	}()

	go func() {
		ch <- elb.updateLoadBalancer(elbProvider, service, needsUpdate, healthCheckPort, members)
	}()

	go func() {
		ch <- elb.deleteLoadBalancer(elbProvider, service, needsDelete)
	}()

	var errs []error
	for i := 3; i > 0; i-- {
		err := <-ch
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	status := &v1.LoadBalancerStatus{}
	status.Ingress = append(status.Ingress, v1.LoadBalancerIngress{IP: service.Spec.LoadBalancerIP})
	return status, nil
}

// UpdateTCPLoadBalancer is an implementation of TCPLoadBalancer.UpdateTCPLoadBalancer.
// if update return failed, the caller must backup the failed service and try to update again.
func (elb *ELBCloud) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, hosts []*v1.Node) error {
	// if the node changed ,the server_id mark the VM will change, need to update the global
	klog.Infof("Begin to update loadbalancer configuration of service(%s/%s)", service.Namespace, service.Name)
	elbProvider, err := elb.ELBClient()
	if err != nil {
		return err
	}

	members, err := elb.generateMembers(service)
	if err != nil {
		return err
	}

	listeners, err := elb.getListenersByService(service)
	if err != nil {
		return err
	}

	var errs []error
	for _, listener := range listeners {
		preMembers, err := elbProvider.ListMembers(listener.ID)
		if err != nil {
			errs = append(errs, err)
			klog.Errorf("List members of listener(listener.ID) in service(%s/%s) error: %v", service.Namespace, service.Name, err)
			continue
		}

		if err = elb.updateListenerMembers(elbProvider, service, listener.ID, members, preMembers); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}

// updateELbMembers delete the old node that pod has been evicted, add the new node the pod run in
func (elb *ELBCloud) updateListenerMembers(
	elbProvider *ELBClient,
	service *v1.Service,
	listenerID string,
	newMembers []*Member,
	preMembers []*MemDetail) error {
	addMembers := []*Member{}
	matchedMembers := map[string]*MemDetail{}
	membersDel := &MembersDel{}
	jobs := []tempJobInfo{}

	for _, member := range newMembers {
		create := true
		for _, preMember := range preMembers {
			if preMember.ServerID == member.ServerID {
				create = false
				matchedMembers[preMember.ServerID] = preMember
				break
			}
		}

		if create {
			addMembers = append(addMembers, member)
		}
	}

	for _, preMember := range preMembers {
		if _, ok := matchedMembers[preMember.ServerID]; !ok {
			memberRm := MemberRm{ID: preMember.ID, Address: preMember.Address}
			membersDel.RemoveMember = append(membersDel.RemoveMember, memberRm)
		}
	}

	if len(addMembers) != 0 {
		klog.Infof("Begin to add members(%v) of service(%s/%s)", addMembers, service.Namespace, service.Name)
		addJob, err := elbProvider.AsyncCreateMembers(listenerID, addMembers)
		if err != nil {
			msg := fmt.Sprintf("Add members of listener(%s) error: %v", listenerID, err)
			elb.sendEvent("UpdateLoadBalancerFailed", msg, service)
			return fmt.Errorf(msg)
		}
		jobs = append(jobs, tempJobInfo{
			jobID:  addJob.JobID,
			detail: fmt.Sprintf("Add listener(%s) members", listenerID),
		})
	}

	if elb.gracefulRemoveElbMembers(matchedMembers) {
		if len(membersDel.RemoveMember) != 0 {
			klog.Infof("Can trigger graceful remove members(%s/%d) of service(%s/%s)",
				listenerID, len(membersDel.RemoveMember), service.Namespace, service.Name)
			delJob, err := elbProvider.AsyncDeleteMembers(listenerID, membersDel)
			if err != nil {
				msg := fmt.Sprintf("Delete members of listener(%s) error: %v", listenerID, err)
				elb.sendEvent("UpdateLoadBalancerFailed", msg, service)
				return fmt.Errorf(msg)
			}
			jobs = append(jobs, tempJobInfo{
				jobID:  delJob.JobID,
				detail: fmt.Sprintf("Delete listener(%s) members", listenerID),
			})
		}
	}

	if len(addMembers) != 0 {
		klog.Infof("In upgrading process(%s/%s), when add member is finshed, needs retry to clean the useless members",
			service.Namespace, service.Name)
		elb.asyncWaitJobs(elbProvider, service, jobs, listenerID, addMembers, true)
		return nil
	}
	elb.asyncWaitJobs(elbProvider, service, jobs, listenerID, nil, false)

	return nil
}

// although member has already been added, in fact,
// the member will wait the health check is ok, then
// transfer the packages, so before delete the old member,
// we need to check if there have available members
func (elb *ELBCloud) gracefulRemoveElbMembers(existMembers map[string]*MemDetail) bool {
	hasAvailableMember := false
	for _, member := range existMembers {
		if member.HealthStatus == MemberNormal {
			hasAvailableMember = true
			break
		}
	}

	return hasAvailableMember
}

// EnsureTCPLoadBalancerDeleted is an implementation of TCPLoadBalancer.EnsureTCPLoadBalancerDeleted.
func (elb *ELBCloud) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.Infof("Begin to delete loadbalancer configuration of service(%s/%s)", service.Namespace, service.Name)
	elbProvider, err := elb.ELBClient()
	if err != nil {
		return err
	}

	listeners, err := elb.getListenersByService(service)
	if err != nil {
		return err
	}

	klog.Infof("Begin to delete listeners of service(%s/%s)", service.Namespace, service.Name)
	// get the server name same to the  listener (it had been assign at create listener)
	var errs []error
	for _, listener := range listeners {
		if err = deleteListener(elbProvider, listener.ID, listener.HealthcheckID); err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Delete listener(%s) error: %v", listener.ID, err)
			elb.sendEvent("DeleteLoadBalancerFailed", msg, service)
		}
	}

	klog.Infof("Delete listeners of service(%s/%s) finish, total: %d, failed: %d",
		service.Namespace, service.Name, len(listeners), len(errs))

	return utilerrors.NewAggregate(errs)
}

func (elb *ELBCloud) getListenersByService(service *v1.Service) ([]*ListenerDetail, error) {
	elbProvider, err := elb.ELBClient()
	if err != nil {
		return nil, err
	}

	// we should get all listeners, for three reasons:
	// 1. service can without elb.id
	// 2. service can without loadbalancerIP
	// 3. service maybe update the loadbalancerIP
	// and check the listener name(TODO: this is not a safe way)
	// return the matched items.
	listenerList, err := elbProvider.ListListeners("")
	if err != nil {
		return nil, err
	}

	var listeners []*ListenerDetail
	for _, listener := range listenerList {
		if listener.Name == GetListenerName(service) || listener.Name == GetOldListenerName(service) {
			listeners = append(listeners, listener)
		}
	}
	return listeners, nil
}

func (elb *ELBCloud) compare(
	loadBalancerID string,
	service *v1.Service,
	listeners []*ListenerDetail) ([]v1.ServicePort, map[string]tempServicePort, []*ListenerDetail) {
	needsCreate := []v1.ServicePort{}
	needsUpdate := make(map[string]tempServicePort)
	needsDelete := []*ListenerDetail{}
	for i := range service.Spec.Ports {
		port := service.Spec.Ports[i]
		if port.Name == HealthzCCE {
			continue
		}
		create := true
		for j := range listeners {
			listener := listeners[j]
			if int(port.Port) == listener.Port &&
				listener.LoadbalancerID == loadBalancerID {
				create = false
				needsUpdate[listener.ID] = tempServicePort{
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
		if _, ok := needsUpdate[listener.ID]; ok {
			continue
		}
		needsDelete = append(needsDelete, listener)
	}

	return needsCreate, needsUpdate, needsDelete
}

func (elb *ELBCloud) generateMembers(service *v1.Service) ([]*Member, error) {
	podList, err := elb.getPods(service.Name, service.Namespace)
	if err != nil {
		return nil, err
	}

	members := []*Member{}
	hasNodeExist := map[string]bool{}
	for _, item := range podList.Items {
		if item.Status.HostIP == "" {
			klog.Errorf("pod(%s/%s) has not been scheduled", item.Namespace, item.Name)
			continue
		}

		if !IsPodActive(item) {
			klog.Errorf("pod(%s/%s) is not active", item.Namespace, item.Name)
			continue
		}

		if hasNodeExist[item.Status.HostIP] {
			continue
		}

		node, err := elb.kubeClient.Nodes().Get(context.TODO(), item.Spec.NodeName, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("Get node(%s) error: %v", item.Spec.NodeName, err)
			continue
		}

		hasNodeExist[item.Status.HostIP] = true
		// Get the sever by private IP, it must only have one, if it exist.
		member := Member{ServerID: node.Status.NodeInfo.MachineID, Address: item.Status.HostIP}
		members = append(members, &member)
	}

	// TODO: consider the pods have been deleted.
	if len(members) == 0 {
		return nil, fmt.Errorf("have no node to bind")
	}

	return members, nil
}

func (elb *ELBCloud) createLoadBalancer(
	elbProvider *ELBClient,
	loadBalancerID string,
	service *v1.Service,
	needsCreate []v1.ServicePort,
	healthCheckPort *v1.ServicePort,
	members []*Member) error {
	var (
		errs []error
		jobs []tempJobInfo
	)
	lsName := GetListenerName(service)
	sessionAffinity, err := elb.getSessionAffinityType(service)
	if err != nil {
		msg := fmt.Sprintf("Create loadbalancer(%s) error: %v", service.Spec.LoadBalancerIP, err)
		elb.sendEvent("CreateLoadBalancerFailed", msg, service)
		return err
	}
	sessionAffinityOptions, err := elb.getSessionAffinityOptions(service)
	if err != nil {
		msg := fmt.Sprintf("Create loadbalancer(%s) error: %v", service.Spec.LoadBalancerIP, err)
		elb.sendEvent("CreateLoadBalancerFailed", msg, service)
		return err
	}

	for _, port := range needsCreate {
		// Step 1. create listener if needed
		listenerID, err := elb.ensureCreateListener(
			elbProvider,
			lsName,
			"ROUND_ROBIN", //TODO: we will support other algorithms later
			port,
			loadBalancerID,
			sessionAffinity,
			sessionAffinityOptions,
		)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Create listener(%d) error: %v", port.Port, err)
			elb.sendEvent("CreateLoadBalancerFailed", msg, service)
			continue
		}

		if listenerID == "" {
			msg := fmt.Sprintf("The listener(%d) has already exist", port.Port)
			elb.sendEvent("CreateLoadBalancerFailed", msg, service)
			continue
		}

		klog.Infof("Create listener(%s/%d) of loadbalancer(%s) success.", lsName, port.Port, service.Spec.LoadBalancerIP)

		// Step 2. Create health check
		healthCheck := HealthCheck{
			HealthcheckConnectPort: int(port.NodePort),
			HealthcheckInterval:    5,
			HealthcheckProtocol:    ELBProtocol(port.Protocol),
			HealthcheckTimeout:     10,
			HealthyThreshold:       3,
			ListenerID:             listenerID,
			UnhealthyThreshold:     3,
		}
		if healthCheckPort != nil {
			healthCheck.HealthcheckConnectPort = int(healthCheckPort.NodePort)
			healthCheck.HealthcheckProtocol = ELBProtocol(healthCheckPort.Protocol)
		}

		_, err = elbProvider.CreateHealthCheck(&healthCheck)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Create healthcheck of listener(%s) error: %v", listenerID, err)
			elb.sendEvent("CreateLoadBalancerFailed", msg, service)
			continue
		}

		klog.Infof("Create healthcheck of listener(%s) success.", listenerID)

		// Step 3. add backend hosts
		job, err := elbProvider.AsyncCreateMembers(listenerID, members)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Create members of listener(%s) error: %v", listenerID, err)
			elb.sendEvent("CreateLoadBalancerFailed", msg, service)
			continue
		}
		klog.Infof("Create members of listener(%s) success.", listenerID)
		jobs = append(jobs, tempJobInfo{
			jobID:  job.JobID,
			detail: fmt.Sprintf("Create members of listener(%s)", listenerID),
		})
	}

	elb.asyncWaitJobs(elbProvider, service, jobs, "", nil, false)

	if len(errs) != 0 {
		return utilerrors.NewAggregate(errs)
	}

	return nil
}

func (elb *ELBCloud) updateLoadBalancer(
	elbProvider *ELBClient,
	service *v1.Service,
	needsUpdate map[string]tempServicePort,
	healthCheckPort *v1.ServicePort,
	members []*Member) error {
	var errs []error

	sessionAffinity, err := elb.getSessionAffinityType(service)
	if err != nil {
		msg := fmt.Sprintf("Update loadbalancer(%s) error: %v", service.Spec.LoadBalancerIP, err)
		elb.sendEvent("UpdateLoadBalancerFailed", msg, service)
		return err
	}
	sessionAffinityOptions, err := elb.getSessionAffinityOptions(service)
	if err != nil {
		msg := fmt.Sprintf("Update loadbalancer(%s) error: %v", service.Spec.LoadBalancerIP, err)
		elb.sendEvent("UpdateLoadBalancerFailed", msg, service)
		return err
	}

	for _, tempPort := range needsUpdate {
		if ELBProtocol(tempPort.servicePort.Protocol) != tempPort.listener.Protocol {
			msg := fmt.Sprintf("The protocol of listener(%s) can not be modified", tempPort.listener.ID)
			elb.sendEvent("UpdateLoadBalancerFailed", msg, service)
			continue
		}

		var sessionSticky bool
		var timeout int
		switch sessionAffinity {
		case ELBSessionSourceIP:
			sessionSticky = true
			timeout, _ = strconv.Atoi(sessionAffinityOptions[ELBPersistenceTimeout])
		case ELBSessionNone:
			sessionSticky = false
		}

		// needs to update listener
		if int(tempPort.servicePort.NodePort) != tempPort.listener.BackendPort || tempPort.listener.SessionSticky != sessionSticky || tempPort.listener.TCPTimeout != timeout {
			klog.Infof("Needs to update listener(%s)'s backend port(%d->%d), session_sticky(%v->%v) ,session_timeout(%d->%d)of service(%s/%s)",
				tempPort.listener.ID, tempPort.listener.BackendPort, tempPort.servicePort.NodePort, tempPort.listener.SessionSticky, sessionSticky, tempPort.listener.TCPTimeout, timeout, service.Namespace, service.Name)
			ll := &Listener{}
			ll.BackendPort = int(tempPort.servicePort.NodePort)
			ll.SessionSticky = sessionSticky
			if sessionSticky {
				ll.TCPTimeout = timeout
			}
			_, err := elbProvider.UpdateListener(ll, tempPort.listener.ID)
			if err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Update listener(%s) error: %v", tempPort.listener.ID, err)
				elb.sendEvent("UpdateLoadBalancerFailed", msg, service)
				continue
			}
		}

		// update healthcheck if needed
		err := elb.updateHealthcheckIfNeeded(elbProvider, service, tempPort, healthCheckPort)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// update members if needed
		currentMembers, err := elbProvider.ListMembers(tempPort.listener.ID)
		if err != nil {
			errs = append(errs, err)
			klog.Errorf("List members of listener(%s) in service(%s/%s) error: %v",
				tempPort.listener.ID, service.Namespace, service.Name, err)
			continue
		}

		err = elb.updateListenerMembers(elbProvider, service, tempPort.listener.ID, members, currentMembers)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (elb *ELBCloud) deleteLoadBalancer(
	elbProvider *ELBClient,
	service *v1.Service,
	needsDelete []*ListenerDetail) error {
	if len(needsDelete) == 0 {
		return nil
	}
	klog.Infof("Begin to delete useless listeners of service(%s/%s)", service.Namespace, service.Name)
	var errs []error
	for _, listener := range needsDelete {
		if err := deleteListener(elbProvider, listener.ID, listener.HealthcheckID); err != nil {
			errs = append(errs, err)
			elb.sendEvent("DeleteLoadBalancerFailed", err.Error(), service)
			continue
		}
	}

	klog.Infof("Delete useless listeners of service(%s/%s) finish, total: %d, failed: %d",
		service.Namespace, service.Name, len(needsDelete), len(errs))

	return utilerrors.NewAggregate(errs)
}

func deleteListener(
	elbProvider *ELBClient,
	listenerID, healthcheckID string) error {
	if listenerID != "" {
		err := elbProvider.DeleteMembers(listenerID)
		if err != nil {
			return fmt.Errorf("Delete members of listener(%s) error: %v", listenerID, err)
		}
	}

	// TODO: this is a bug of elb, when there has no healthcheck in the listener
	// its healthcheck_id is not empty, but "null"
	if healthcheckID != "" && healthcheckID != "null" {
		err := elbProvider.DeleteHealthCheck(healthcheckID)
		if err != nil {
			return fmt.Errorf("Delete healthcheck of listener(%s) error: %v", listenerID, err)
		}
	}

	if listenerID != "" {
		err := elbProvider.DeleteListener(listenerID)
		if err != nil {
			return fmt.Errorf("Delete listener(%s) error: %v", listenerID, err)
		}
	}

	return nil
}

func (elb *ELBCloud) updateHealthcheckIfNeeded(
	elbProvider *ELBClient,
	service *v1.Service,
	tempPort tempServicePort,
	healthCheckPort *v1.ServicePort) error {
	healthcheckPort := tempPort.servicePort.NodePort
	healthcheckProtocol := tempPort.servicePort.Protocol
	if healthCheckPort != nil {
		healthcheckPort = healthCheckPort.NodePort
		healthcheckProtocol = healthCheckPort.Protocol
	}

	var (
		notexist bool
		healthz  *HealthCheckDetail
		errResp  *ErrorRsp
		err      error
	)
	if tempPort.listener.HealthcheckID == "" ||
		tempPort.listener.HealthcheckID == "null" {
		notexist = true
	} else {
		healthz, errResp, err = elbProvider.GetHealthCheck(tempPort.listener.HealthcheckID)
		if err != nil {
			// if healthcheck is not exist, this maybe happen when rollback is not finished,
			// then we should create the healthcheck again.
			if errResp != nil && errResp.Error.Code == ElbError7020 {
				notexist = true
			} else {
				klog.Errorf("Get healthcheck of listener(%s) in service(%s/%s) error: %v",
					tempPort.listener.ID, service.Namespace, service.Name, err)
				return err
			}
		}
	}

	// needs to create healthcheck
	if notexist {
		klog.Infof("Needs to create healthcheck(%d/%s) of listener(%s) in service(%s/%s)",
			healthcheckPort, healthcheckProtocol, tempPort.listener.ID, service.Namespace, service.Name)
		h := &HealthCheck{
			HealthcheckConnectPort: int(healthcheckPort),
			HealthcheckInterval:    5,
			HealthcheckProtocol:    ELBProtocol(healthcheckProtocol),
			HealthcheckTimeout:     10,
			HealthyThreshold:       3,
			ListenerID:             tempPort.listener.ID,
			UnhealthyThreshold:     3,
		}
		_, err = elbProvider.CreateHealthCheck(h)
		if err != nil {
			msg := fmt.Sprintf("Create healthcheck of listener(%s) error: %v", tempPort.listener.ID, err)
			elb.sendEvent("UpdateLoadBalancerFailed", msg, service)
			return err
		}
		return nil
	}

	// needs to update healthcheck
	if int(healthcheckPort) != healthz.HealthcheckConnectPort ||
		ELBProtocol(healthcheckProtocol) != healthz.HealthcheckProtocol {
		klog.Infof("Needs to update healthcheck(%d/%s->%d/%s) of listener(%s) in service(%s/%s)",
			healthz.HealthcheckConnectPort, healthz.HealthcheckProtocol, healthcheckPort, healthcheckProtocol,
			tempPort.listener.ID, service.Namespace, service.Name)
		h := &HealthCheck{
			HealthcheckConnectPort: int(healthcheckPort),
			HealthcheckInterval:    5,
			HealthcheckProtocol:    ELBProtocol(healthcheckProtocol),
			HealthcheckTimeout:     10,
			HealthyThreshold:       3,
			UnhealthyThreshold:     3,
		}
		_, err = elbProvider.UpdateHealthCheck(h, tempPort.listener.HealthcheckID)
		if err != nil {
			msg := fmt.Sprintf("Update healthcheck of listener(%s) error: %v", tempPort.listener.ID, err)
			elb.sendEvent("UpdateLoadBalancerFailed", msg, service)
			return err
		}
	}

	return nil
}

func (elb *ELBCloud) getSessionAffinityType(service *v1.Service) (string, error) {
	switch mode := GetSessionAffinityType(service); mode {
	case ELBSessionSourceIP:
		return ELBSessionSourceIP, nil
	case "":
		return ELBSessionNone, nil
	default:
		return "", fmt.Errorf("session affinity type:%s not support now", mode)
	}
}

func (elb *ELBCloud) getSessionAffinityOptions(service *v1.Service) (map[string]string, error) {
	sessionAffinityOptions := make(map[string]string)
	if option := GetSessionAffinityOptions(service); option != "" {
		err := json.Unmarshal([]byte(option), &sessionAffinityOptions)
		if err != nil {
			return nil, fmt.Errorf("invalid session affinity option[parse json failed]")
		}
	}
	switch mode := GetSessionAffinityType(service); mode {
	case ELBSessionSourceIP:
		if val, ok := sessionAffinityOptions[ELBPersistenceTimeout]; ok {
			timeout, err := strconv.Atoi(val)
			if err != nil || timeout > ELBSessionSourceIPMaxTimeout || timeout < ELBSessionSourceIPMinTimeout {
				return nil, fmt.Errorf("invalid session affinity option ,invalid cookie timeout [%d<timeout<%d]",
					ELBSessionSourceIPMinTimeout, ELBSessionSourceIPMaxTimeout)
			}
		} else {
			//set default persistent timeout to 60 minus
			sessionAffinityOptions[ELBPersistenceTimeout] = fmt.Sprintf("%d", ELBSessionSourceIPDefaultTimeout)
		}
	}
	return sessionAffinityOptions, nil
}

func GetListenerName(service *v1.Service) string {
	return string(service.UID)
}

// to suit for old version
// if the elb has been created with the old version
// its listener name is service.name+service.uid
func GetOldListenerName(service *v1.Service) string {
	return strings.Replace(service.Name+"_"+string(service.UID), ".", "_", -1)
}

func (elb *ELBCloud) updateServiceStatus(kubeClient corev1.CoreV1Interface, service *v1.Service) {
	for i := 0; i < MaxRetry; i++ {
		toUpdate := service.DeepCopy()
		mark, ok := toUpdate.Annotations[ELBMarkAnnotation]
		if !ok {
			mark = "1"
			if toUpdate.Annotations == nil {
				toUpdate.Annotations = map[string]string{}
			}
		} else {
			retry, err := strconv.Atoi(mark)
			if err != nil {
				mark = "1"
			} else {
				// always retry will send too many requests to apigateway, this maybe case ddos
				if retry >= MaxRetry {
					elb.sendEvent("CreateLoadBalancerFailed", "Retry LoadBalancer configuration too many times", service)
					return
				}
				retry++
				mark = fmt.Sprintf("%d", retry)
			}
		}
		toUpdate.Annotations[ELBMarkAnnotation] = mark
		_, err := kubeClient.Services(service.Namespace).Update(context.TODO(), toUpdate, metav1.UpdateOptions{})
		if err == nil {
			return
		}
		// If the object no longer exists, we don't want to recreate it. Just bail
		// out so that we can process the delete, which we should soon be receiving
		// if we haven't already.
		if apierrors.IsNotFound(err) {
			klog.Infof("Not persisting update to service '%s/%s' that no longer exists: %v",
				service.Namespace, service.Name, err)
			return
		}

		if apierrors.IsConflict(err) {
			service, err = kubeClient.Services(service.Namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
			if err != nil {
				klog.Warningf("Get service(%s/%s) error: %v", service.Namespace, service.Name, err)
				continue
			}
		}
	}
}

// if async job succeed, need to init mark again
func updateServiceMarkIfNeeded(
	kubeClient corev1.CoreV1Interface,
	service *v1.Service,
	tryAgain bool) {
	for i := 0; i < MaxRetry; i++ {
		toUpdate := service.DeepCopy()
		_, ok := toUpdate.Annotations[ELBMarkAnnotation]
		if !ok {
			if !tryAgain {
				return
			}

			if toUpdate.Annotations == nil {
				toUpdate.Annotations = map[string]string{}
			}
			toUpdate.Annotations[ELBMarkAnnotation] = "0"
		} else {
			delete(toUpdate.Annotations, ELBMarkAnnotation)
		}

		_, err := kubeClient.Services(service.Namespace).Update(context.TODO(), toUpdate, metav1.UpdateOptions{})
		if err == nil {
			return
		}

		// If the object no longer exists, we don't want to recreate it. Just bail
		// out so that we can process the delete, which we should soon be receiving
		// if we haven't already.
		if apierrors.IsNotFound(err) {
			klog.Infof("Not persisting update to service '%s/%s' that no longer exists: %v",
				service.Namespace, service.Name, err)
			return
		}

		if apierrors.IsConflict(err) {
			service, err = kubeClient.Services(service.Namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
			if err != nil {
				klog.Warningf("Get service(%s/%s) error: %v", service.Namespace, service.Name, err)
				continue
			}
		}
	}

}
