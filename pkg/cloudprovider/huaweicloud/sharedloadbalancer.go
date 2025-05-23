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

package huaweicloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	eipmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eip/v2/model"
	elbmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v2/model"
	elbmodelv3 "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v3/model"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/wrapper"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/common"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/utils"
)

const (
	defaultMaxNameLength     = 255
	maxServerGroupNameLength = 64
)

var (
	allowedIPTypes = map[v1.NodeAddressType]bool{
		v1.NodeInternalIP: true,
		v1.NodeExternalIP: true,
	}
)

type SharedLoadBalancer struct {
	Basic
}

func (l *SharedLoadBalancer) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	klog.Infof("GetLoadBalancer: called with service %s/%s", service.Namespace, service.Name)
	loadbalancer, err := l.getLoadBalancerInstance(ctx, clusterName, service)
	if err != nil {
		if common.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	portID := loadbalancer.VipPortId
	if portID == "" {
		return nil, false, status.Errorf(codes.Unavailable, "The ELB %s VipPortId is empty, "+
			"and the instance is unavailable", l.GetLoadBalancerName(ctx, clusterName, service))
	}
	ingressIP := loadbalancer.VipAddress

	ips, err := l.eipClient.List(&eipmodel.ListPublicipsRequest{PortId: &[]string{portID}})
	if err != nil {
		return nil, false, status.Errorf(codes.Unavailable, "error querying EIPs base on PortId (%s): %s", portID, err)
	}
	if len(ips) > 0 {
		ingressIP = *ips[0].PublicIpAddress
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{IP: ingressIP},
		},
	}, true, nil
}

// nolint: revive
func (l *SharedLoadBalancer) getLoadBalancerInstance(ctx context.Context, clusterName string, service *v1.Service) (*elbmodel.LoadbalancerResp, error) {
	if id := getStringFromSvsAnnotation(service, ElbID, ""); id != "" {
		return l.sharedELBClient.GetInstance(id)
	}

	name := l.GetLoadBalancerName(ctx, clusterName, service)
	list, err := l.sharedELBClient.ListInstances(&elbmodel.ListLoadbalancersRequest{Name: &name})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, status.Errorf(codes.NotFound, "not found ELB instance %s", name)
	}
	if len(list) != 1 {
		return nil, status.Errorf(codes.Unavailable, "error, found %d ELBs named %s, make sure there is only one",
			len(list), name)
	}
	return &list[0], nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *SharedLoadBalancer) GetLoadBalancerName(_ context.Context, clusterName string, service *v1.Service) string {
	klog.Infof("GetLoadBalancerName: called with service %s/%s", service.Namespace, service.Name)
	if l.loadbalancerOpts.BusinessName != "" {
		clusterName = l.loadbalancerOpts.BusinessName
	}
	name := fmt.Sprintf("k8s_service_%s_%s_%s", clusterName, service.Namespace, service.Name)
	return utils.CutString(name, defaultMaxNameLength)
}

func ensureLoadBalancerValidation(service *v1.Service, nodes []*v1.Node) error {
	if len(nodes) == 0 {
		return fmt.Errorf("there are no available nodes for LoadBalancer service %s/%s",
			service.Namespace, service.Name)
	}

	ports := service.Spec.Ports
	if len(ports) == 0 {
		return fmt.Errorf("the loadbalancer service does not configure Spec.Ports")
	}
	if len(service.Spec.Selector) == 0 {
		return fmt.Errorf("the loadbalancer service does not provide Selector, " +
			"services custom endpoints are not supported")
	}

	return nil
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
//
// nolint: revive
func (l *SharedLoadBalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	klog.Infof("EnsureLoadBalancer: called with service %s/%s, node: %d", service.Namespace, service.Name, len(nodes))
	if !l.isSupportedSvc(service) {
		return nil, cloudprovider.ImplementedElsewhere
	}

	if err := ensureLoadBalancerValidation(service, nodes); err != nil {
		return nil, err
	}

	// get exits or create a new ELB instance
	loadbalancer, err := l.getLoadBalancerInstance(ctx, clusterName, service)
	specifiedID := getStringFromSvsAnnotation(service, ElbID, "")
	if common.IsNotFound(err) && specifiedID != "" {
		return nil, err
	}
	if err != nil && common.IsNotFound(err) {
		subnetID := getStringFromSvsAnnotation(service, ElbSubnetID, l.cloudConfig.VpcOpts.SubnetID)
		if subnetID == "" {
			return nil, status.Errorf(codes.InvalidArgument, "missing subnet-id, "+
				"can not to read subnet-id from service or cloud-config")
		}
		loadbalancer, err = l.createLoadbalancer(clusterName, subnetID, service)
	}
	if err != nil {
		return nil, err
	}

	// query ELB listeners list
	listeners, err := l.sharedELBClient.ListListeners(&elbmodel.ListListenersRequest{LoadbalancerId: &loadbalancer.Id})
	if err != nil {
		return nil, err
	}

	for _, port := range service.Spec.Ports {
		listener := l.filterListenerByPort(listeners, service, port)
		// add or update listener
		if listener == nil {
			listener, err = l.createListener(loadbalancer.Id, service, port)
		} else {
			err = l.updateListener(listener, service)
		}
		if err != nil {
			return nil, err
		}

		listeners = popListener(listeners, listener.Id)

		// query pool or create pool
		pool, err := l.getPool(loadbalancer.Id, listener.Id)
		if err != nil && common.IsNotFound(err) {
			pool, err = l.createPool(listener, service)
		}
		if err != nil {
			return nil, err
		}

		// add new members and remove the obsolete members.
		if err = l.addOrRemoveMembers(loadbalancer, service, pool, port, nodes); err != nil {
			return nil, err
		}

		// add or remove health monitor
		if err = l.ensureHealthCheck(loadbalancer.Id, pool, port, service, nodes[0]); err != nil {
			return nil, err
		}
	}

	if specifiedID == "" {
		// All remaining listeners are obsolete, delete them
		err = l.deleteListeners(loadbalancer.Id, listeners)
		if err != nil {
			return nil, err
		}
	}

	ingressIP := loadbalancer.VipAddress
	publicIPAddr, err := l.createOrAssociateEIP(loadbalancer, service)
	if err == nil {
		if publicIPAddr != "" {
			ingressIP = publicIPAddr
		}

		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{{IP: ingressIP}},
		}, nil
	}

	// rollback
	klog.Errorf("rollback：failed to create the EIP, delete ELB instance created, error: %s", err)
	errs := []error{err}
	err = l.EnsureLoadBalancerDeleted(ctx, clusterName, service)
	if err != nil {
		errs = append(errs, err)
		klog.Errorf("rollback: error deleting ELB instance: %s", err)
	}
	return nil, errors.NewAggregate(errs)
}

