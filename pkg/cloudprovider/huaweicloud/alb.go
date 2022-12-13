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
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/hashicorp/golang-lru"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
)

// ALBCloud implements shared load balancers
type ALBCloud struct {
	config        *LBConfig
	kubeClient    corev1.CoreV1Interface
	lrucache      *lru.Cache
	eventRecorder record.EventRecorder
	subnetMap     map[string]string
	subnetMapLock sync.RWMutex
}

type tempALBServicePort struct {
	servicePort *v1.ServicePort
	listener    ALBListener
}

// GetLoadBalancer get a loadbalancer for a service.
func (alb *ALBCloud) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	status = &v1.LoadBalancerStatus{}
	albProvider, err := alb.getALBClient()

	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}

		return nil, false, err
	}

	listeners, err := albProvider.findListenerOfService(service)
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
	albProvider, err := alb.getALBClient()
	if err != nil {
		return nil, err
	}

	loadBalancerID := ""
	//auto create lb if not exist
	if isNeedAutoCreateLB(service) {
		loadBalancerID, err = alb.createAlbInstance(service, albProvider)
		if err != nil {
			return nil, err
		}
	} else {
		loadBalancerID, err = alb.checkAlbInstance(service, albProvider)
		if err != nil {
			return nil, err
		}
	}

	listeners, err := albProvider.findListenerOfService(service)
	if err != nil {
		return nil, err
	}

	members, err := alb.generateMembers(service, hosts)
	if err != nil {
		return nil, err
	}

	needsCreate, needsUpdate, needsDelete := alb.compare(loadBalancerID, service, listeners)
	ch := make(chan error, 3)

	go func() {
		ch <- alb.createLoadBalancer(albProvider, loadBalancerID, service, needsCreate, members)
	}()

	go func() {
		ch <- alb.updateLoadBalancer(albProvider, service, needsUpdate, members)
	}()

	go func() {
		ch <- alb.deleteLoadBalancer(albProvider, service, needsDelete)
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

	klog.V(4).Infof("load balancer ip is:%s", service.Spec.LoadBalancerIP)
	status := &v1.LoadBalancerStatus{}
	status.Ingress = append(status.Ingress, v1.LoadBalancerIngress{IP: service.Spec.LoadBalancerIP})
	return status, nil
}

// update members in the service
//
//	(1) find out new members for the service according to service pods and the health status
//	(2) find out the pool
//	(3) get previous members under the pool
//	(4) compare the equality of two member sets, if not equal, update the pool members
func (alb *ALBCloud) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.Infof("Begin to update loadbalancer configuration of service(%s/%s)", service.Namespace, service.Name)
	albProvider, err := alb.getALBClient()
	if err != nil {
		return err
	}

	members, err := alb.generateMembers(service, nodes)
	if err != nil {
		return err
	}

	listeners, err := albProvider.findListenerOfService(service)
	if err != nil {
		return err
	}

	c, err := alb.generateServiceLBConfig(service)
	if err != nil {
		return err
	}

	var errs []error
	for _, listener := range listeners {
		pool, err := albProvider.findPoolOfListener(listener.Id)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Get pool of listener(%s) error: %v", listener.Id, err)
			sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
			continue
		}

		if pool == nil {
			poolConf := ALBPool{
				LbAlgorithm:  c.AgConfig.LBAlgorithm,
				Protocol:     listener.Protocol,
				ListenerId:   listener.Id,
				AdminStateUp: true,
			}

			if c.SAConfig.LBSessionAffinityType != "" {
				poolConf.SessionPersistence.Type = c.SAConfig.LBSessionAffinityType
				switch poolConf.SessionPersistence.Type {
				case ELBSessionSource:
					timeout := c.SAConfig.LBSessionAffinityOption.PersistenceTimeout
					poolConf.SessionPersistence.PersistenceTimeout = timeout
				}
			}

			pool, err = albProvider.CreatePool(&poolConf)
			if err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Create pool of listener(%s) error: %v", listener.Id, err)
				sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
				continue
			}
		}

		if c.HMConfig.LBHealthCheckStatus &&
			pool.HealthMonitorId == "" {
			healthMonitorConf := ALBHealthMonitor{
				Type:         c.HMConfig.LBHealthCheckOption.Protocol,
				PoolId:       pool.Id,
				Delay:        c.HMConfig.LBHealthCheckOption.Delay,
				Timeout:      c.HMConfig.LBHealthCheckOption.Timeout,
				MaxRetries:   c.HMConfig.LBHealthCheckOption.MaxRetries,
				MonitorPort:  c.HMConfig.LBHealthCheckOption.CheckPort,
				UrlPath:      c.HMConfig.LBHealthCheckOption.UrlPath,
				AdminStateUp: true,
			}

			_, err = albProvider.CreateHealthMonitor(&healthMonitorConf)
			if err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Create health Monitor of pool(%s) error: %v", pool.Id, err)
				sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
				continue
			}
			klog.Infof("Create health monitor of loadbalancer/pool(%s:%s) success.", service.Spec.LoadBalancerIP, pool.Id)
		}

		var curPort int32
		for _, port := range service.Spec.Ports {
			protocolPort, err := alb.getMemberProtocolPort(service, port)
			if err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Failed to get service(%s) ports, error: %v", service.Name, err)
				sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
				continue
			}
			if port.Port == listener.ProtocolPort {
				curPort = protocolPort
				break
			}
		}
		if curPort == 0 {
			msg := fmt.Sprintf("Get backend port of pool(%s) error", pool.Id)
			sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
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
			sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
			continue
		}
		preMembers := make(map[string][]ALBMember)
		for _, member := range preListMembers.Members {
			preMembers[member.Address] = append(preMembers[member.Address], member)
		}

		addedMember, err := alb.updateALbMembers(albProvider, service, curMembers, preMembers, pool.Id)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Update member of listener/pool(%s/%s) error: %v", listener.Id, pool.Id, err)
			sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
		}

		//addedMember contain members new added
		if len(addedMember) != 0 {
			alb.asyncWaitMembers(albProvider, service, pool.Id, addedMember, true)
		}
	}
	klog.Infof("Update loadbalancer of service(%s/%s) finish, total: %d, failed: %d",
		service.Namespace, service.Name, len(listeners), len(errs))
	return utilerrors.NewAggregate(errs)
}

