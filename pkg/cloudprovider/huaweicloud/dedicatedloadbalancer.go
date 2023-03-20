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

package huaweicloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	eipmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eip/v2/model"
	elbmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v3/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/common"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/utils"
)

const (
	ElbEnableCrossVpc    = "kubernetes.io/elb.enable-cross-vpc"
	ElbL4FlavorID        = "kubernetes.io/elb.l4-flavor-id"
	ElbL7FlavorID        = "kubernetes.io/elb.l7-flavor-id"
	ElbAvailabilityZones = "kubernetes.io/elb.availability-zones"

	ElbEnableTransparentClientIP = "kubernetes.io/elb.enable-transparent-client-ip"
)

type DedicatedLoadBalancer struct {
	Basic
}

func (d *DedicatedLoadBalancer) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (
	*v1.LoadBalancerStatus, bool, error) {

	klog.Infof("GetLoadBalancer: called with service %s/%s", service.Namespace, service.Name)
	loadbalancer, err := d.getLoadBalancerInstance(ctx, clusterName, service)
	if err != nil {
		if common.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	portID := loadbalancer.VipPortId
	if portID == "" {
		return nil, false, status.Errorf(codes.Unavailable, "The ELB %s VipPortId is empty, "+
			"and the instance is unavailable", d.GetLoadBalancerName(ctx, clusterName, service))
	}
	ingressIP := loadbalancer.VipAddress

	ips, err := d.eipClient.List(&eipmodel.ListPublicipsRequest{PortId: &[]string{portID}})
	if err != nil {
		return nil, false, status.Errorf(codes.Unavailable, "error querying EIP list base on PortId (%s): %s",
			portID, err)
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

func (d *DedicatedLoadBalancer) getLoadBalancerInstance(ctx context.Context, clusterName string, service *v1.Service,
) (*elbmodel.LoadBalancer, error) {
	if id := getStringFromSvsAnnotation(service, ElbID, ""); id != "" {
		return d.dedicatedELBClient.GetInstance(id)
	}

	name := d.GetLoadBalancerName(ctx, clusterName, service)
	names := []string{name}
	list, err := d.dedicatedELBClient.ListInstances(&elbmodel.ListLoadBalancersRequest{Name: &names})
	if err != nil {
		return nil, err
	}

	count := len(list)
	if count == 0 {
		return nil, status.Errorf(codes.NotFound, "not found dedicated ELB instance %s", name)
	}
	if count != 1 {
		return nil, status.Errorf(codes.Unavailable, "error, found %d dedicated ELB named %s, "+
			"make sure there is only one", len(list), name)
	}
	return &list[0], nil
}

func (d *DedicatedLoadBalancer) GetLoadBalancerName(_ context.Context, clusterName string, service *v1.Service) string {
	klog.Infof("GetLoadBalancerName: called with service %s/%s", service.Namespace, service.Name)
	name := fmt.Sprintf("k8s_service_%s_%s_%s", clusterName, service.Namespace, service.Name)
	return utils.CutString(name, defaultMaxNameLength)
}

func (d *DedicatedLoadBalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	klog.Infof("EnsureLoadBalancer: called with service %s/%s, node: %d",
		service.Namespace, service.Name, len(nodes))

	if err := ensureLoadBalancerValidation(service, nodes); err != nil {
		return nil, err
	}

	// get exits or create a new ELB instance
	loadbalancer, err := d.getLoadBalancerInstance(ctx, clusterName, service)
	specifiedID := getStringFromSvsAnnotation(service, ElbID, "")
	if common.IsNotFound(err) && specifiedID != "" {
		return nil, err
	}
	if err != nil && common.IsNotFound(err) {
		subnetID, e := d.getSubnetID(service, nodes[0])
		if e != nil {
			return nil, e
		}
		loadbalancer, err = d.createLoadbalancer(clusterName, subnetID, service)
	}
	if err != nil {
		return nil, err
	}

	// query ELB listeners list
	loadbalancerIDs := []string{loadbalancer.Id}
	listeners, err := d.dedicatedELBClient.ListListeners(&elbmodel.ListListenersRequest{
		LoadbalancerId: &loadbalancerIDs,
	})
	if err != nil {
		return nil, err
	}

	for _, port := range service.Spec.Ports {
		listener := d.filterListenerByPort(listeners, service, port)
		// add or update listener
		if listener == nil {
			listener, err = d.createListener(loadbalancer.Id, service, port)
		} else {
			err = d.updateListener(listener, service, port)
		}
		if err != nil {
			return nil, err
		}

		listeners = d.popListener(listeners, listener.Id)

		// query pool or create pool
		pool, err := d.getPool(loadbalancer.Id, listener.Id)
		if err != nil && common.IsNotFound(err) {
			pool, err = d.createPool(listener, service)
		}
		if err != nil {
			return nil, err
		}

		// add new members and remove the obsolete members.
		if err = d.addOrRemoveMembers(loadbalancer, service, pool, port, nodes); err != nil {
			return nil, err
		}

		// add or remove health monitor
		if err = d.addOrRemoveHealthMonitor(loadbalancer.Id, pool, port, service); err != nil {
			return nil, err
		}
	}

	if specifiedID == "" {
		// All remaining listeners are obsolete, delete them
		err = d.deleteListeners(loadbalancer.Id, listeners)
		if err != nil {
			return nil, err
		}
	}

	ingressIP := loadbalancer.VipAddress

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{{IP: ingressIP}},
	}, nil
}

func (d *DedicatedLoadBalancer) createLoadbalancer(clusterName, subnetID string, service *v1.Service) (*elbmodel.LoadBalancer, error) {
	name := d.GetLoadBalancerName(context.TODO(), clusterName, service)
	desc := fmt.Sprintf("Created by the ELB service(%s/%s) of the k8s cluster(%s).",
		service.Namespace, service.Name, clusterName)

	azStr := getStringFromSvsAnnotation(service, ElbAvailabilityZones, "")
	if azStr == "" {
		return nil, status.Errorf(codes.InvalidArgument,
			"Invalid argument, annotation \"kubernetes.io/elb.availability-zones\" cannot be empty")
	}
	availabilityZoneList := strings.Split(azStr, ";")

	createOpt := &elbmodel.CreateLoadBalancerOption{
		Name:                 &name,
		AvailabilityZoneList: availabilityZoneList,
		VipSubnetCidrId:      &subnetID,
		Provider:             pointer.String("vlb"),
		Description:          &desc,
	}
	enableCrossVpc := getBoolFromSvsAnnotation(service, ElbEnableCrossVpc, d.loadbalancerOpts.EnableCrossVpc)
	if enableCrossVpc {
		createOpt.IpTargetEnable = &enableCrossVpc
	}
	if l4FlavorID := getStringFromSvsAnnotation(service, ElbL4FlavorID, d.loadbalancerOpts.L4FlavorID); l4FlavorID != "" {
		createOpt.L4FlavorId = &l4FlavorID
	}
	if l7FlavorID := getStringFromSvsAnnotation(service, ElbL7FlavorID, d.loadbalancerOpts.L7FlavorID); l7FlavorID != "" {
		createOpt.L7FlavorId = &l7FlavorID
	}

	// eip
	eipID := getStringFromSvsAnnotation(service, ElbEipID, "")
	if eipID != "" {
		publicIPIDs := []string{eipID}
		createOpt.PublicipIds = &publicIPIDs
	} else {
		// use auto create EIP options
		eipCreateOpts, err := d.parsePublicIP(service)
		if err != nil {
			return nil, err
		}
		createOpt.Publicip = eipCreateOpts
	}

	loadbalancer, err := d.dedicatedELBClient.CreateInstanceCompleted(createOpt)
	if err != nil {
		return nil, err
	}
	return loadbalancer, nil
}

func (d *DedicatedLoadBalancer) parsePublicIP(service *v1.Service) (*elbmodel.CreateLoadBalancerPublicIpOption, error) {
	eipOpt, err := parseEIPAutoCreateOptions(service)
	if err != nil {
		return nil, err
	}

	if eipOpt == nil {
		return nil, nil
	}
	publicIP := &elbmodel.CreateLoadBalancerPublicIpOption{
		NetworkType: eipOpt.IPType,
	}
	if eipOpt.BandwidthSize != 0 {
		shareType := &elbmodel.CreateLoadBalancerBandwidthOptionShareType{}
		if err = shareType.UnmarshalJSON([]byte(eipOpt.ShareType)); err != nil {
			return nil, err
		}

		name := fmt.Sprintf("%s_%s", service.Namespace, service.Name)
		publicIP.Bandwidth = &elbmodel.CreateLoadBalancerBandwidthOption{
			Name:      &name,
			Size:      &eipOpt.BandwidthSize,
			ShareType: shareType,
		}
	}
	if eipOpt.ShareID != "" {
		publicIP.Bandwidth = &elbmodel.CreateLoadBalancerBandwidthOption{
			Id: &eipOpt.ShareID,
		}
	}
	return publicIP, nil
}

func (d *DedicatedLoadBalancer) filterListenerByPort(listeners []elbmodel.Listener, service *v1.Service,
	port v1.ServicePort) *elbmodel.Listener {
	protocol := parseProtocol(service, port)
	for _, listener := range listeners {
		if listener.Protocol == protocol && listener.ProtocolPort == port.Port {
			return &listener
		}
	}

	return nil
}

func (d *DedicatedLoadBalancer) createListener(loadbalancerID string, service *v1.Service, port v1.ServicePort,
) (*elbmodel.Listener, error) {
	xForwardFor := getBoolFromSvsAnnotation(service, ElbXForwardedHost, false)
	name := utils.CutString(fmt.Sprintf("%s_%s_%v", service.Name, port.Protocol, port.Port), defaultMaxNameLength)

	createOpt := &elbmodel.CreateListenerOption{
		Name:           &name,
		LoadbalancerId: loadbalancerID,
		ProtocolPort:   port.Port,
		InsertHeaders:  &elbmodel.ListenerInsertHeaders{XForwardedHost: &xForwardFor},
	}

	protocol := parseProtocol(service, port)
	if protocol == ProtocolTerminatedHTTPS {
		defaultTLSContainerRef := getStringFromSvsAnnotation(service, DefaultTLSContainerRef, "")
		createOpt.DefaultTlsContainerRef = &defaultTLSContainerRef
	} else if xForwardFor {
		protocol = ProtocolHTTP
	}
	createOpt.Protocol = protocol

	transparentClientIPEnable := getBoolFromSvsAnnotation(service, ElbEnableTransparentClientIP,
		d.loadbalancerOpts.EnableTransparentClientIP)
	if transparentClientIPEnable {
		createOpt.TransparentClientIpEnable = &transparentClientIPEnable
	}

	if timeout := getIntFromSvsAnnotation(service, ElbIdleTimeout, d.loadbalancerOpts.IdleTimeout); timeout != 0 {
		createOpt.KeepaliveTimeout = pointer.Int32(int32(timeout))
	}

	if protocol == ProtocolHTTP || protocol == ProtocolTerminatedHTTPS {
		if timeout := getIntFromSvsAnnotation(service, ElbRequestTimeout, d.loadbalancerOpts.RequestTimeout); timeout != 0 {
			createOpt.ClientTimeout = pointer.Int32(int32(timeout))
		}
		if timeout := getIntFromSvsAnnotation(service, ElbResponseTimeout, d.loadbalancerOpts.ResponseTimeout); timeout != 0 {
			createOpt.MemberTimeout = pointer.Int32(int32(timeout))
		}
	}

	listener, err := d.dedicatedELBClient.CreateListener(createOpt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create listener for loadbalancer %s: %v",
			loadbalancerID, err)
	}

	return listener, nil
}

func (d *DedicatedLoadBalancer) updateListener(listener *elbmodel.Listener, service *v1.Service, port v1.ServicePort) error {
	xForwardFor := getBoolFromSvsAnnotation(service, ElbXForwardedHost, false)
	name := utils.CutString(fmt.Sprintf("%s_%s_%v", service.Name, port.Protocol, port.Port), defaultMaxNameLength)

	updateOpts := &elbmodel.UpdateListenerOption{
		Name: &name,
	}

	protocol := parseProtocol(service, port)

	transparentClientIPEnable := getBoolFromSvsAnnotation(service, ElbEnableTransparentClientIP,
		d.loadbalancerOpts.EnableTransparentClientIP)
	if transparentClientIPEnable {
		updateOpts.TransparentClientIpEnable = &transparentClientIPEnable
	} else if protocol == ProtocolUDP || protocol == ProtocolTCP {
		updateOpts.TransparentClientIpEnable = &transparentClientIPEnable
	}

	if timeout := getIntFromSvsAnnotation(service, ElbIdleTimeout, d.loadbalancerOpts.IdleTimeout); timeout != 0 {
		updateOpts.KeepaliveTimeout = pointer.Int32(int32(timeout))
	}

	if protocol == ProtocolTerminatedHTTPS {
		defaultTLSContainerRef := getStringFromSvsAnnotation(service, DefaultTLSContainerRef, "")
		updateOpts.DefaultTlsContainerRef = &defaultTLSContainerRef
	} else if xForwardFor {
		protocol = ProtocolHTTP
	}

	if protocol == ProtocolHTTP || protocol == ProtocolTerminatedHTTPS {
		if timeout := getIntFromSvsAnnotation(service, ElbRequestTimeout, d.loadbalancerOpts.RequestTimeout); timeout != 0 {
			updateOpts.ClientTimeout = pointer.Int32(int32(timeout))
		}
		if timeout := getIntFromSvsAnnotation(service, ElbResponseTimeout, d.loadbalancerOpts.ResponseTimeout); timeout != 0 {
			updateOpts.MemberTimeout = pointer.Int32(int32(timeout))
		}
	}

	klog.V(4).Infof("[DEBUG] Update dedicated instance listener options: %s", utils.ToString(updateOpts))

	err := d.dedicatedELBClient.UpdateListener(listener.Id, updateOpts)
	if err != nil {
		return err
	}

	klog.Infof("Listener updated, id: %s, name: %s", listener.Id, listener.Name)
	return nil
}

func (d *DedicatedLoadBalancer) deleteListeners(elbID string, listeners []elbmodel.Listener) error {
	errs := make([]error, 0)
	for _, lis := range listeners {
		pool, err := d.getPool(elbID, lis.Id)
		if err != nil && !common.IsNotFound(err) {
			errs = append(errs, err)
			continue
		}
		if err == nil {
			delErrs := d.deletePool(pool)
			if len(delErrs) > 0 {
				errs = append(errs, delErrs...)
			}
		}
		// delete ELB listener
		if err = d.dedicatedELBClient.DeleteListener(elbID, lis.Id); err != nil && !common.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete ELB listener %s : %s ", lis.Id, err))
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("failed to delete listeners: %s", errors.NewAggregate(errs))
	}

	return nil
}

func (d *DedicatedLoadBalancer) popListener(listeners []elbmodel.Listener, id string) []elbmodel.Listener {
	for i, listener := range listeners {
		if listener.Id == id {
			listeners[i] = listeners[len(listeners)-1]
			listeners = listeners[:len(listeners)-1]
			break
		}
	}
	return listeners
}

func (d *DedicatedLoadBalancer) createPool(listener *elbmodel.Listener, service *v1.Service) (*elbmodel.Pool, error) {
	var sessionPersistence *elbmodel.CreatePoolSessionPersistenceOption

	persistence := d.getSessionAffinity(service)
	if persistence != nil {
		sessionPersistenceType := &elbmodel.CreatePoolSessionPersistenceOptionType{}
		if err := sessionPersistenceType.UnmarshalJSON([]byte(persistence.Type)); err != nil {
			return nil, err
		}
		sessionPersistence = &elbmodel.CreatePoolSessionPersistenceOption{
			CookieName:         persistence.CookieName,
			Type:               *sessionPersistenceType,
			PersistenceTimeout: persistence.PersistenceTimeout,
		}
	}

	lbAlgorithm := getStringFromSvsAnnotation(service, ElbAlgorithm, d.loadbalancerOpts.LBAlgorithm)
	name := fmt.Sprintf("pl_%s", listener.Name)
	protocol := listener.Protocol
	if protocol == ProtocolTerminatedHTTPS {
		protocol = ProtocolHTTP
	}
	return d.dedicatedELBClient.CreatePool(&elbmodel.CreatePoolOption{
		Name:               &name,
		Protocol:           protocol,
		LbAlgorithm:        lbAlgorithm,
		ListenerId:         &listener.Id,
		SessionPersistence: sessionPersistence,
	})
}

func (d *DedicatedLoadBalancer) getPool(elbID, listenerID string) (*elbmodel.Pool, error) {
	loadbalancerIDs := []string{elbID}
	pools, err := d.dedicatedELBClient.ListPools(&elbmodel.ListPoolsRequest{
		LoadbalancerId: &loadbalancerIDs,
	})
	if err != nil {
		return nil, err
	}

	for _, pool := range pools {
		for _, listener := range pool.Listeners {
			if listener.Id == listenerID {
				return &pool, nil
			}
		}
	}
	return nil, status.Errorf(codes.NotFound, "not found pool matched ListenerId: %s, ELB ID: %s", listenerID, elbID)
}

func (d *DedicatedLoadBalancer) deletePool(pool *elbmodel.Pool) []error {
	errs := make([]error, 0)
	// delete all members of pool
	if err := d.sharedELBClient.DeleteAllPoolMembers(pool.Id); err != nil {
		errs = append(errs, err)
	}
	// delete the pool monitor if exists
	if err := d.dedicatedELBClient.DeleteHealthMonitor(pool.HealthmonitorId); err != nil && !common.IsNotFound(err) {
		errs = append(errs, err)
	}
	// delete ELB listener pool
	if err := d.dedicatedELBClient.DeletePool(pool.Id); err != nil && !common.IsNotFound(err) {
		errs = append(errs, err)
	}
	return errs
}

func (d *DedicatedLoadBalancer) addOrRemoveMembers(loadbalancer *elbmodel.LoadBalancer, service *v1.Service,
	pool *elbmodel.Pool, port v1.ServicePort, nodes []*v1.Node) error {

	members, err := d.dedicatedELBClient.ListMembers(&elbmodel.ListMembersRequest{PoolId: pool.Id})
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

	podList, err := d.listPodsBySelector(context.TODO(), service.Namespace, service.Spec.Selector)
	if err != nil {
		return err
	}
	klog.Infof("LoadBalancer Service: %s/%s, Pod list: %v", service.Namespace, service.Name, len(podList.Items))
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

		address, err := getNodeAddress(node)
		if err != nil {
			if common.IsNotFound(err) {
				// Node failure, do not create member
				klog.Warningf("Failed to create SharedLoadBalancer pool member for node %s: %v", node.Name, err)
				continue
			} else {
				return fmt.Errorf("error getting address for node %s: %v", node.Name, err)
			}
		}

		key := fmt.Sprintf("%s:%d", address, port.NodePort)
		if existsMember[key] {
			klog.Infof("[addOrRemoveMembers] node already exists, skip adding, name: %s, address: %s, port: %d",
				node.Name, address, port.NodePort)
			members = d.popMember(members, address, port.NodePort)
			continue
		}

		klog.Infof("[addOrRemoveMembers] add node to pool, name: %s, address: %s, port: %d",
			node.Name, address, port.NodePort)
		// Add a member to the pool.
		if err = d.addMember(loadbalancer, pool, port, node); err != nil {
			return err
		}
		existsMember[key] = true
	}

	// delete the remaining elements in members
	for _, member := range members {
		klog.Infof("[addOrRemoveMembers] remove node from pool, name: %s, address: %s, port: %d",
			member.Name, member.Address, member.ProtocolPort)
		err = d.deleteMember(loadbalancer.Id, pool.Id, member)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *DedicatedLoadBalancer) addMember(loadbalancer *elbmodel.LoadBalancer, pool *elbmodel.Pool, port v1.ServicePort,
	node *v1.Node) error {
	klog.Infof("Add a member(%s) to pool %s", node.Name, pool.Id)
	address, err := getNodeAddress(node)
	if err != nil {
		return err
	}

	name := utils.CutString(fmt.Sprintf("member_%s_%s", pool.Name, node.Name), defaultMaxNameLength)
	opt := &elbmodel.CreateMemberOption{
		Name:         &name,
		ProtocolPort: port.NodePort,
		Address:      address,
	}
	if !loadbalancer.IpTargetEnable {
		opt.SubnetCidrId = &loadbalancer.VipSubnetCidrId
	}

	if _, err = d.dedicatedELBClient.AddMember(pool.Id, opt); err != nil {
		return fmt.Errorf("error creating SharedLoadBalancer pool member for node: %s, %v", node.Name, err)
	}

	loadbalancer, err = d.dedicatedELBClient.WaitStatusActive(loadbalancer.Id)
	if err != nil {
		return fmt.Errorf("timeout when waiting for loadbalancer to be ACTIVE after adding members, "+
			"current status %s", loadbalancer.ProvisioningStatus)
	}

	return nil
}

func (d *DedicatedLoadBalancer) deleteMember(elbID string, poolID string, member elbmodel.Member) error {
	klog.V(4).Infof("Deleting exists member %s for pool %s address %s", member.Id, poolID, member.Address)
	err := d.dedicatedELBClient.DeleteMember(poolID, member.Id)
	if err != nil && !common.IsNotFound(err) {
		return fmt.Errorf("error deleting obsolete member %s for pool %s address %s: %v",
			poolID, member.Id, member.Address, err)
	}
	loadbalancer, err := d.dedicatedELBClient.WaitStatusActive(elbID)
	if err != nil {
		return fmt.Errorf("timeout when waiting for loadbalancer to be ACTIVE after creating member, "+
			"current provisioning status %s", loadbalancer.ProvisioningStatus)
	}
	return nil
}

func (d *DedicatedLoadBalancer) popMember(members []elbmodel.Member, addr string, port int32) []elbmodel.Member {
	for i, m := range members {
		if m.Address == addr && m.ProtocolPort == port {
			members[i] = members[len(members)-1]
			members = members[:len(members)-1]
		}
	}
	return members
}

func (d *DedicatedLoadBalancer) getSessionAffinity(service *v1.Service) *elbmodel.SessionPersistence {
	globalOpts := d.loadbalancerOpts
	sessionMode := getStringFromSvsAnnotation(service, ElbSessionAffinityFlag, globalOpts.SessionAffinityFlag)
	if sessionMode == "" || sessionMode == "off" {
		return nil
	}

	persistenceV2 := globalOpts.SessionAffinityOption

	opts := getStringFromSvsAnnotation(service, ElbSessionAffinityOption, "")
	if opts == "" {
		klog.V(4).Infof("[DEBUG] SessionAffinityOption is empty, use default: %#v", persistenceV2)
		return &elbmodel.SessionPersistence{
			CookieName:         persistenceV2.CookieName,
			Type:               persistenceV2.Type.Value(),
			PersistenceTimeout: persistenceV2.PersistenceTimeout,
		}
	}

	err := json.Unmarshal([]byte(opts), &persistenceV2)
	if err != nil {
		klog.Warningf("error parsing \"kubernetes.io/elb.session-affinity-option\": %s, ignore options: %s",
			err, opts)
	}
	printSessionAffinity(service, persistenceV2)
	return &elbmodel.SessionPersistence{
		CookieName:         persistenceV2.CookieName,
		Type:               persistenceV2.Type.Value(),
		PersistenceTimeout: persistenceV2.PersistenceTimeout,
	}
}

func (d *DedicatedLoadBalancer) addOrRemoveHealthMonitor(loadbalancerID string, pool *elbmodel.Pool,
	port v1.ServicePort, service *v1.Service) error {
	healthCheckOpts := getHealthCheckOptionFromAnnotation(service, d.loadbalancerOpts)
	monitorID := pool.HealthmonitorId
	klog.Infof("add or remove health check: %s : %#v", monitorID, healthCheckOpts)

	// create health monitor
	if monitorID == "" && healthCheckOpts.Enable {
		_, err := d.createHealthMonitor(loadbalancerID, pool.Id, pool.Protocol, healthCheckOpts)
		return err
	}

	// update health monitor
	if monitorID != "" && healthCheckOpts.Enable {
		return d.updateHealthMonitor(monitorID, port.Protocol, healthCheckOpts)
	}

	// delete health monitor
	if monitorID != "" && !healthCheckOpts.Enable {
		klog.Infof("Deleting health monitor %s for pool %s", monitorID, pool.Id)
		err := d.dedicatedELBClient.DeleteHealthMonitor(monitorID)
		if err != nil {
			return fmt.Errorf("failed to delete health monitor %s for pool %s, error: %v", monitorID, pool.Id, err)
		}
	}

	return nil
}

func (d *DedicatedLoadBalancer) updateHealthMonitor(id string, protocol v1.Protocol, opts *config.HealthCheckOption,
) error {
	monitorProtocol := string(protocol)
	if protocol == v1.ProtocolSCTP {
		return status.Errorf(codes.InvalidArgument, "Protocol SCTP not supported")
	}

	return d.dedicatedELBClient.UpdateHealthMonitor(id, &elbmodel.UpdateHealthMonitorOption{
		Type:       &monitorProtocol,
		Timeout:    &opts.Timeout,
		Delay:      &opts.Delay,
		MaxRetries: &opts.MaxRetries,
	})
}

func (d *DedicatedLoadBalancer) createHealthMonitor(loadbalancerID, poolID, protocol string, opts *config.HealthCheckOption) (*elbmodel.HealthMonitor, error) {
	monitor, err := d.dedicatedELBClient.CreateHealthMonitor(&elbmodel.CreateHealthMonitorOption{
		PoolId:     poolID,
		Type:       protocol,
		Timeout:    opts.Timeout,
		Delay:      opts.Delay,
		MaxRetries: opts.MaxRetries,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating SharedLoadBalancer pool health monitor: %v", err)
	}

	loadbalancer, err := d.dedicatedELBClient.WaitStatusActive(loadbalancerID)
	if err != nil {
		return nil, fmt.Errorf("timeout when waiting for loadbalancer to be ACTIVE after creating member, "+
			"current provisioning status %s", loadbalancer.ProvisioningStatus)
	}
	return monitor, nil
}

func (d *DedicatedLoadBalancer) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.Infof("UpdateLoadBalancer: called with service %s/%s, node: %d", service.Namespace, service.Name, len(nodes))
	// get exits or create a new ELB instance
	loadbalancer, err := d.getLoadBalancerInstance(ctx, clusterName, service)
	if err != nil {
		return err
	}

	// query ELB listeners list
	loadbalancerIDs := []string{loadbalancer.Id}
	listeners, err := d.dedicatedELBClient.ListListeners(&elbmodel.ListListenersRequest{
		LoadbalancerId: &loadbalancerIDs,
	})
	if err != nil {
		return err
	}

	for _, port := range service.Spec.Ports {
		listener := d.filterListenerByPort(listeners, service, port)
		if listener == nil {
			return status.Errorf(codes.Unavailable, "error, can not find a listener matching %s:%v",
				port.Protocol, port.Port)
		}

		// query pool or create pool
		pool, err := d.getPool(loadbalancer.Id, listener.Id)
		if err != nil && common.IsNotFound(err) {
			pool, err = d.createPool(listener, service)
		}
		if err != nil {
			return err
		}

		// add new members and remove the obsolete members.
		if err = d.addOrRemoveMembers(loadbalancer, service, pool, port, nodes); err != nil {
			return err
		}

		// add or remove health monitor
		if err = d.addOrRemoveHealthMonitor(loadbalancer.Id, pool, port, service); err != nil {
			return err
		}
	}
	return nil
}

func (d *DedicatedLoadBalancer) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.Infof("EnsureLoadBalancerDeleted: called with service %s/%s", service.Namespace, service.Name)
	serviceName := fmt.Sprintf("%s/%s", service.Namespace, service.Name)
	klog.Infof("EnsureLoadBalancerDeleted(%s, %s)", clusterName, serviceName)

	loadBalancer, err := d.getLoadBalancerInstance(ctx, clusterName, service)
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
		err = d.deleteListener(loadBalancer, service)
	} else {
		err = d.deleteELBInstance(loadBalancer, service)
	}

	if err != nil {
		return err
	}
	return nil
}