// nolint: revive
func (l *SharedLoadBalancer) createOrAssociateEIP(loadbalancer *elbmodel.LoadbalancerResp, service *v1.Service) (string, error) {
	var err error
	specifiedEip := true
	instance, err := l.sharedELBClient.Show(loadbalancer.Id)
	if err != nil {
		return "", status.Errorf(codes.Internal, "rollback：failed to query detail of ELB instance, error: %s", err)
	}

	eipID := getStringFromSvsAnnotation(service, ElbEipID, "")
	if eipID == "" {
		opts, err := parseEIPAutoCreateOptions(service)
		if err != nil || opts == nil {
			return "", err
		}

		if len(instance.PublicIPs) > 0 {
			klog.Infof("the ELB has bound EIP: %s / %s, skip creating EIP", instance.PublicIPs[0].Address, instance.PublicIPs[0].ID)
			return instance.PublicIPs[0].Address, nil
		}

		eipID, err = l.createEIP(service)
		if err != nil {
			return "", status.Errorf(codes.Internal, "rollback：failed to create EIP, delete ELB instance, error: %s", err)
		}
		specifiedEip = false
	}
	if eipID == "" {
		return "", nil
	}

	if specifiedEip && len(instance.PublicIPs) > 0 {
		if instance.PublicIPs[0].ID == eipID {
			klog.Infof("the ELB has been bound to the specified EIP %s", eipID)
			return instance.PublicIPs[0].Address, nil
		}

		// remove bound EIP
		klog.Infof("remove the bound EIP %s and the specified will be used: %s", instance.PublicIPs[0].ID, eipID)
		err = l.eipClient.Unbind(instance.PublicIPs[0].ID)
		if err != nil {
			return "", status.Errorf(codes.Internal, "rollback：failed to unbind EIP from ELB instance, error: %s", err)
		}
	}

	eip, err := l.eipClient.Get(eipID)
	if err != nil {
		return "", status.Errorf(codes.Internal, "rollback：failed to get EIP, delete ELB instance, error: %s", err)
	}

	if eip.PortId != nil && *eip.PortId == loadbalancer.VipPortId {
		return getEipAddress(eip)
	}

	err = l.eipClient.Bind(eipID, loadbalancer.VipPortId)
	if err != nil {
		return "", err
	}

	eip, err = l.eipClient.Get(eipID)
	if err != nil {
		return "", err
	}

	return getEipAddress(eip)
}

func getEipAddress(eip *eipmodel.PublicipShowResp) (string, error) {
	if eip.PublicIpAddress == nil {
		return "", status.Errorf(codes.Internal, "rollback: error EIP address is empty, delete ELB instance")
	}
	return *eip.PublicIpAddress, nil
}

func (l *SharedLoadBalancer) createLoadbalancer(clusterName, subnetID string, service *v1.Service) (*elbmodel.LoadbalancerResp, error) {
	name := l.GetLoadBalancerName(context.TODO(), clusterName, service)
	provider := elbmodel.GetCreateLoadbalancerReqProviderEnum().VLB
	desc := fmt.Sprintf("Created by the ELB service(%s/%s) of the k8s cluster(%s).",
		service.Namespace, service.Name, clusterName)
	loadbalancer, err := l.sharedELBClient.CreateInstanceCompleted(&elbmodel.CreateLoadbalancerReq{
		Name:        &name,
		VipSubnetId: subnetID,
		Provider:    &provider,
		Description: &desc,
	})
	if err != nil {
		return nil, err
	}
	return loadbalancer, nil
}