// delete all resources under a service, including listener, pool, healthMonitor and member
//
//	(1) find out the listener of the service
//	(2) find out the pool of the listener
//	(3) get all members & healthMonitor under this pool
//	(4) delete these members & healthMonitors in the pool
//	(5) delete the pool & listener
func (alb *ALBCloud) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.Infof("Begin to delete loadbalancer configuration of service(%s/%s)", service.Namespace, service.Name)
	albProvider, err := alb.getALBClient()
	if err != nil {
		return err
	}

	listeners, err := albProvider.findListenerOfService(service)
	if err != nil {
		return err
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
			sendEvent(alb.eventRecorder, "DeleteListenerFailed", msg, service)
		}

	}

	if len(listeners) != 0 {
		klog.Infof("Delete listeners of service(%s/%s) finish, total: %d, failed: %d",
			service.Namespace, service.Name, len(listeners), len(errs))
	}

	if len(errs) != 0 {
		return utilerrors.NewAggregate(errs)
	}

	//has auto create load balancer instance
	if hasAutoCreateLB(service) {
		if persist := GetPersistAutoCreate(service); persist {
			klog.Infof("persist LoadBalancer of service(%s/%s) when persist Auto create set to true",
				service.Namespace, service.Name)
			return nil
		}

		klog.Infof("Begin to delete loadbalancer of service(%s/%s)", service.Namespace, service.Name)
		if statusCode, err := albProvider.DeleteLoadBalancer(service.Annotations[ELBIDAnnotation]); err != nil {
			if statusCode != http.StatusConflict {
				errs = append(errs, err)
			}
			msg := fmt.Sprintf("Delete Loadbalancer(%s) error: %v", service.Annotations[ELBIDAnnotation], err)
			sendEvent(alb.eventRecorder, "DeleteLoadBalancerFailed", msg, service)
			return utilerrors.NewAggregate(errs)
		}

		var elbAutoCreate ElbAutoCreate
		err = json.Unmarshal([]byte(service.Annotations[ELBAutoCreateAnnotation]), &elbAutoCreate)
		if err != nil {
			msg := fmt.Sprintf("Unmarshal elbAutoCreate params failed: %v", err)
			sendEvent(alb.eventRecorder, "DeleteLoadBalancerFailed", msg, service)
			return utilerrors.NewAggregate(errs)
		}

		if elbAutoCreate.ElbType == "public" {
			if statusCode, err := albProvider.DeleteEip(service.Annotations[ELBEIPIDAnnotation]); err != nil {
				if statusCode != http.StatusConflict {
					errs = append(errs, err)
				}
				msg := fmt.Sprintf("Delete eip(%s) error: %v", service.Annotations[ELBEIPIDAnnotation], err)
				sendEvent(alb.eventRecorder, "DeleteEIPFailed", msg, service)
				return utilerrors.NewAggregate(errs)
			}
		}
		klog.Infof("Delete loadbalancer of service(%s/%s) successfully", service.Namespace, service.Name)
	}

	//change from load balancer to other service type
	if service.Spec.Type != v1.ServiceTypeLoadBalancer {
		serviceCopy := service.DeepCopy()
		serviceCopy.Annotations = make(map[string]string)
		for k, v := range service.Annotations {
			if !strings.HasPrefix(k, ELBAnnotationPrefix) {
				serviceCopy.Annotations[k] = v
			}
		}
		serviceCopy, err = alb.updateService(serviceCopy)
		if err != nil {
			klog.Errorf("Update service failed: %v", err)
			return utilerrors.NewAggregate(errs)
		}
		*service = *serviceCopy
	}

	return utilerrors.NewAggregate(errs)
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
func (alb *ALBCloud) getALBClient() (*ALBClient, error) {
	secret, err := alb.getSecret(alb.config.SecretName)
	if err != nil {
		return nil, err
	}

	var accessKey, secretKey, securityToken string
	if len(secret.Credential) > 0 { // 'Temporary Security Credentials'
		klog.Infof("Using temporary security credentials.")
		var sc SecurityCredential
		err = json.Unmarshal([]byte(secret.Credential), &sc)
		if err != nil {
			return nil, fmt.Errorf("unmarshal security credential failed, error: %v", err)
		}
		accessKey = sc.AccessKey
		secretKey = sc.SecretKey
		securityToken = sc.SecurityToken
	} else { // 'Permanent Security Credentials'
		klog.Infof("Using permanent security credentials.")
		accessKey = secret.AccessKey
		secretKey = secret.SecretKey
	}

	return NewALBClient(
		alb.config.ALBEndpoint,
		alb.config.VPCEndpoint,
		alb.config.TenantId,
		accessKey,
		secretKey,
		securityToken,
		alb.config.Region,
		alb.config.SignerType,
		alb.config.EnterpriseEnable,
	), nil
}