func (d *DedicatedLoadBalancer) deleteListener(loadBalancer *elbmodel.LoadBalancer, service *v1.Service) error {
	// query ELB listeners list
	loadbalancerIDs := []string{loadBalancer.Id}
	listenerArr, err := d.dedicatedELBClient.ListListeners(&elbmodel.ListListenersRequest{
		LoadbalancerId: &loadbalancerIDs,
	})
	if err != nil {
		return err
	}

	listenersMatched := make([]elbmodel.Listener, 0)
	for _, port := range service.Spec.Ports {
		listener := d.filterListenerByPort(listenerArr, service, port)
		if listener != nil {
			listenersMatched = append(listenersMatched, *listener)
		}
	}

	if err = d.deleteListeners(loadBalancer.Id, listenersMatched); err != nil {
		return err
	}
	return nil
}

func (d *DedicatedLoadBalancer) deleteELBInstance(loadBalancer *elbmodel.LoadBalancer, service *v1.Service) error {
	// query ELB listeners list
	loadbalancerIDs := []string{loadBalancer.Id}
	listenerArr, err := d.dedicatedELBClient.ListListeners(&elbmodel.ListListenersRequest{
		LoadbalancerId: &loadbalancerIDs,
	})
	if err != nil {
		return err
	}

	if err = d.deleteListeners(loadBalancer.Id, listenerArr); err != nil {
		return err
	}

	eipID := getStringFromSvsAnnotation(service, ElbEipID, "")
	keepEip := getBoolFromSvsAnnotation(service, ELBKeepEip, d.loadbalancerOpts.KeepEIP)
	if err = unbindEIP(d.eipClient, loadBalancer.VipPortId, eipID, keepEip); err != nil {
		return err
	}
	if err = d.sharedELBClient.DeleteInstance(loadBalancer.Id); err != nil {
		return err
	}
	return nil
}