// ensureHealthCheck add or update or remove health check
func (l *SharedLoadBalancer) ensureHealthCheck(loadbalancerID string, pool *elbmodel.PoolResp,
	port v1.ServicePort, service *v1.Service, _ *v1.Node) error {
	healthCheckOpts := getHealthCheckOptionFromAnnotation(service, l.loadbalancerOpts)
	monitorID := pool.HealthmonitorId
	klog.Infof("add or update or remove health check: %s : %#v", monitorID, healthCheckOpts)

	protocolStr := parseProtocol(service, port)
	// create health monitor
	if monitorID == "" && healthCheckOpts.Enable {
		_, err := l.createHealthMonitor(loadbalancerID, pool.Id, protocolStr, healthCheckOpts)
		return err
	}

	// update health monitor
	if monitorID != "" && healthCheckOpts.Enable {
		return l.updateHealthMonitor(monitorID, protocolStr, healthCheckOpts)
	}

	// delete health monitor
	if monitorID != "" && !healthCheckOpts.Enable {
		klog.Infof("Deleting health monitor %s for pool %s", monitorID, pool.Id)
		err := l.sharedELBClient.DeleteHealthMonitor(monitorID)
		if err != nil {
			return fmt.Errorf("failed to delete health monitor %s for pool %s, error: %v", monitorID, pool.Id, err)
		}
	}

	return nil
}

func (l *SharedLoadBalancer) updateHealthMonitor(id, protocol string, opts *config.HealthCheckOption) error {
	if protocol == ProtocolHTTPS || protocol == ProtocolTerminatedHTTPS {
		protocol = ProtocolHTTP
	} else if protocol == ProtocolUDP {
		protocol = ""
	}

	updateOpts := elbmodel.UpdateHealthmonitorReq{
		Timeout:    &opts.Timeout,
		Delay:      &opts.Delay,
		MaxRetries: &opts.MaxRetries,
	}

	if protocol != "" {
		updateOpts.Type = &protocol
	}

	return l.sharedELBClient.UpdateHealthMonitor(id, &updateOpts)
}