func (alb *ALBCloud) getSecret(secretName string) (*Secret, error) {
	var kubeSecret *v1.Secret

	key := providerNamespace + "/" + secretName
	obj, ok := alb.lrucache.Get(key)
	if ok {
		kubeSecret = obj.(*v1.Secret)
	} else {
		secret, err := alb.kubeClient.Secrets(providerNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		kubeSecret = secret
	}

	bytes, err := json.Marshal(kubeSecret.Data)
	if err != nil {
		return nil, err
	}

	var secret Secret
	err = json.Unmarshal(bytes, &secret)
	if err != nil {
		return nil, err
	}

	if err := secret.DecodeBase64(); err != nil {
		klog.Errorf("Decode secret failed with error: %v", err)
		return nil, err
	}

	return &secret, nil
}

func (alb *ALBCloud) getPods(name, namespace string) (*v1.PodList, error) {
	service, err := alb.kubeClient.Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if len(service.Spec.Selector) == 0 {
		return nil, fmt.Errorf("the service %s-%s has no selector to associate the pods", service.Namespace, service.Name)
	}

	set := labels.Set(service.Spec.Selector)
	labelSelector := labels.SelectorFromSet(set)

	opts := metav1.ListOptions{LabelSelector: labelSelector.String()}
	return alb.kubeClient.Pods(namespace).List(context.TODO(), opts)
}

func (alb *ALBCloud) ensureCreateLoadbalancer(albProvider *ALBClient, elbAC ElbAutoCreate, service *v1.Service) (*ALB, error) {
	neutronSubnetId, err := alb.getClusterNeutronSubnetId(alb.config.SubnetId)
	if err != nil {
		return nil, err
	}
	lbName := elbAC.ELbName
	if lbName == "" {
		lbName = GetLoadbalancerName(service)
	}
	albInstanceConf := &ALB{
		Name:         lbName,
		TenantId:     albProvider.albClient.TenantId,
		VipSubnetId:  neutronSubnetId,
		AdminStateUp: true,
		Description:  Attention,
	}
	if alb.config.EnterpriseEnable == "true" {
		albInstanceConf.EnterpriseProjectId = DefaultEnterpriseProjectID
		if service.Annotations[ELBEnterpriseAnnotationKey] != "" {
			albInstanceConf.EnterpriseProjectId = service.Annotations[ELBEnterpriseAnnotationKey]
		}
	}
	albInstanceInfo, err := albProvider.CreateLoadBalancer(albInstanceConf)
	if err != nil {
		return nil, fmt.Errorf("EnsureCreateloadbalancer: Failed to CreateLoadbalancer : %v", err)
	}

	return albInstanceInfo, nil
}

// make sure the listener is created under specific load balancer, if not, let's create one
// albProvider: alb client
// name: listener name
// albAlgorithm: load balance algorithm
// port: the port in "EIP:port"
// albId: load balancer ID
func (alb *ALBCloud) ensureCreateListener(albProvider ALBClient,
	name string,
	service v1.Service,
	port v1.ServicePort,
	albId string) (string, error) {
	ListenerDesc := ELBListenerDescription{
		ClusterID: os.Getenv(ClusterID),
		ServiceID: string(service.UID),
		Attention: Attention,
	}
	d, err := json.Marshal(&ListenerDesc)
	if err != nil {
		return "", fmt.Errorf("EnsureCreateListener: Failed to get Listener Description : %v", err)
	}
	listenerConf := &ALBListener{
		Name:           name,
		AdminStateUp:   true,
		Protocol:       ELBProtocol(port.Protocol),
		ProtocolPort:   port.Port,
		LoadbalancerId: albId,
		Description:    string(d),
	}

	listener, err := albProvider.CreateListener(listenerConf)
	if err != nil {
		return "", fmt.Errorf("EnsureCreateListener: Failed to CreateListener : %v", err)
	}
	return listener.Id, nil
}

func (alb *ALBCloud) updateALbMembers(
	albProvider *ALBClient,
	service *v1.Service,
	newMembers map[string]*ALBMember,
	preMembers map[string][]ALBMember,
	poolId string) (map[string]*ALBMember, error) {
	var errs []error
	deleteMemberIds := make(map[string]bool)
	exitMembers := []*ALBMember{}
	addedMembers := make(map[string]*ALBMember)
	for memberAddress, preMembers := range preMembers {
		for _, preMember := range preMembers {
			tmpMember := preMember
			newMember, ok := newMembers[memberAddress]
			if !ok || tmpMember.ProtocolPort != newMember.ProtocolPort {
				deleteMemberIds[tmpMember.Id] = true
			} else {
				delete(newMembers, memberAddress)
				exitMembers = append(exitMembers, &tmpMember)
			}
		}
	}
	for memberAddress := range newMembers {
		newMember := newMembers[memberAddress]
		klog.Infof("Begin to add member(%s) of service(%s/%s)",
			memberAddress, service.Namespace, service.Name)
		m, err := albProvider.AddMember(poolId, newMember)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		addedMembers[memberAddress] = m
	}

	klog.V(3).Infof("Process service(%s/%s), when add member is finished, "+
		"needs retry to clean the useless members", service.Namespace, service.Name)
	if alb.gracefulAlbRemoveMembers(exitMembers) {
		klog.Infof("Can trigger graceful remove members(%s/%d) of service(%s/%s)",
			poolId, len(deleteMemberIds), service.Namespace, service.Name)
		for deleteMemberId := range deleteMemberIds {
			klog.Infof("graceful remove member(%s/%s) of service(%s/%s)",
				poolId, deleteMemberId, service.Namespace, service.Name)
			err := albProvider.DeleteMember(poolId, deleteMemberId)
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}

	return addedMembers, utilerrors.NewAggregate(errs)
}

func (alb *ALBCloud) asyncWaitMembers(
	albProvider *ALBClient,
	service *v1.Service,
	poolID string,
	newMembers map[string]*ALBMember,
	tryAgain bool,
) {
	go func() {
		var errs []error
		var needsDelete []string
		for _, member := range newMembers {
			err := albProvider.WaitMemberComplete(poolID, member.Id)
			if err != nil {
				needsDelete = append(needsDelete, member.Id)
				errs = append(errs, err)
				msg := fmt.Sprintf("Member(%s/%s) is offline: %v",
					poolID, member.Id, err)
				sendEvent(alb.eventRecorder, "CreateLoadBalancerFailed", msg, service)
				continue
			}
			klog.Infof("Member of service(%s/%s) is success", service.Namespace, service.Name)
		}

		if len(errs) != 0 {
			klog.Errorf("Members(%s) of service(%s/%s) is offline need clean off and retry",
				strings.Join(needsDelete, ","), service.Namespace, service.Name)
			alb.cleanOfflineMemberAndRetry(albProvider, service, poolID, needsDelete)
		} else {
			updateServiceMarkIfNeeded(alb.kubeClient, service, tryAgain)
		}

		klog.Infof("Members of service(%s/%s) have finished: total: %d, failed: %d",
			service.Namespace, service.Name, len(newMembers), len(errs))
	}()
}

func (alb *ALBCloud) cleanOfflineMemberAndRetry(
	albProvider *ALBClient,
	service *v1.Service,
	poolID string,
	needsDelete []string) {
	for _, memberID := range needsDelete {
		err := albProvider.DeleteMember(poolID, memberID)
		if err != nil {
			klog.Warningf("Clean offline member(%s/%s) error: %v", poolID, memberID, err)
		}
	}

	updateServiceStatus(alb.kubeClient, alb.eventRecorder, service)
}

// although member has already been added, in fact,
// the member will wait the health check is ok, then
// transfer the packages, so before delete the old member,
// we need to check if there have available members
func (alb *ALBCloud) gracefulAlbRemoveMembers(existMembers []*ALBMember) bool {
	hasAvailableMember := false
	for _, member := range existMembers {
		if member.OperatingStatus == MemberStatusONLINE ||
			member.OperatingStatus == MemberStatusNOMONITOR {
			hasAvailableMember = true
			break
		}
	}

	return hasAvailableMember
}

func (alb *ALBCloud) checkServiceLBConfig(service *v1.Service, c *ServiceLBConfig) error {
	if c.AgConfig.LBAlgorithm == ELBAlgorithmSRC && c.SAConfig.LBSessionAffinityType != "" {
		return fmt.Errorf("algorithm (%s) can not support session affinity type(%s)",
			c.AgConfig.LBAlgorithm, c.SAConfig.LBSessionAffinityType)
	}
	//all protocol in a service is the same
	port := service.Spec.Ports[0]
	if port.Protocol == v1.ProtocolTCP {
		if c.HMConfig.LBHealthCheckOption.Protocol != ELBProtocolHTTP &&
			c.HMConfig.LBHealthCheckOption.Protocol != ELBProtocolTCP {
			return fmt.Errorf("heath check protocol(%s) for service(%s/%s) can not be supported"+
				" when all ports in service is in protocol(%s)",
				c.HMConfig.LBHealthCheckOption.Protocol, service.Namespace, service.Name, v1.ProtocolTCP)
		}
	}
	if port.Protocol == v1.ProtocolUDP {
		if c.HMConfig.LBHealthCheckOption.Protocol != ELBHealthMonitorTypeUDP {
			return fmt.Errorf("heath check protocol(%s) for service(%s/%s) can not be supported "+
				"when all ports in service is in protocol(%s)",
				c.HMConfig.LBHealthCheckOption.Protocol, service.Namespace, service.Name, v1.ProtocolUDP)
		}
	}
	return nil
}

func (alb *ALBCloud) generateServiceLBConfig(service *v1.Service) (*ServiceLBConfig, error) {
	algorithm, err := getLBAlgorithm(service)
	if err != nil {
		return nil, err
	}
	saType, saOption, err := alb.getLBSessionAffinity(service)
	if err != nil {
		return nil, err
	}
	hmc, err := alb.getLBHealthMonitor(service)
	if err != nil {
		return nil, err
	}

	//check if config is fine
	sc := &ServiceLBConfig{
		AgConfig: &LBAlgorithmConfig{
			LBAlgorithm: algorithm,
		},
		SAConfig: &LBSessionAffinityConfig{
			LBSessionAffinityType:   saType,
			LBSessionAffinityOption: saOption,
		},
		HMConfig: hmc,
	}
	err = alb.checkServiceLBConfig(service, sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

func (alb *ALBCloud) createAlbInstance(service *v1.Service, albProvider *ALBClient) (string, error) {
	serviceCopy := service.DeepCopy()
	var elbAutoCreate ElbAutoCreate
	err := json.Unmarshal([]byte(serviceCopy.Annotations[ELBAutoCreateAnnotation]), &elbAutoCreate)
	if err != nil {
		msg := fmt.Sprintf("Unmarshal elbAutoCreate params failed: %v", err)
		sendEvent(alb.eventRecorder, "CreateLoadBalancerFailed", msg, serviceCopy)
		return "", err
	}

	albInstanceInfo, err := alb.ensureCreateLoadbalancer(albProvider, elbAutoCreate, serviceCopy)
	if err != nil {
		msg := fmt.Sprintf("Create Loadbalancer error: %v", err)
		sendEvent(alb.eventRecorder, "CreateLoadBalancerFailed", msg, serviceCopy)
		return "", err
	}
	serviceCopy.Spec.LoadBalancerIP = albInstanceInfo.VipAddress
	serviceCopy.Annotations[ELBIDAnnotation] = albInstanceInfo.Id

	eipConf := &PublicIp{
		Publicip: PublicIpSpec{
			Type: elbAutoCreate.EipType,
		},
		Bandwidth: BandwidthSpec{
			Name:       elbAutoCreate.BandwidthName,
			Size:       elbAutoCreate.BandwidthSize,
			ShareType:  elbAutoCreate.BandwidthSharetype,
			ChargeMode: elbAutoCreate.BandwidthChargemode,
		},
	}
	var eip *PublicIp
	if elbAutoCreate.ElbType == "public" {
		eip, err = albProvider.CreateEip(eipConf)
		if err != nil {
			msg := fmt.Sprintf("Create EIP error: %v", err)
			sendEvent(alb.eventRecorder, "CreateLoadBalancerFailed", msg, serviceCopy)
			alb.deleteElb(albProvider, albInstanceInfo.Id, service)
			return "", err
		}

		eipBindBody := &PublicIp{
			Publicip: PublicIpSpec{
				PortID: albInstanceInfo.VipPortDd,
			},
		}
		_, err = albProvider.BindEip(eipBindBody, eip.Publicip.ID)
		if err != nil {
			msg := fmt.Sprintf("Bind EIP error: %v", err)
			sendEvent(alb.eventRecorder, "CreateLoadBalancerFailed", msg, serviceCopy)
			alb.deleteElb(albProvider, albInstanceInfo.Id, service)
			alb.deleteEip(albProvider, eip.Publicip.ID, service)
			return "", err
		}
		serviceCopy.Annotations[ELBEIPIDAnnotation] = eip.Publicip.ID
		serviceCopy.Spec.LoadBalancerIP = eip.Publicip.PublicIpAddress
	}

	serviceCopy, err = alb.updateService(serviceCopy)
	if err != nil {
		klog.Errorf("Update service failed: %v", err)
		alb.deleteElb(albProvider, albInstanceInfo.Id, serviceCopy)
		if eip.Publicip.ID != "" {
			alb.deleteEip(albProvider, eip.Publicip.ID, serviceCopy)
		}
		return "", err
	}
	*service = *serviceCopy

	return albInstanceInfo.Id, nil
}

func (alb *ALBCloud) checkAlbInstance(service *v1.Service, albProvider *ALBClient) (string, error) {
	loadBalancerId := service.Annotations[ELBIDAnnotation]
	serviceCopy := service.DeepCopy()
	if service.Spec.LoadBalancerIP == "" || loadBalancerId == "" {
		lb, _, err := alb.getAlbInstanceInfo(service, albProvider, false)
		if err != nil {
			return "", err
		}
		loadBalancerId = lb.Id
		if serviceCopy.Annotations == nil {
			serviceCopy.Annotations = make(map[string]string)
		}
		serviceCopy.Annotations[ELBIDAnnotation] = lb.Id
		if service.Spec.LoadBalancerIP == "" {
			eip, err := alb.getPublicIPFromPrivateIP(lb.VipPortDd)
			if err != nil {
				return "", err
			}
			serviceCopy.Spec.LoadBalancerIP = eip
		}

		serviceCopy, err = alb.updateService(serviceCopy)
		if err != nil {
			klog.Errorf("Update service failed: %v", err)
			return "", err
		}
		*service = *serviceCopy
	}

	return loadBalancerId, nil
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
		vipAddress, err := alb.getPrivateIpFromLoadbalancerIp(service.Spec.LoadBalancerIP)
		if err != nil {
			return nil, "", err
		}

		params := map[string]string{"vip_address": vipAddress}
		neutronSubnetId, err := alb.getElbNeutronSubnetIdFromElbIp(vipAddress)
		if err != nil {
			return nil, "", err
		}
		if neutronSubnetId != "" {
			params["vip_subnet_id"] = neutronSubnetId
		}

		albInstances, err := albProvider.ListLoadBalancers(params)
		if err != nil {
			return nil, "", err
		}
		if len(albInstances.Loadbalancers) == 0 {
			return nil, "", fmt.Errorf("there has no LoadBalancer found")
		}
		albInstanceInfo = &albInstances.Loadbalancers[0]
		if onlyGetLoadBalanceID {
			return nil, albInstanceInfo.VipAddress, nil
		}
	}

	return albInstanceInfo, albInstanceInfo.Id, nil
}

func (alb *ALBCloud) compare(
	loadBalancerID string,
	service *v1.Service,
	listeners map[string]ALBListener) ([]v1.ServicePort, map[string]tempALBServicePort, []ALBListener) {
	var needsCreate []v1.ServicePort
	needsUpdate := make(map[string]tempALBServicePort)
	var needsDelete []ALBListener
	for i := range service.Spec.Ports {
		port := service.Spec.Ports[i]
		if port.Name == HealthzCCE {
			continue
		}
		create := true
		for _, listener := range listeners {
			if len(listener.Loadbalancers) == 0 {
				klog.Warningf("The listener(%s/%s) has no related loadbalancer", listener.Id, listener.Name)
				continue
			}
			if port.Port == listener.ProtocolPort &&
				listener.Loadbalancers[0].Id == loadBalancerID {
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

func (alb *ALBCloud) generateMembers(service *v1.Service, nodes []*v1.Node) ([]*ALBMember, error) {
	podList, err := alb.getPods(service.Name, service.Namespace)
	if err != nil {
		return nil, err
	}

	nodesMap := map[string]*v1.Node{}
	for _, node := range nodes {
		nodesMap[node.Name] = node
	}

	var members []*ALBMember
	hasNodeExist := map[string]bool{}
	for _, item := range podList.Items {
		if item.Status.HostIP == "" {
			klog.Errorf("pod(%s/%s) has not been scheduled", item.Namespace, item.Name)
			continue
		}

		if !IsPodActive(&item) {
			klog.Errorf("pod(%s/%s) is not active", item.Namespace, item.Name)
			continue
		}

		if hasNodeExist[item.Status.HostIP] {
			continue
		}

		node, ok := nodesMap[item.Spec.NodeName]
		if !ok {
			klog.Errorf("node(%s) is not found", item.Spec.NodeName)
			return nil, fmt.Errorf("node(%s) is not found", item.Spec.NodeName)
		}
		subnetId, ok := node.Labels[NodeSubnetIDLabelKey]
		if !ok {
			subnetId = alb.config.SubnetId
		}

		neutronSubnetId, err := alb.getClusterNeutronSubnetId(subnetId)
		if err != nil {
			return nil, err
		}

		hasNodeExist[item.Status.HostIP] = true
		member := ALBMember{
			Address:      item.Status.HostIP,
			SubnetId:     neutronSubnetId,
			AdminStateUp: true,
		}
		members = append(members, &member)
	}

	// TODO: consider the pods have been deleted.
	if len(members) == 0 {
		return nil, fmt.Errorf("have no node to bind")
	}

	return members, nil
}

func (alb *ALBCloud) createLoadBalancer(
	albProvider *ALBClient,
	loadBalancerID string,
	service *v1.Service,
	needsCreate []v1.ServicePort,
	members []*ALBMember) error {
	var (
		errs []error
	)

	c, err := alb.generateServiceLBConfig(service)
	if err != nil {
		msg := fmt.Sprintf("Create loadbalancer(%s) error: %v", service.Spec.LoadBalancerIP, err)
		sendEvent(alb.eventRecorder, "CreateLoadBalancerFailed", msg, service)
		return err
	}

	for _, port := range needsCreate {
		//generate service name: k8s_protocol_port
		lsName := GetListenerNameV1(&port)
		// Step 1. create listener if needed
		listenerId, err := alb.ensureCreateListener(
			*albProvider,
			lsName,
			*service,
			port,
			loadBalancerID)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Create listener(port:%d) error: %v", port.Port, err)
			sendEvent(alb.eventRecorder, "CreateLoadBalancerFailed", msg, service)
			continue
		}

		klog.Infof("Create listener(%s/%d) of loadbalancer(%s) success.", lsName, port.Port, service.Spec.LoadBalancerIP)

		poolName := GetPoolNameV1(service, &port)
		poolDesc := ELBPoolDescription{
			ClusterID: os.Getenv(ClusterID),
			ServiceID: string(service.UID),
			Port:      port.TargetPort.IntVal,
			Attention: Attention,
		}
		d, _ := json.Marshal(&poolDesc)
		// Step 2. Create pool
		poolConf := ALBPool{
			Name:         poolName,
			LbAlgorithm:  c.AgConfig.LBAlgorithm,
			Protocol:     ELBProtocol(port.Protocol),
			ListenerId:   listenerId,
			AdminStateUp: true,
			Description:  string(d),
		}

		if c.SAConfig.LBSessionAffinityType != "" {
			poolConf.SessionPersistence.Type = c.SAConfig.LBSessionAffinityType
			switch poolConf.SessionPersistence.Type {
			case ELBSessionSource:
				timeout := c.SAConfig.LBSessionAffinityOption.PersistenceTimeout
				poolConf.SessionPersistence.PersistenceTimeout = timeout
			}
		}

		pool, err := albProvider.CreatePool(&poolConf)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Create pool of listener(%s) error: %v", listenerId, err)
			sendEvent(alb.eventRecorder, "CreateLoadBalancerFailed", msg, service)
			continue
		}

		klog.Infof("Create pool(%s) of loadbalancer/listener(%s:%s) success.", pool.Id, service.Spec.LoadBalancerIP, listenerId)

		// step 3 create health monitor if needed
		if c.HMConfig.LBHealthCheckStatus {
			healthMonitorConf := ALBHealthMonitor{
				Type:         c.HMConfig.LBHealthCheckOption.Protocol,
				PoolId:       pool.Id,
				Delay:        c.HMConfig.LBHealthCheckOption.Delay,
				Timeout:      c.HMConfig.LBHealthCheckOption.Timeout,
				MaxRetries:   c.HMConfig.LBHealthCheckOption.MaxRetries,
				MonitorPort:  c.HMConfig.LBHealthCheckOption.CheckPort,
				UrlPath:      c.HMConfig.LBHealthCheckOption.UrlPath,
				AdminStateUp: true,
			}

			_, err = albProvider.CreateHealthMonitor(&healthMonitorConf)
			if err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Create health Monitor of pool(%s) error: %v", pool.Id, err)
				sendEvent(alb.eventRecorder, "CreateLoadBalancerFailed", msg, service)
				continue
			}
			klog.Infof("Create health monitor of loadbalancer/pool(%s:%s) success.", service.Spec.LoadBalancerIP, pool.Id)
		}

		// Step 4. add backend hosts
		protocolPort, err := alb.getMemberProtocolPort(service, port)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Failed to get service(%s) ports, error: %v", service.Name, err)
			sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
			continue
		}
		newMembers := make(map[string]*ALBMember)
		for _, member := range members {
			member.ProtocolPort = protocolPort
			m, err := albProvider.AddMember(pool.Id, member)
			if err != nil {
				errs = append(errs, err)
				klog.Warningf("create member for listener failed. listener: %s, member address: %s, member protocol: %d, error: %v", listenerId, member.Address, member.ProtocolPort, err)
				msg := fmt.Sprintf("Create members of listener(%s) error: %v", listenerId, err)
				sendEvent(alb.eventRecorder, "CreateLoadBalancerFailed", msg, service)
				continue
			}
			newMembers[member.Address] = m
			klog.Infof("successfully added a member for listener. listener: %s, member address: %s, member protocol: %d", listenerId, member.Address, member.ProtocolPort)
		}

		if len(newMembers) != 0 {
			klog.Infof("Wait member of service(%s/%s) to be complete", service.Namespace, service.Name)
			alb.asyncWaitMembers(albProvider, service, pool.Id, newMembers, false)
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
				sendEvent(alb.eventRecorder, "DeleteLoadBalancerFailed", err.Error(), service)
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

	c, err := alb.generateServiceLBConfig(service)
	if err != nil {
		msg := fmt.Sprintf("Update loadbalancer(%s) error: %v", service.Spec.LoadBalancerIP, err)
		sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
		return err
	}

	for _, tempPort := range needsUpdate {
		needUpdateListener := false
		updateListener := &ALBListener{
			//Id: l.Id,
		}
		ListenerDesc := ELBListenerDescription{
			ClusterID: os.Getenv(ClusterID),
			ServiceID: string(service.UID),
			Attention: Attention,
		}
		d, _ := json.Marshal(&ListenerDesc)
		if tempPort.listener.Description != string(d) {
			needUpdateListener = true
			updateListener.Description = string(d)
		}

		oldListenerName1 := GetListenerName(service)
		oldListenerName2 := GetOldListenerName(service)
		listenerNameV1 := GetListenerNameV1(tempPort.servicePort)
		if tempPort.listener.Name != listenerNameV1 &&
			(tempPort.listener.Name == oldListenerName1 ||
				tempPort.listener.Name == oldListenerName2) {
			needUpdateListener = true
			updateListener.Name = listenerNameV1
			klog.Infof("Update listener name from %s to %s", tempPort.listener.Name, listenerNameV1)
		}

		if needUpdateListener {
			_, err := albProvider.UpdateListener(updateListener, tempPort.listener.Id)
			if err != nil {
				msg := fmt.Sprintf("Update loadbalancer(%s) listener(%s) error: %v", service.Spec.LoadBalancerIP, tempPort.listener.Name, err)
				sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
				errs = append(errs, err)
				continue
			}
		}

		// Step 1: check protocol which is not allowed to update
		if ELBProtocol(tempPort.servicePort.Protocol) != tempPort.listener.Protocol {
			msg := fmt.Sprintf("The protocol of listener(%s) can not be modified", tempPort.listener.Id)
			sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
			err := fmt.Errorf("%s", msg)
			errs = append(errs, err)
			continue
		}
		pool, err := albProvider.findPoolOfListener(tempPort.listener.Id)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Get pool of listener(%s) error: %v", tempPort.listener.Id, err)
			sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
			continue
		}
		if pool == nil {
			poolName := GetPoolNameV1(service, tempPort.servicePort)
			poolDesc := ELBPoolDescription{
				ClusterID: os.Getenv(ClusterID),
				ServiceID: string(service.UID),
				Port:      tempPort.servicePort.TargetPort.IntVal,
				Attention: Attention,
			}
			d, _ := json.Marshal(&poolDesc)
			poolConf := ALBPool{
				Name:         poolName,
				LbAlgorithm:  c.AgConfig.LBAlgorithm,
				Protocol:     ELBProtocol(tempPort.servicePort.Protocol),
				ListenerId:   tempPort.listener.Id,
				AdminStateUp: true,
				Description:  string(d),
			}

			//set session affinity
			if c.SAConfig.LBSessionAffinityType != "" {
				poolConf.SessionPersistence.Type = c.SAConfig.LBSessionAffinityType
				switch poolConf.SessionPersistence.Type {
				case ELBSessionSource:
					timeout := c.SAConfig.LBSessionAffinityOption.PersistenceTimeout
					poolConf.SessionPersistence.PersistenceTimeout = timeout
				}
			}

			pool, err = albProvider.CreatePool(&poolConf)
			if err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Create pool of listener(%s) error: %v", tempPort.listener.Id, err)
				sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
				continue
			}
		}

		//step 2: update health monitor
		if !c.HMConfig.LBHealthCheckStatus {
			//clean up exist monitor
			if pool.HealthMonitorId != "" {
				err = albProvider.DeleteHealthMonitor(pool.HealthMonitorId)
				if err != nil {
					errs = append(errs, err)
					msg := fmt.Sprintf("Delete health Monitor of pool(%s) error: %v", pool.Id, err)
					sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
					continue
				}
			}
		} else {
			portHMOption := c.HMConfig.LBHealthCheckOption
			if pool.HealthMonitorId == "" { //create new health monitor
				//create health monitor
				healthMonitorConf := ALBHealthMonitor{
					Type:         portHMOption.Protocol,
					PoolId:       pool.Id,
					Delay:        portHMOption.Delay,
					Timeout:      portHMOption.Timeout,
					MaxRetries:   portHMOption.MaxRetries,
					MonitorPort:  portHMOption.CheckPort,
					UrlPath:      portHMOption.UrlPath,
					AdminStateUp: true,
				}

				_, err = albProvider.CreateHealthMonitor(&healthMonitorConf)
				if err != nil {
					errs = append(errs, err)
					msg := fmt.Sprintf("Create health Monitor of pool(%s) error: %v", pool.Id, err)
					sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
					continue
				}
				klog.Infof("Create health monitor of loadbalancer/pool(%s:%s) success.", service.Spec.LoadBalancerIP, pool.Id)
			} else { //update exist health monitor
				hm, err := albProvider.GetHealthMonitor(pool.HealthMonitorId)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				compareMonitorPort := func(p1 *int, p2 *int) bool {
					if p1 == nil && p2 == nil {
						return true
					}
					if p1 != nil && p2 != nil {
						return *p1 == *p2
					}
					return false
				}

				albHM := &ALBHealthMonitor{}
				if portHMOption.Delay != hm.Delay ||
					portHMOption.Timeout != hm.Timeout ||
					portHMOption.MaxRetries != hm.MaxRetries ||
					!compareMonitorPort(portHMOption.CheckPort, hm.MonitorPort) ||
					portHMOption.UrlPath != hm.UrlPath {
					albHM.Delay = portHMOption.Delay
					albHM.Timeout = portHMOption.Timeout
					albHM.MaxRetries = portHMOption.MaxRetries
					albHM.MonitorPort = portHMOption.CheckPort
					albHM.UrlPath = portHMOption.UrlPath

					_, err = albProvider.UpdateHealthMonitor(albHM, hm.Id)
					if err != nil {
						errs = append(errs, err)
						msg := fmt.Sprintf("Update health Monitor of pool(%s) error: %v", pool.Id, err)
						sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
						continue
					}
					klog.Infof("Update health monitor of loadbalancer/pool(%s:%s) success.", service.Spec.LoadBalancerIP, pool.Id)
				}
			}
		}

		// Step 3: update pool's info
		var tempPool *ALBPool
		needUpdatePool := false
		tempPool = &ALBPool{}

		poolDesc := ELBPoolDescription{
			ClusterID: os.Getenv(ClusterID),
			ServiceID: string(service.UID),
			Port:      tempPort.servicePort.TargetPort.IntVal,
			Attention: Attention,
		}
		d, _ = json.Marshal(&poolDesc)
		if pool.Description != string(d) {
			needUpdatePool = true
			tempPool.Description = string(d)
		}

		poolNameV1 := GetPoolNameV1(service, tempPort.servicePort)
		if pool.Name == "" {
			needUpdatePool = true
			tempPool.Name = poolNameV1
			klog.Infof("Update Pool(%s/%s) name to %s",
				pool.ListenerId, pool.Id, poolNameV1)
		}

		//update persistence info
		timeOut := c.SAConfig.LBSessionAffinityOption.PersistenceTimeout
		if pool.LbAlgorithm != c.AgConfig.LBAlgorithm {
			needUpdatePool = true
			tempPool.LbAlgorithm = c.AgConfig.LBAlgorithm
			tempPool.SessionPersistence.Type = c.SAConfig.LBSessionAffinityType
			switch tempPool.SessionPersistence.Type {
			case ELBSessionSource:
				tempPool.SessionPersistence.PersistenceTimeout = timeOut
			}
		} else {
			switch c.SAConfig.LBSessionAffinityType {
			case ELBSessionSource:
				if pool.SessionPersistence.Type != ELBSessionSource ||
					pool.SessionPersistence.PersistenceTimeout != timeOut {
					needUpdatePool = true
					tempPool.SessionPersistence.Type = c.SAConfig.LBSessionAffinityType
					tempPool.SessionPersistence.PersistenceTimeout = timeOut
				}
			case "":
				if pool.SessionPersistence.Type != "" {
					needUpdatePool = true
					//tempPool = &ALBPool{}
				}
			}
		}

		if needUpdatePool {
			_, err = albProvider.UpdatePool(tempPool, pool.Id)
			if err != nil {
				errs = append(errs, err)
				msg := fmt.Sprintf("Update pool of listener(%s) error: %v", tempPort.listener.Id, err)
				sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
				continue
			}
		}

		//Step 4: update members
		protocolPort, err := alb.getMemberProtocolPort(service, *tempPort.servicePort)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Failed to get service(%s) ports, error: %v", service.Name, err)
			sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
			continue
		}
		//Step 3: update members
		curMembers := make(map[string]*ALBMember)
		for _, member := range members {
			member.ProtocolPort = protocolPort
			curMembers[member.Address] = member
		}

		preListMembers, err := albProvider.ListMembers(pool.Id)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Get member of listener/pool(%s/%s) error: %v", tempPort.listener.Id, pool.Id, err)
			sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
			continue
		}
		preMembers := make(map[string][]ALBMember)
		for _, member := range preListMembers.Members {
			preMembers[member.Address] = append(preMembers[member.Address], member)
		}

		addedMembers, err := alb.updateALbMembers(albProvider, service, curMembers, preMembers, pool.Id)
		if err != nil {
			errs = append(errs, err)
			msg := fmt.Sprintf("Update member of listener/pool(%s/%s) error: %v", tempPort.listener.Id, pool.Id, err)
			sendEvent(alb.eventRecorder, "UpdateLoadBalancerFailed", msg, service)
		}

		//addedMembers contain members new added
		if len(addedMembers) != 0 {
			alb.asyncWaitMembers(albProvider, service, pool.Id, addedMembers, true)
		}
	}

	klog.Infof("Update listeners of service(%s/%s) finish, total: %d, failed: %d",
		service.Namespace, service.Name, len(needsUpdate), len(errs))
	return utilerrors.NewAggregate(errs)
}

// help function for rollback resource, not care statusCode
func (alb *ALBCloud) deleteElb(albProvider *ALBClient, id string, service *v1.Service) {
	_, err := albProvider.DeleteLoadBalancer(id)
	if err != nil {
		msg := fmt.Sprintf("Delete elb(%s) error: %v", id, err)
		sendEvent(alb.eventRecorder, "DeleteLoadBalancerFailed", msg, service)
	}
}

// help function for rollback resource, not care statusCode
func (alb *ALBCloud) deleteEip(albProvider *ALBClient, id string, service *v1.Service) {
	_, err := albProvider.DeleteEip(id)
	if err != nil {
		msg := fmt.Sprintf("Delete eip(%s) error: %v", id, err)
		sendEvent(alb.eventRecorder, "DeleteLoadBalancerFailed", msg, service)
	}
}

// if error, then return origin service
func (alb *ALBCloud) updateService(service *v1.Service) (*v1.Service, error) {
	var err error
	serviceCopy := service.DeepCopy()
	for i := 0; i < MaxRetry; i++ {
		toUpdate := service.DeepCopy()
		toUpdate, err = alb.kubeClient.Services(toUpdate.Namespace).Update(context.TODO(), toUpdate, metav1.UpdateOptions{})
		if err == nil {
			return toUpdate, nil
		}

		if apierrors.IsNotFound(err) {
			klog.Infof("Not persisting update to service '%s/%s' that no longer exists: %v",
				service.Namespace, service.Name, err)
			return serviceCopy, err
		}

		if apierrors.IsConflict(err) {
			service, err = alb.kubeClient.Services(service.Namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
			if err != nil {
				service = serviceCopy
				klog.Warningf("Get service(%s/%s) error: %v", service.Namespace, service.Name, err)
				continue
			}
			service.Annotations[ELBEIPIDAnnotation] = serviceCopy.Annotations[ELBEIPIDAnnotation]
			service.Spec.LoadBalancerIP = serviceCopy.Spec.LoadBalancerIP
		}
		klog.Warningf("Failed to update service '%s/%s' after creating its load balancer: %v",
			service.Namespace, service.Name, err)
	}
	return serviceCopy, err
}

func (alb *ALBCloud) getClusterNeutronSubnetId(subnetId string) (string, error) {
	if neutronSubnetId, ok := alb.subnetMap[subnetId]; ok {
		return neutronSubnetId, nil
	}

	albProvider, err := alb.getALBClient()
	if err != nil {
		return "", err
	}

	subnet, err := albProvider.GetSubnet(subnetId)
	if err != nil {
		return "", err
	}
	if subnet.NeutronSubnetId != "" {
		alb.subnetMapLock.Lock()
		alb.subnetMap[subnetId] = subnet.NeutronSubnetId
		alb.subnetMapLock.Unlock()
		return subnet.NeutronSubnetId, nil
	}

	return "", fmt.Errorf("get cluster's neutron_subnet_id failed")
}

func (alb *ALBCloud) getElbNeutronSubnetIdFromElbIp(elbIp string) (string, error) {
	albProvider, err := alb.getALBClient()
	if err != nil {
		return "", err
	}

	params := map[string]string{"vpc_id": alb.config.VPCId}
	subnetList, err := albProvider.ListSubnets(params)
	if err != nil {
		return "", err
	}

	ip := net.ParseIP(elbIp)
	for _, subnet := range subnetList.Subnets {
		_, cidr, err := net.ParseCIDR(subnet.Cidr)
		if err != nil {
			klog.Warningf("parse cidr(%s) failed: %s", subnet.Cidr, err)
			continue
		}
		if cidr.Contains(ip) {
			return subnet.NeutronSubnetId, nil
		}
	}

	return "", fmt.Errorf("get elb'neutron_subnet_id failed")

}

func (alb *ALBCloud) getPrivateIpFromLoadbalancerIp(ipStr string) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("service loadbalancer IP is invalid: %s", ipStr)
	}

	albProvider, err := alb.getALBClient()
	if err != nil {
		return "", err
	}

	eips, err := albProvider.ListEips(nil)
	if err != nil {
		return "", err
	}

	for _, eip := range eips.Publicips {
		if eip.PublicIpAddress == ipStr && eip.PrivateIpAddress != "" {
			return eip.PrivateIpAddress, nil
		}
	}

	return ipStr, nil
}

func (alb *ALBCloud) getPublicIPFromPrivateIP(portID string) (string, error) {
	albProvider, err := alb.getALBClient()
	if err != nil {
		return "", err
	}
	params := make(map[string]string)
	params["port_id"] = portID
	eips, err := albProvider.ListEips(params)
	if err != nil {
		return "", err
	}
	if len(eips.Publicips) != 1 {
		return "", fmt.Errorf("get public ip from private ip failed")
	}

	return eips.Publicips[0].PublicIpAddress, nil
}

func (alb *ALBCloud) getLBSessionAffinity(service *v1.Service) (ELBSessionPersistenceType, LBSessionAffinityOption, error) {
	var lbSessionAffinityOption LBSessionAffinityOption
	switch af := GetSessionAffinityType(service); af {
	case ELBSessionSourceIP:
		affinityOptions := make(map[string]string)
		if option := GetSessionAffinityOptions(service); option != "" {
			err := json.Unmarshal([]byte(option), &affinityOptions)
			if err != nil {
				return "", lbSessionAffinityOption, fmt.Errorf("invalid session affinity option[%s],parse json failed", option)
			}
		}
		if val, ok := affinityOptions[ELBPersistenTimeout]; ok {
			timeout, err := strconv.Atoi(val)
			if err != nil ||
				timeout > ELBSessionSourceIPMaxTimeout ||
				timeout < ELBSessionSourceIPMinTimeout {
				return "", lbSessionAffinityOption, fmt.Errorf("invalid session affinity option,invalid cookie timeout [%d<timeout<%d]",
					ELBSessionSourceIPMinTimeout, ELBSessionSourceIPMaxTimeout)
			}
			lbSessionAffinityOption.PersistenceTimeout = timeout
		} else {
			lbSessionAffinityOption.PersistenceTimeout = ELBSessionSourceIPDefaultTimeout
		}
		return ELBSessionSource, lbSessionAffinityOption, nil
	case ELBSessionNone:
		return "", lbSessionAffinityOption, nil
	default:
		return "", lbSessionAffinityOption, fmt.Errorf("session affinity type:%s not support", af)
	}
}