func (l *SharedLoadBalancer) createHealthMonitor(loadbalancerID, poolID, protocol string,
	opts *config.HealthCheckOption) (*elbmodel.HealthmonitorResp, error) {
	if protocol == ProtocolHTTPS || protocol == ProtocolTerminatedHTTPS {
		protocol = ProtocolHTTP
	} else if protocol == ProtocolUDP {
		protocol = "UDP_CONNECT"
	}

	protocolType := elbmodel.CreateHealthmonitorReqType{}
	if err := protocolType.UnmarshalJSON([]byte(protocol)); err != nil {
		return nil, err
	}

	monitor, err := l.sharedELBClient.CreateHealthMonitor(&elbmodel.CreateHealthmonitorReq{
		PoolId:     poolID,
		Type:       protocolType,
		Timeout:    opts.Timeout,
		Delay:      opts.Delay,
		MaxRetries: opts.MaxRetries,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating SharedLoadBalancer pool health monitor: %v", err)
	}

	loadbalancer, err := l.sharedELBClient.WaitStatusActive(loadbalancerID)
	if err != nil {
		return nil, fmt.Errorf("timeout when waiting for loadbalancer to be ACTIVE after creating member, "+
			"current provisioning status %s", loadbalancer.ProvisioningStatus)
	}
	return monitor, nil
}

// nolint: revive
func (l *SharedLoadBalancer) addOrRemoveMembers(loadbalancer *elbmodel.LoadbalancerResp, service *v1.Service, pool *elbmodel.PoolResp,
	svcPort v1.ServicePort, nodes []*v1.Node) error {

	members, err := l.sharedELBClient.ListMembers(&elbmodel.ListMembersRequest{PoolId: pool.Id})
	if err != nil {
		return err
	}

	existsMember := make(map[string]bool)
	for _, m := range members {
		existsMember[fmt.Sprintf("%s:%d", m.Address, m.ProtocolPort)] = true
	}

	nodeNameMapping := make(map[string]*v1.Node)
	for _, node := range nodes {
		nodeNameMapping[node.Name] = node
	}

	podList, err := l.listPodsBySelector(context.TODO(), service.Namespace, service.Spec.Selector)
	if err != nil {
		return err
	}
	for _, pod := range podList.Items {
		if !IsPodActive(pod) {
			klog.Errorf("Pod %s/%s is not activated skipping adding to ELB", pod.Namespace, pod.Name)
			continue
		}

		if pod.Status.HostIP == "" {
			klog.Errorf("Pod %s/%s is not scheduled, skipping adding to ELB", pod.Namespace, pod.Name)
			continue
		}

		node, ok := nodeNameMapping[pod.Spec.NodeName]
		if !ok {
			return fmt.Errorf("could not find the node where the Pod resides, Pod: %s/%s",
				pod.Namespace, pod.Spec.NodeName)
		}

		// address, err := getNodeAddress(node)
		address, portNum, err := l.getMemberIP(service, node, pod, svcPort)
		if err != nil {
			if common.IsNotFound(err) {
				// Node failure, do not create member
				klog.Warningf("Failed to create SharedLoadBalancer pool member for node %s: %v", node.Name, err)
				continue
			} else {
				return fmt.Errorf("error getting address for node %s: %v", node.Name, err)
			}
		}

		key := fmt.Sprintf("%s:%d", address, portNum)
		if existsMember[key] {
			klog.Infof("[addOrRemoveMembers] node already exists, skip adding, name: %s, address: %s, port: %d",
				node.Name, address, portNum)
			members = popMember(members, address, portNum)
			continue
		}

		klog.Infof("[addOrRemoveMembers] add node to pool, name: %s, address: %s, port: %d",
			node.Name, address, portNum)
		// Add a member to the pool.
		if err = l.addMember(service, loadbalancer.Id, pool.Id, svcPort, pod, node); err != nil {
			return err
		}
		existsMember[key] = true
	}

	// delete the remaining elements in members
	for _, member := range members {
		klog.Infof("[addOrRemoveMembers] remove node from pool, name: %s, address: %s, port: %d",
			member.Name, member.Address, member.ProtocolPort)
		err = l.deleteMember(loadbalancer.Id, pool.Id, member)
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *SharedLoadBalancer) getMemberIP(service *v1.Service, node *v1.Node, pod v1.Pod, svcPort v1.ServicePort) (string, int32, error) {
	if service.Spec.AllocateLoadBalancerNodePorts != nil && *service.Spec.AllocateLoadBalancerNodePorts {
		klog.Infof("add member using the Node's IP and port, service: %s/%s, port: %s ", service.Namespace, service.Name, svcPort.Name)

		address := ""
		if pod.Status.HostIP != "" {
			address = pod.Status.HostIP
		} else {
			addr, err := getNodeAddress(node)
			if err != nil {
				return "", 0, err
			}
			address = addr
		}

		address, err := l.getPrimaryIP(address)
		if err != nil {
			return "", 0, err
		}
		return address, svcPort.NodePort, nil
	}

	if service.Spec.AllocateLoadBalancerNodePorts != nil && !*service.Spec.AllocateLoadBalancerNodePorts {
		klog.Infof("add member using the Pod's IP and port, service: %s/%s, port: %s ", service.Namespace, service.Name, svcPort.Name)
		// get IP and port from Pod
		if svcPort.TargetPort.Type == intstr.Int {
			klog.V(6).Infof("targetPort is a number, service: %s/%s, port: %s ", service.Namespace, service.Name, svcPort.Name)
			return pod.Status.PodIP, svcPort.TargetPort.IntVal, nil
		}

		klog.V(6).Infof("targetPort is a name, service: %s/%s, port: %s ", service.Namespace, service.Name, svcPort.Name)
		for _, c := range pod.Spec.Containers {
			for _, p := range c.Ports {
				if p.Name == svcPort.TargetPort.StrVal && string(p.Protocol) == string(svcPort.Protocol) {
					return pod.Status.PodIP, p.ContainerPort, nil
				}
			}
		}
	}
	return "", 0, fmt.Errorf("not found member IP and port")
}

func (l *SharedLoadBalancer) addMember(service *v1.Service, elbID, poolID string, svcPort v1.ServicePort, pod v1.Pod, node *v1.Node) error {
	klog.Infof("Add a member(%s) to pool %s", node.Name, poolID)
	address, port, err := l.getMemberIP(service, node, pod, svcPort)
	if err != nil {
		return err
	}

	subnetID, err := l.getNodeSubnetIDByHostIP(address)
	if err != nil {
		return err
	}

	req := elbmodel.CreateMemberReq{
		ProtocolPort: port,
		SubnetId:     subnetID,
		Address:      address,
	}
	_, err = l.sharedELBClient.AddMember(poolID, &req)
	if err != nil {
		return fmt.Errorf("error creating SharedLoadBalancer pool member for node: %s, %v, options: %s",
			node.Name, err, utils.ToString(req))
	}

	loadbalancer, err := l.sharedELBClient.WaitStatusActive(elbID)
	if err != nil {
		return fmt.Errorf("timeout when waiting for loadbalancer to be ACTIVE after adding members, current status %s",
			loadbalancer.ProvisioningStatus)
	}

	return nil
}

func (l *SharedLoadBalancer) deleteMember(elbID string, poolID string, member elbmodel.MemberResp) error {
	klog.V(4).Infof("Deleting obsolete member %s for pool %s address %s", member.Id, poolID, member.Address)
	err := l.sharedELBClient.DeleteMember(poolID, member.Id)
	if err != nil && !common.IsNotFound(err) {
		return fmt.Errorf("error deleting obsolete member %s for pool %s address %s: %v",
			poolID, member.Id, member.Address, err)
	}
	loadbalancer, err := l.sharedELBClient.WaitStatusActive(elbID)
	if err != nil {
		return fmt.Errorf("timeout when waiting for loadbalancer to be ACTIVE after creating member, "+
			"current provisioning status %s", loadbalancer.ProvisioningStatus)
	}
	return nil
}

func (l *SharedLoadBalancer) getPool(elbID, listenerID string) (*elbmodel.PoolResp, error) {
	pools, err := l.sharedELBClient.ListPools(&elbmodel.ListPoolsRequest{
		LoadbalancerId: &elbID,
	})
	if err != nil {
		return nil, err
	}

	for _, p := range pools {
		for _, lis := range p.Listeners {
			if lis.Id == listenerID {
				return &p, nil
			}
		}
	}
	return nil, status.Errorf(codes.NotFound, "not found pool matched ListenerId: %s, ELB ID: %s", listenerID, elbID)
}

func (l *SharedLoadBalancer) getSessionAffinity(service *v1.Service) *elbmodel.SessionPersistence {
	globalOpts := l.loadbalancerOpts
	sessionMode := getStringFromSvsAnnotation(service, ElbSessionAffinityFlag, globalOpts.SessionAffinityFlag)
	if sessionMode == "" || sessionMode == "off" {
		return nil
	}

	persistence := globalOpts.SessionAffinityOption

	opts := getStringFromSvsAnnotation(service, ElbSessionAffinityOption, "")
	if opts == "" {
		klog.V(4).Infof("[DEBUG] SessionAffinityOption is empty, use default: %#v", persistence)
		return &persistence
	}

	err := json.Unmarshal([]byte(opts), &persistence)
	if err != nil {
		klog.Warningf("error parsing \"kubernetes.io/elb.session-affinity-option\": %s, ignore options: %s",
			err, opts)
	}
	printSessionAffinity(service, persistence)
	return &persistence
}

func printSessionAffinity(service *v1.Service, per elbmodel.SessionPersistence) {
	cookieName := ""
	if per.CookieName != nil {
		cookieName = *per.CookieName
	}
	timeout := int32(0)
	if per.PersistenceTimeout != nil {
		timeout = *per.PersistenceTimeout
	}

	klog.V(4).Infof("[DEBUG] service name: %s/%s, SessionAffinity: { mode: %s, CookieName: %s, "+
		"PersistenceTimeout: %d min }", service.Namespace, service.Name, per.Type.Value(), cookieName, timeout)
}

func (l *SharedLoadBalancer) createPool(listener *elbmodel.ListenerResp, service *v1.Service) (*elbmodel.PoolResp, error) {
	lbAlgorithm := getStringFromSvsAnnotation(service, ElbAlgorithm, l.loadbalancerOpts.LBAlgorithm)
	persistence := l.getSessionAffinity(service)

	protocolStr := listener.Protocol.Value()
	if protocolStr == ProtocolHTTPS || protocolStr == ProtocolTerminatedHTTPS {
		protocolStr = ProtocolHTTP
	}
	protocol := elbmodel.CreatePoolReqProtocol{}
	if err := protocol.UnmarshalJSON([]byte(protocolStr)); err != nil {
		return nil, err
	}

	name := utils.CutString(fmt.Sprintf("sg_%s", listener.Name), maxServerGroupNameLength)
	return l.sharedELBClient.CreatePool(&elbmodel.CreatePoolReq{
		Name:               &name,
		Protocol:           protocol,
		LbAlgorithm:        lbAlgorithm,
		ListenerId:         &listener.Id,
		SessionPersistence: persistence,
	})
}

func popMember(members []elbmodel.MemberResp, addr string, port int32) []elbmodel.MemberResp {
	for i, m := range members {
		if m.Address == addr && m.ProtocolPort == port {
			members[i] = members[len(members)-1]
			members = members[:len(members)-1]
		}
	}
	return members
}

func popListener(arr []elbmodel.ListenerResp, id string) []elbmodel.ListenerResp {
	for i, lis := range arr {
		if lis.Id == id {
			arr[i] = arr[len(arr)-1]
			arr = arr[:len(arr)-1]
			break
		}
	}
	return arr
}

func (l *SharedLoadBalancer) deleteListeners(elbID string, listeners []elbmodel.ListenerResp) error {
	errs := make([]error, 0)
	for _, lis := range listeners {
		pool, err := l.getPool(elbID, lis.Id)
		if err != nil && !common.IsNotFound(err) {
			errs = append(errs, err)
			continue
		}
		if err == nil {
			delErrs := l.deletePool(pool)
			if len(delErrs) > 0 {
				errs = append(errs, delErrs...)
			}
		}
		// delete ELB listener
		if err = l.sharedELBClient.DeleteListener(elbID, lis.Id); err != nil && !common.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete ELB listener %s : %s ", lis.Id, err))
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("failed to delete listeners: %s", errors.NewAggregate(errs))
	}

	return nil
}

func (l *SharedLoadBalancer) deletePool(pool *elbmodel.PoolResp) []error {
	errs := make([]error, 0)
	// delete all members of pool
	if err := l.sharedELBClient.DeleteAllPoolMembers(pool.Id); err != nil {
		errs = append(errs, err)
	}
	// delete the pool monitor if exists
	if err := l.sharedELBClient.DeleteHealthMonitor(pool.HealthmonitorId); err != nil && !common.IsNotFound(err) {
		errs = append(errs, err)
	}
	// delete ELB listener pool
	if err := l.sharedELBClient.DeletePool(pool.Id); err != nil && !common.IsNotFound(err) {
		errs = append(errs, err)
	}
	return errs
}

func (l *SharedLoadBalancer) createListener(loadbalancerID string, service *v1.Service, port v1.ServicePort) (
	*elbmodel.ListenerResp, error) {
	xForwardFor := getBoolFromSvsAnnotation(service, ElbXForwardedHost, false)
	createOpt := &elbmodelv3.CreateListenerOption{
		LoadbalancerId: loadbalancerID,
		ProtocolPort:   port.Port,
		InsertHeaders:  &elbmodelv3.ListenerInsertHeaders{XForwardedHost: &xForwardFor},
	}

	protocol := parseProtocol(service, port)
	if protocol == ProtocolTerminatedHTTPS {
		defaultTLSContainerRef := getStringFromSvsAnnotation(service, DefaultTLSContainerRef, "")
		createOpt.DefaultTlsContainerRef = &defaultTLSContainerRef
	} else if xForwardFor {
		protocol = ProtocolHTTP
	}
	createOpt.Protocol = protocol
	name := utils.CutString(fmt.Sprintf("%s_%s_%v", service.Name, protocol, port.Port), defaultMaxNameLength)
	createOpt.Name = &name

	// Set timeout parameters
	globalOpts := l.loadbalancerOpts
	if timeout := getIntFromSvsAnnotation(service, ElbIdleTimeout, globalOpts.IdleTimeout); timeout != 0 {
		createOpt.KeepaliveTimeout = pointer.Int32(int32(timeout))
	}

	if protocol == ProtocolHTTP || protocol == ProtocolTerminatedHTTPS {
		if timeout := getIntFromSvsAnnotation(service, ElbRequestTimeout, globalOpts.RequestTimeout); timeout != 0 {
			createOpt.ClientTimeout = pointer.Int32(int32(timeout))
		}
		if timeout := getIntFromSvsAnnotation(service, ElbResponseTimeout, globalOpts.ResponseTimeout); timeout != 0 {
			createOpt.MemberTimeout = pointer.Int32(int32(timeout))
		}
	}

	if protocol == ProtocolTCP || protocol == ProtocolUDP {
		// TCP or UDP listeners transparent_client_ip_enable can be true or false.
		transparentClientIPEnable := getBoolFromSvsAnnotation(service, ElbEnableTransparentClientIP,
			l.loadbalancerOpts.EnableTransparentClientIP)
		createOpt.TransparentClientIpEnable = &transparentClientIPEnable
	}

	listener, err := l.dedicatedELBClient.CreateListener(createOpt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create listener for loadbalancer %s: %v",
			loadbalancerID, err)
	}

	return convertToListenerV2(listener)
}

func (l *SharedLoadBalancer) updateListener(listener *elbmodel.ListenerResp, service *v1.Service) error {
	name := fmt.Sprintf("%s_%s_%v", service.Name, listener.Protocol.Value(), listener.ProtocolPort)
	name = utils.CutString(name, defaultMaxNameLength)
	xForwardFor := getBoolFromSvsAnnotation(service, ElbXForwardedHost, false)
	updateOpt := &elbmodelv3.UpdateListenerOption{
		Name:          &name,
		InsertHeaders: &elbmodelv3.ListenerInsertHeaders{XForwardedHost: &xForwardFor},
	}

	// Set timeout parameters
	globalOpts := l.loadbalancerOpts
	if timeout := getIntFromSvsAnnotation(service, ElbIdleTimeout, globalOpts.IdleTimeout); timeout != 0 {
		updateOpt.KeepaliveTimeout = pointer.Int32(int32(timeout))
	}
	if listener.Protocol.Value() == ProtocolHTTP || listener.Protocol.Value() == ProtocolTerminatedHTTPS {
		if timeout := getIntFromSvsAnnotation(service, ElbRequestTimeout, globalOpts.RequestTimeout); timeout != 0 {
			updateOpt.ClientTimeout = pointer.Int32(int32(timeout))
		}
		if timeout := getIntFromSvsAnnotation(service, ElbResponseTimeout, globalOpts.ResponseTimeout); timeout != 0 {
			updateOpt.MemberTimeout = pointer.Int32(int32(timeout))
		}
	}

	if listener.Protocol.Value() == ProtocolTCP || listener.Protocol.Value() == ProtocolUDP {
		// TCP or UDP listeners transparent_client_ip_enable can be true or false.
		transparentClientIPEnable := getBoolFromSvsAnnotation(service, ElbEnableTransparentClientIP,
			l.loadbalancerOpts.EnableTransparentClientIP)
		updateOpt.TransparentClientIpEnable = &transparentClientIPEnable
	}

	err := l.dedicatedELBClient.UpdateListener(listener.Id, updateOpt)
	if err != nil {
		return err
	}

	klog.Infof("Listener updated, id: %s, name: %s", listener.Id, listener.Name)
	return nil
}

func convertToListenerV2(listener *elbmodelv3.Listener) (*elbmodel.ListenerResp, error) {
	protocol := elbmodel.ListenerRespProtocol{}
	err := protocol.UnmarshalJSON([]byte(listener.Protocol))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to convert Listener V3 to V2, %s", err)
	}

	loadBalancers := make([]elbmodel.ResourceList, 0)
	for _, lb := range listener.Loadbalancers {
		if lb.Id == nil {
			klog.Warningf("The ELB Listener \"Loadbalancers\" field is empty, listenerID: %s", listener.Id)
		}
		loadBalancers = append(loadBalancers, elbmodel.ResourceList{
			Id: *lb.Id,
		})
	}

	tags := make([]string, 0)
	for _, t := range listener.Tags {
		val := ""
		if t.Value != nil {
			val = *t.Value
		}
		tags = append(tags, fmt.Sprintf("%s=%s", *t.Key, val))
	}
	return &elbmodel.ListenerResp{
		Id:                      listener.Id,
		TenantId:                "",
		Name:                    listener.Name,
		Description:             listener.Description,
		AdminStateUp:            listener.AdminStateUp,
		Loadbalancers:           loadBalancers,
		ConnectionLimit:         listener.ConnectionLimit,
		Http2Enable:             listener.Http2Enable,
		Protocol:                protocol,
		ProtocolPort:            listener.ProtocolPort,
		DefaultPoolId:           listener.DefaultPoolId,
		DefaultTlsContainerRef:  listener.DefaultTlsContainerRef,
		ClientCaTlsContainerRef: listener.ClientCaTlsContainerRef,
		SniContainerRefs:        listener.SniContainerRefs,
		Tags:                    tags,
		CreatedAt:               listener.CreatedAt,
		UpdatedAt:               listener.UpdatedAt,
		InsertHeaders: &elbmodel.InsertHeader{
			XForwardedHost:  listener.InsertHeaders.XForwardedHost,
			XForwardedELBIP: listener.InsertHeaders.XForwardedELBIP,
		},
		ProjectId:        listener.ProjectId,
		TlsCiphersPolicy: listener.TlsCiphersPolicy,
	}, nil
}

func (*SharedLoadBalancer) filterListenerByPort(listeners []elbmodel.ListenerResp, service *v1.Service, port v1.ServicePort) *elbmodel.ListenerResp {
	protocol := parseProtocol(service, port)
	for _, listener := range listeners {
		if listener.Protocol.Value() == protocol && listener.ProtocolPort == port.Port {
			return &listener
		}
	}

	return nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
func (l *SharedLoadBalancer) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.Infof("UpdateLoadBalancer: called with service %s/%s, node: %d", service.Namespace, service.Name, len(nodes))
	if !l.isSupportedSvc(service) {
		return cloudprovider.ImplementedElsewhere
	}

	// get exits or create a new ELB instance
	loadbalancer, err := l.getLoadBalancerInstance(ctx, clusterName, service)
	if err != nil {
		return err
	}

	// query ELB listeners list
	listeners, err := l.sharedELBClient.ListListeners(&elbmodel.ListListenersRequest{LoadbalancerId: &loadbalancer.Id})
	if err != nil {
		return err
	}

	for _, port := range service.Spec.Ports {
		listener := l.filterListenerByPort(listeners, service, port)
		if listener == nil {
			return status.Errorf(codes.Unavailable, "error, can not find a listener matching %s:%v",
				port.Protocol, port.Port)
		}

		// query pool or create pool
		pool, err := l.getPool(loadbalancer.Id, listener.Id)
		if err != nil && common.IsNotFound(err) {
			pool, err = l.createPool(listener, service)
		}
		if err != nil {
			return err
		}

		// add new members and remove the obsolete members.
		if err = l.addOrRemoveMembers(loadbalancer, service, pool, port, nodes); err != nil {
			return err
		}

		// add or remove health monitor
		if err = l.ensureHealthCheck(loadbalancer.Id, pool, port, service, nodes[0]); err != nil {
			return err
		}
	}
	return nil
}