func (alb *ALBCloud) getHealthCheckOption(service *v1.Service) (LBHealthCheckOption, error) {
	var healthcheckOption LBHealthCheckOption
	//set default health monitor

	var userHMOptions map[string]string
	if userOptions := GetHealthCheckOption(service); userOptions != "" {
		err := json.Unmarshal([]byte(userOptions), &userHMOptions)
		if err != nil {
			return healthcheckOption, fmt.Errorf("invalid health check option[%s],parse options failed",
				userOptions)
		}
	}

	if delayStr, ok := userHMOptions["delay"]; !ok {
		healthcheckOption.Delay = ELBHealthMonitorOptionDefaultDelay
	} else {
		delayInt, err := strconv.Atoi(delayStr)
		if err != nil || delayInt > ELBHealthMonitorOptionMaxDelay || delayInt < ELBHealthMonitorOptionMinDelay {
			return healthcheckOption, fmt.Errorf("invalid health check delay(%s) for service(%s/%s),"+
				"delay must in range[%d,%d]", delayStr, service.Namespace, service.Name,
				ELBHealthMonitorOptionMinDelay, ELBHealthMonitorOptionMaxDelay)
		}
		healthcheckOption.Delay = delayInt
	}

	if timeStr, ok := userHMOptions["timeout"]; !ok {
		healthcheckOption.Timeout = ELBHealthMonitorOptionDefaultTimeout
	} else {
		timeoutInt, err := strconv.Atoi(timeStr)
		if err != nil || timeoutInt < ELBHealthMonitorOptionMinTimeout || timeoutInt > ELBHealthMonitorOptionMaxTimeout {
			return healthcheckOption, fmt.Errorf("invalid health check timeout(%s) for service(%s/%s),"+
				"timeout must in range[%d,%d]", timeStr, service.Namespace, service.Name,
				ELBHealthMonitorOptionMinTimeout, ELBHealthMonitorOptionMaxTimeout)

		}
		healthcheckOption.Timeout = timeoutInt
	}

	if maxRetriesStr, ok := userHMOptions["max_retries"]; !ok {
		healthcheckOption.MaxRetries = ELBHealthMonitorOptionDefaultRetrys
	} else {
		maxRetriesInt, err := strconv.Atoi(maxRetriesStr)
		if err != nil || maxRetriesInt < ELBHealthMonitorOptionMinRetrys || maxRetriesInt > ELBHealthMonitorOptionMaxMRetrys {
			return healthcheckOption, fmt.Errorf("invalid health check max_retrys(%s) for service(%s/%s),"+
				"max_retrys must in range[%d,%d]", maxRetriesStr, service.Namespace, service.Name,
				ELBHealthMonitorOptionMinRetrys, ELBHealthMonitorOptionMaxMRetrys)
		}
		healthcheckOption.MaxRetries = maxRetriesInt
	}

	serviceProtocol := service.Spec.Ports[0].Protocol
	if protocol, ok := userHMOptions["protocol"]; !ok {
		if serviceProtocol == v1.ProtocolTCP {
			healthcheckOption.Protocol = ELBProtocolTCP
		} else {
			healthcheckOption.Protocol = ELBHealthMonitorTypeUDP
		}
	} else {
		switch protocol {
		case "TCP":
			healthcheckOption.Protocol = ELBProtocolTCP
		case "UDP":
			healthcheckOption.Protocol = ELBHealthMonitorTypeUDP
		case "HTTP":
			healthcheckOption.Protocol = ELBProtocolHTTP
		default:
			return healthcheckOption, fmt.Errorf("invalid health check protocol(%s) for service(%s/%s), "+
				"protocol must be TCP/UDP/HTTP", protocol, service.Namespace, service.Name)
		}
	}

	if urlPath, ok := userHMOptions["path"]; !ok {
		if healthcheckOption.Protocol == ELBProtocolHTTP {
			healthcheckOption.UrlPath = "/"
		}
	} else {
		//abandon url path when protocol is not http
		if healthcheckOption.Protocol == ELBProtocolHTTP {
			healthcheckOption.UrlPath = urlPath
		}
	}

	return healthcheckOption, nil
}