// EnsureLoadBalancerDeleted deletes the specified load balancer
func (l *SharedLoadBalancer) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.Infof("EnsureLoadBalancerDeleted: called with service %s/%s", service.Namespace, service.Name)

	loadBalancer, err := l.getLoadBalancerInstance(ctx, clusterName, service)
	if err != nil {
		if common.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err != nil {
		return err
	}

	specifiedID := getStringFromSvsAnnotation(service, ElbID, "")
	if specifiedID != "" {
		err = l.deleteListener(loadBalancer, service)
	} else {
		err = l.deleteELBInstance(loadBalancer, service)
	}

	if err != nil {
		return err
	}
	return nil
}

func (l *SharedLoadBalancer) deleteListener(loadBalancer *elbmodel.LoadbalancerResp, service *v1.Service) error {
	// query ELB listeners list
	listenerArr, err := l.sharedELBClient.ListListeners(&elbmodel.ListListenersRequest{
		LoadbalancerId: &loadBalancer.Id,
	})
	if err != nil {
		return err
	}

	listenersMatched := make([]elbmodel.ListenerResp, 0)
	for _, port := range service.Spec.Ports {
		listener := l.filterListenerByPort(listenerArr, service, port)
		if listener != nil {
			listenersMatched = append(listenersMatched, *listener)
		}
	}
	return l.deleteListeners(loadBalancer.Id, listenersMatched)
}