func (alb *ALBCloud) getLBHealthMonitor(service *v1.Service) (*LBHealthMonitorConfig, error) {
	var hmConfig LBHealthMonitorConfig

	status, err := getHealthCheckFlag(service)
	if err != nil {
		return nil, err
	}
	hmConfig.LBHealthCheckStatus = status
	option, err := alb.getHealthCheckOption(service)
	if err != nil {
		return nil, err
	}
	hmConfig.LBHealthCheckOption = option

	checkPort := GetHealthCheckPort(service)
	if checkPort != nil {
		nodePort := int(checkPort.NodePort)
		hmConfig.LBHealthCheckOption.CheckPort = &nodePort
	} else {
		hmConfig.LBHealthCheckOption.CheckPort = nil
	}

	return &hmConfig, nil
}

// If kubernetes.io/hws-hostNetwork = "true" and backend pod in mode localNetwork true ,
// we will add targetPort to listener , so that ELB can access to container direct
// Otherwise add nodeport to listener
func (alb *ALBCloud) getMemberProtocolPort(service *v1.Service, servicePort v1.ServicePort) (listenerPort int32, err error) {
	listenerPort = servicePort.NodePort
	if !isHostNetworkService(service) {
		return listenerPort, nil
	}

	podList, err := alb.getPods(service.Name, service.Namespace)
	if err != nil {
		return listenerPort, err
	}

	if len(podList.Items) == 0 {
		return listenerPort, fmt.Errorf("can't find backend pod with serivce (%s)", service.Name)
	}

	if podList.Items[0].Spec.HostNetwork {
		tmpPort, err := podutil.FindPort(&podList.Items[0], &servicePort)
		if err != nil {
			return listenerPort, err
		}
		listenerPort = int32(tmpPort)
	}

	return listenerPort, nil
}