func (l *SharedLoadBalancer) deleteELBInstance(loadBalancer *elbmodel.LoadbalancerResp, service *v1.Service) error {
	// query ELB listeners list
	listenerArr, err := l.sharedELBClient.ListListeners(&elbmodel.ListListenersRequest{
		LoadbalancerId: &loadBalancer.Id,
	})
	if err != nil {
		return err
	}

	if err = l.deleteListeners(loadBalancer.Id, listenerArr); err != nil {
		return err
	}

	eipID := getStringFromSvsAnnotation(service, ElbEipID, "")
	keepEip := getBoolFromSvsAnnotation(service, ELBKeepEip, l.loadbalancerOpts.KeepEIP)
	if err = unbindEIP(l.eipClient, loadBalancer.VipPortId, eipID, keepEip); err != nil {
		return err
	}
	return l.sharedELBClient.DeleteInstance(loadBalancer.Id)
}

// nolint: revive
func unbindEIP(eipClient *wrapper.EIpClient, vipPortID, eipID string, keepEIP bool) error {
	if eipID == "" {
		ips, err := eipClient.List(&eipmodel.ListPublicipsRequest{
			PortId: &[]string{vipPortID},
		})

		if err != nil {
			return err
		}
		if len(ips) == 0 {
			return nil
		}
		eipID = *ips[0].Id
	}

	err := eipClient.Unbind(eipID)
	if err != nil {
		return err
	}

	if keepEIP {
		return nil
	}

	err = eipClient.Delete(eipID)
	if err != nil {
		return err
	}
	return nil
}

func getNodeAddress(node *v1.Node) (string, error) {
	addresses := node.Status.Addresses
	if len(addresses) == 0 {
		return "", status.Errorf(codes.NotFound, "error, current node do not have addresses, nodeName: %s",
			node.Name)
	}

	for _, addr := range addresses {
		if _, ok := allowedIPTypes[addr.Type]; ok {
			return addr.Address, nil
		}
	}
	return "", status.Errorf(codes.NotFound, "error, current node do not have any valid addresses, nodeName: %s",
		node.Name)
}

func getHealthCheckOptionFromAnnotation(service *v1.Service, opts *config.LoadBalancerOptions) *config.HealthCheckOption {
	checkOpts := opts.HealthCheckOption

	healthCheckFlag := getStringFromSvsAnnotation(service, ElbHealthCheckFlag, opts.HealthCheckFlag)
	if healthCheckFlag == "" || healthCheckFlag == "on" {
		checkOpts.Enable = true
	}

	str := getStringFromSvsAnnotation(service, ElbHealthCheckOptions, "")
	if str == "" {
		return &checkOpts
	}
	if err := json.Unmarshal([]byte(str), &checkOpts); err != nil {
		klog.Errorf("error parsing health check options: %s, using default", err)
	}
	return &checkOpts
}

func (l *SharedLoadBalancer) createEIP(service *v1.Service) (string, error) {
	opts, err := parseEIPAutoCreateOptions(service)
	if err != nil || opts == nil {
		return "", err
	}

	shareType := eipmodel.CreatePublicipBandwidthOptionShareType{}
	err = shareType.UnmarshalJSON([]byte(opts.ShareType))
	if err != nil {
		return "", err
	}

	chargeModel := &eipmodel.CreatePublicipBandwidthOptionChargeMode{}
	err = chargeModel.UnmarshalJSON([]byte(opts.ChargeMode))
	if err != nil {
		return "", err
	}

	name := fmt.Sprintf("%s_%s", service.Namespace, service.Name)
	eip, err := l.eipClient.Create(&eipmodel.CreatePublicipRequestBody{
		Bandwidth: &eipmodel.CreatePublicipBandwidthOption{
			Name:       &name,
			Id:         utils.StringToPtr(opts.ShareID),
			Size:       &opts.BandwidthSize,
			ShareType:  shareType,
			ChargeMode: chargeModel,
		},
		Publicip: &eipmodel.CreatePublicipOption{Type: opts.IPType},
	})
	if err != nil {
		return "", err
	}

	return *eip.Id, nil
}

type CreateEIPOptions struct {
	BandwidthSize int32  `json:"bandwidth_size"`
	ShareType     string `json:"share_type"`
	ShareID       string `json:"share_id"`
	ChargeMode    string `json:"charge_mode"`

	IPType string `json:"ip_type"`
}

func parseEIPAutoCreateOptions(service *v1.Service) (*CreateEIPOptions, error) {
	str := getStringFromSvsAnnotation(service, AutoCreateEipOptions, "")
	if str == "" {
		return nil, nil
	}

	opts := &CreateEIPOptions{}
	err := json.Unmarshal([]byte(str), opts)
	if opts.ChargeMode == "" {
		opts.ChargeMode = "traffic"
	}
	return opts, err
}

func parseProtocol(service *v1.Service, port v1.ServicePort) string {
	xForwardFor := getBoolFromSvsAnnotation(service, ElbXForwardedHost, false)

	protocol := string(port.Protocol)
	defaultTLSContainerRef := getStringFromSvsAnnotation(service, DefaultTLSContainerRef, "")
	if defaultTLSContainerRef != "" {
		protocol = ProtocolTerminatedHTTPS
	} else if xForwardFor {
		protocol = ProtocolHTTP
	}
	return protocol
}

func getStringFromSvsAnnotation(service *v1.Service, key string, defaultSetting string) string {
	if annotationValue, ok := service.Annotations[key]; ok {
		klog.V(4).Infof("Found annotation: %v = %v", key, annotationValue)
		return annotationValue
	}
	klog.V(4).Infof("Annotation %s is empty, use default value: %v", key, defaultSetting)
	return defaultSetting
}

func getBoolFromSvsAnnotation(service *v1.Service, key string, defaultVal bool) bool {
	value, ok := service.Annotations[key]
	if !ok {
		return defaultVal
	}

	rstValue := false
	switch value {
	case "true":
		rstValue = true
	case "false":
		rstValue = false
	default:
		klog.Warningf("unknown %s annotation: %v, specify \"true\" or \"false\" ", key, value)
		rstValue = defaultVal
	}
	return rstValue
}

func getIntFromSvsAnnotation(service *v1.Service, key string, defaultVal int) int {
	if annotationValue, ok := service.Annotations[key]; ok {
		klog.V(4).Infof("Found annotation: %v = %v", key, annotationValue)
		val, err := strconv.Atoi(annotationValue)
		if err != nil {
			return defaultVal
		}
		return val
	}
	klog.V(4).Infof("Annotation %s is empty, use default value: %v", key, defaultVal)
	return defaultVal
}
