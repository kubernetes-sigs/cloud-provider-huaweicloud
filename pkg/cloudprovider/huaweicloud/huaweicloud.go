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
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	ecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
	gocache "github.com/patrickmn/go-cache"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/cloud-provider"
	"k8s.io/cloud-provider/options"
	servicehelper "k8s.io/cloud-provider/service/helpers"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/wrapper"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/common"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/utils"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/utils/mutexkv"
)

// Cloud provider name: PaaS Web Services.
const (
	ProviderName = "huaweicloud"

	LoadBalancerClass = "huaweicloud.com/elb"

	ElbClass = "kubernetes.io/elb.class"
	ElbID    = "kubernetes.io/elb.id"

	ElbSubnetID          = "kubernetes.io/elb.subnet-id"
	ElbEipID             = "kubernetes.io/elb.eip-id"
	ELBKeepEip           = "kubernetes.io/elb.keep-eip"
	AutoCreateEipOptions = "kubernetes.io/elb.eip-auto-create-option"

	ElbAlgorithm             = "kubernetes.io/elb.lb-algorithm"
	ElbSessionAffinityFlag   = "kubernetes.io/elb.session-affinity-flag"
	ElbSessionAffinityOption = "kubernetes.io/elb.session-affinity-option"

	ElbHealthCheckFlag    = "kubernetes.io/elb.health-check-flag"
	ElbHealthCheckOptions = "kubernetes.io/elb.health-check-option"

	ElbXForwardedHost      = "kubernetes.io/elb.x-forwarded-host"
	DefaultTLSContainerRef = "kubernetes.io/elb.default-tls-container-ref"

	ElbIdleTimeout     = "kubernetes.io/elb.idle-timeout"
	ElbRequestTimeout  = "kubernetes.io/elb.request-timeout"
	ElbResponseTimeout = "kubernetes.io/elb.response-timeout"

	NodeSubnetIDLabelKey = "node.kubernetes.io/subnetid"
	ELBMarkAnnotation    = "kubernetes.io/elb.mark"

	MaxRetry   = 3
	HealthzCCE = "cce-healthz"
	// Attention is a warning message that intended to set to auto-created instance, such as ELB listener.
	Attention = "It is auto-generated by cloud-provider-huaweicloud, do not modify!"

	ELBSessionNone        = ""
	ELBSessionSourceIP    = "SOURCE_IP"
	ELBPersistenceTimeout = "persistence_timeout"

	ELBSessionSourceIPDefaultTimeout = 60
	ELBSessionSourceIPMinTimeout     = 1
	ELBSessionSourceIPMaxTimeout     = 60

	ProtocolTCP             = "TCP"
	ProtocolUDP             = "UDP"
	ProtocolHTTP            = "HTTP"
	ProtocolHTTPS           = "HTTPS"
	ProtocolTerminatedHTTPS = "TERMINATED_HTTPS"

	endpointAdded  = "endpointAdded"
	endpointUpdate = "endpointUpdate"

	kubeSystemNamespace = "kube-system"
)

type ELBProtocol string
type ELBAlgorithm string

type Basic struct {
	cloudControllerManagerOpts *options.CloudControllerManagerOptions
	cloudConfig                *config.CloudConfig

	loadbalancerOpts *config.LoadBalancerOptions
	networkingOpts   *config.NetworkingOptions
	metadataOpts     *config.MetadataOptions

	sharedELBClient    *wrapper.SharedLoadBalanceClient
	dedicatedELBClient *wrapper.DedicatedLoadBalanceClient
	eipClient          *wrapper.EIpClient
	ecsClient          *wrapper.EcsClient
	vpcClient          *wrapper.VpcClient

	restConfig    *rest.Config
	kubeClient    *corev1.CoreV1Client
	eventRecorder record.EventRecorder

	mutexLock *mutexkv.MutexKV
}

func (b Basic) listPodsBySelector(ctx context.Context, namespace string, selectors map[string]string) (*v1.PodList, error) {
	labelSelector := labels.SelectorFromSet(selectors)
	opts := metav1.ListOptions{LabelSelector: labelSelector.String()}
	return b.kubeClient.Pods(namespace).List(ctx, opts)
}

func (b Basic) sendEvent(reason, msg string, service *v1.Service) {
	b.eventRecorder.Event(service, v1.EventTypeNormal, reason, msg)
}

func (b Basic) getSubnetID(service *v1.Service, node *v1.Node) (string, error) {
	subnetID, err := b.getNodeSubnetID(node)
	if err != nil {
		klog.Warningf("unable to read subnet-id from the node, try reading from service or cloud-config, error: %s", err)
	}
	if subnetID != "" {
		return subnetID, nil
	}

	subnetID = getStringFromSvsAnnotation(service, ElbSubnetID, b.cloudConfig.VpcOpts.SubnetID)
	if subnetID == "" {
		return "", status.Errorf(codes.InvalidArgument, "missing subnet-id, "+
			"can not to read subnet-id from service or cloud-config")
	}

	return subnetID, nil
}

func (b Basic) getNodeSubnetIDByHostIP(privateIP string) (string, error) {
	instance, err := b.ecsClient.GetByNodeIP(privateIP)
	if err != nil {
		return "", err
	}

	interfaces, err := b.ecsClient.ListInterfaces(&ecsmodel.ListServerInterfacesRequest{ServerId: instance.Id})
	if err != nil {
		return "", err
	}

	for _, inter := range interfaces {
		for _, fixedIP := range *inter.FixedIps {
			if fixedIP.IpAddress != nil && *fixedIP.IpAddress == privateIP {
				return *fixedIP.SubnetId, nil
			}
		}
	}

	return "", fmt.Errorf("failed to get node subnet ID with private IP: %s", privateIP)
}

func (b Basic) getNodeSubnetID(node *v1.Node) (string, error) {
	ipAddress, err := getNodeAddress(node)
	if err != nil {
		return "", err
	}

	instance, err := b.ecsClient.GetByNodeName(node.Name)
	if err != nil {
		return "", err
	}

	interfaces, err := b.ecsClient.ListInterfaces(&ecsmodel.ListServerInterfacesRequest{ServerId: instance.Id})
	if err != nil {
		return "", err
	}

	for _, inter := range interfaces {
		for _, fixedIP := range *inter.FixedIps {
			if fixedIP.IpAddress != nil && *fixedIP.IpAddress == ipAddress {
				return *fixedIP.SubnetId, nil
			}
		}
	}

	return "", fmt.Errorf("failed to get node subnet ID")
}

func (b Basic) updateService(service *v1.Service, lbStatus *v1.LoadBalancerStatus) {
	if service.Spec.LoadBalancerClass == nil || *service.Spec.LoadBalancerClass != LoadBalancerClass {
		return
	}

	if servicehelper.LoadBalancerStatusEqual(&service.Status.LoadBalancer, lbStatus) {
		return
	}
	updated := service.DeepCopy()
	updated.Status.LoadBalancer = *lbStatus

	klog.V(2).Infof("Patching status for service %s/%s", updated.Namespace, updated.Name)
	_, err := servicehelper.PatchService(b.kubeClient, service, updated)
	if err != nil {
		klog.Errorf("failed to patch status for service %s/%s", updated.Namespace, updated.Name)
	}
}

func (b Basic) isSupportedSvc(svs *v1.Service) bool {
	if svs.Spec.Type != v1.ServiceTypeLoadBalancer {
		return false
	}

	if svs.Spec.LoadBalancerClass != nil && *svs.Spec.LoadBalancerClass != LoadBalancerClass {
		klog.Infof("Ignoring service %s/%s using loadbalancer class %s, it is not supported by this controller",
			svs.Namespace, svs.Name, *svs.Spec.LoadBalancerClass)
		return false
	}

	if svs.Spec.LoadBalancerClass == nil && b.loadbalancerOpts.LoadBalancerClass != "" {
		return false
	}

	return true
}

func (b Basic) getPrimaryIP(ip string) (string, error) {
	if b.loadbalancerOpts.PrimaryNic != "force" {
		return ip, nil
	}

	instance, err := b.ecsClient.GetByNodeIPNew(ip)
	if err != nil {
		return "", err
	}
	for _, arr := range instance.Addresses {
		for _, v := range arr {
			if v.Primary {
				klog.Infof("obtain the ECS details through the private IP: %s, and find the primary network card IP: %s", ip, v.Addr)
				return v.Addr, nil
			}
		}
	}
	return "", status.Errorf(codes.NotFound, "not found ECS primary network by private ip: %s", ip)
}

type CloudProvider struct {
	Basic
	providers map[LoadBalanceVersion]cloudprovider.LoadBalancer
}

type LoadBalanceVersion int

const (
	VersionNotNeedLB LoadBalanceVersion = iota // if the service type is not LoadBalancer
	VersionELB                                 // classic load balancer
	VersionShared                              // enhanced load balancer(performance share)
	VersionDedicated                           // enhanced load balancer(performance guarantee)
	VersionNAT                                 // network address translation
)

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		hwsCloud, err := NewHWSCloud(config)
		if err != nil {
			return nil, err
		}
		return hwsCloud, nil
	})
}

func NewHWSCloud(cfg io.Reader) (*CloudProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("huaweicloud provider config is nil")
	}

	cloudConfig, err := config.ReadConfig(cfg)
	if err != nil {
		klog.Fatalf("failed to read AuthOpts CloudConfig: %v", err)
		return nil, err
	}

	elbCfg, err := config.LoadElbConfigFromCM()
	if err != nil {
		klog.Errorf("failed to read loadbalancer config: %v", err)
	}

	klog.Infof("get loadbalancer config: %#v", elbCfg)

	restConfig, kubeClient, err := newKubeClient()
	if err != nil {
		return nil, err
	}

	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: corev1.New(kubeClient.RESTClient()).Events("")})
	recorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "cloud-provider-huaweicloud"})

	ccmOpts, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to init CloudControllerManagerOptions: %s", err)
	}

	basic := Basic{
		cloudControllerManagerOpts: ccmOpts,
		cloudConfig:                cloudConfig,

		loadbalancerOpts: &elbCfg.LoadBalancerOpts,
		networkingOpts:   &elbCfg.NetworkingOpts,
		metadataOpts:     &elbCfg.MetadataOpts,

		sharedELBClient:    &wrapper.SharedLoadBalanceClient{AuthOpts: &cloudConfig.AuthOpts},
		dedicatedELBClient: &wrapper.DedicatedLoadBalanceClient{AuthOpts: &cloudConfig.AuthOpts},
		eipClient:          &wrapper.EIpClient{AuthOpts: &cloudConfig.AuthOpts},
		ecsClient:          &wrapper.EcsClient{AuthOpts: &cloudConfig.AuthOpts},
		vpcClient:          &wrapper.VpcClient{AuthOpts: &cloudConfig.AuthOpts},

		restConfig:    restConfig,
		kubeClient:    kubeClient,
		eventRecorder: recorder,
		mutexLock:     mutexkv.NewMutexKV(),
	}

	hws := &CloudProvider{
		Basic:     basic,
		providers: map[LoadBalanceVersion]cloudprovider.LoadBalancer{},
	}
	err = hws.listenerDeploy()
	if err != nil {
		return nil, err
	}

	hws.providers[VersionELB] = &ELBCloud{Basic: basic}
	hws.providers[VersionShared] = &SharedLoadBalancer{Basic: basic}
	hws.providers[VersionDedicated] = &DedicatedLoadBalancer{Basic: basic}
	hws.providers[VersionNAT] = &NATCloud{Basic: basic}

	return hws, nil
}

func newKubeClient() (*rest.Config, *corev1.CoreV1Client, error) {
	clusterCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("initial cluster configuration failed with error: %v", err)
	}

	kubeClient, err := corev1.NewForConfig(clusterCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create kubeClient failed with error: %v", err)
	}

	return clusterCfg, kubeClient, nil
}

func (h *CloudProvider) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	if !h.isSupportedSvc(service) {
		return nil, false, cloudprovider.ImplementedElsewhere
	}

	lbVersion, err := getLoadBalancerVersion(service)
	if err != nil {
		return nil, false, err
	}

	provider, exist := h.providers[lbVersion]
	if !exist {
		return nil, false, nil
	}

	return provider.GetLoadBalancer(ctx, clusterName, service)
}

func (h *CloudProvider) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	if !h.isSupportedSvc(service) {
		return ""
	}

	lbVersion, err := getLoadBalancerVersion(service)
	if err != nil {
		return ""
	}

	provider, exist := h.providers[lbVersion]
	if !exist {
		return ""
	}

	return provider.GetLoadBalancerName(ctx, clusterName, service)
}

// nolint: revive
func (h *CloudProvider) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	if !h.isSupportedSvc(service) {
		return nil, cloudprovider.ImplementedElsewhere
	}
	key := fmt.Sprintf("%s/%s", service.Namespace, service.Name)
	h.mutexLock.Lock(key)
	defer h.mutexLock.Unlock(key)

	lbVersion, err := getLoadBalancerVersion(service)
	if err != nil {
		return nil, err
	}

	provider, exist := h.providers[lbVersion]
	if !exist {
		return nil, nil
	}

	return provider.EnsureLoadBalancer(ctx, clusterName, service, nodes)
}

func (h *CloudProvider) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	if !h.isSupportedSvc(service) {
		return cloudprovider.ImplementedElsewhere
	}
	key := fmt.Sprintf("%s/%s", service.Namespace, service.Name)
	h.mutexLock.Lock(key)
	defer h.mutexLock.Unlock(key)

	lbVersion, err := getLoadBalancerVersion(service)
	if err != nil {
		return err
	}

	provider, exist := h.providers[lbVersion]
	if !exist {
		return nil
	}

	return provider.UpdateLoadBalancer(ctx, clusterName, service, nodes)
}

func (h *CloudProvider) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	if !h.isSupportedSvc(service) {
		return cloudprovider.ImplementedElsewhere
	}
	key := fmt.Sprintf("%s/%s", service.Namespace, service.Name)
	h.mutexLock.Lock(key)
	defer h.mutexLock.Unlock(key)

	lbVersion, err := getLoadBalancerVersion(service)
	if err != nil {
		return err
	}

	provider, exist := h.providers[lbVersion]
	if !exist {
		return nil
	}

	return provider.EnsureLoadBalancerDeleted(ctx, clusterName, service)
}

func getLoadBalancerVersion(service *v1.Service) (LoadBalanceVersion, error) {
	class := service.Annotations[ElbClass]

	switch class {
	case "elasticity":
		klog.Infof("Load balancer Version I for service %v", service.Name)
		return VersionELB, nil
	case "shared":
		klog.Infof("Shared load balancer for service %v", service.Name)
		return VersionShared, nil
	case "dedicated":
		klog.Infof("Dedicated Load balancer for service %v", service.Name)
		return VersionDedicated, nil
	case "dnat":
		klog.Infof("DNAT for service %v", service.Name)
		return VersionNAT, nil
	default:
		return 0, fmt.Errorf("unknow load balancer elb.class: %s", class)
	}
}

// type Instances interface {}

// ExternalID returns the cloud provider ID of the specified instance (deprecated).
func (*CloudProvider) ExternalID(_ context.Context, _ types.NodeName) (string, error) {
	return "", cloudprovider.NotImplemented
}

// HasClusterID returns true if the cluster has a clusterID
func (*CloudProvider) HasClusterID() bool {
	return true
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the cloud provider.
func (*CloudProvider) Initialize(_ cloudprovider.ControllerClientBuilder, _ <-chan struct{}) {
}

// TCPLoadBalancer returns an implementation of TCPLoadBalancer for Huawei Web Services.
func (h *CloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	// Only services with LoadBalancerClass=huaweicloud.com/elb are processed.
	if h.loadbalancerOpts.LoadBalancerClass != "" {
		return nil, false
	}

	return h, true
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (h *CloudProvider) Instances() (cloudprovider.Instances, bool) {
	instance := &Instances{
		Basic: h.Basic,
	}

	return instance, true
}

// Zones returns an implementation of Zones for Huawei Web Services.
func (*CloudProvider) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

// Clusters returns an implementation of Clusters for Huawei Web Services.
func (h *CloudProvider) Clusters() (cloudprovider.Clusters, bool) {
	return h, true
}

// Routes returns an implementation of Routes for Huawei Web Services.
func (*CloudProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (*CloudProvider) ProviderName() string {
	return ProviderName
}

// InstancesV2 is an implementation for instances and should only be implemented by external cloud providers.
// Don't support this feature for now.
func (h *CloudProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	instance := &Instances{
		Basic: h.Basic,
	}

	return instance, true
}

// ListClusters is an implementation of Clusters.ListClusters
func (*CloudProvider) ListClusters(_ context.Context) ([]string, error) {
	return nil, nil
}

// Master is an implementation of Clusters.Master
func (*CloudProvider) Master(_ context.Context, _ string) (string, error) {
	return "", nil
}

// util functions

func IsPodActive(p v1.Pod) bool {
	if v1.PodSucceeded != p.Status.Phase &&
		v1.PodFailed != p.Status.Phase &&
		p.DeletionTimestamp == nil {
		for _, c := range p.Status.Conditions {
			if c.Type == v1.PodReady && c.Status == v1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

type LoadBalancerServiceListener struct {
	Basic
	kubeClient  *corev1.CoreV1Client
	stopChannel chan struct{}

	goroutinePool       *common.ExecutePool
	serviceCache        map[string]*v1.Service
	invalidServiceCache *gocache.Cache
}

func (e *LoadBalancerServiceListener) stopListenerSlice() {
	klog.Warningf("Stop listening to Endpoints")
	e.stopChannel <- struct{}{}
}

// nolint: revive
func (e *LoadBalancerServiceListener) startEndpointListener(handle func(*v1.Service, bool)) {
	klog.Infof("starting EndpointListener")
	e.goroutinePool.Start()

	for {
		endpointsList, err := e.kubeClient.Endpoints(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{Limit: 1})

		if err != nil {
			klog.Errorf("failed to query a list of Endpoints, try again later, error: %s", err)
			time.Sleep(5 * time.Second)
			continue
		}

		endpointsInformer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					if options.ResourceVersion == "" || options.ResourceVersion == "0" {
						options.ResourceVersion = endpointsList.ResourceVersion
					}
					return e.kubeClient.Endpoints(metav1.NamespaceAll).List(context.TODO(), options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					if options.ResourceVersion == "" || options.ResourceVersion == "0" {
						options.ResourceVersion = endpointsList.ResourceVersion
					}
					return e.kubeClient.Endpoints(metav1.NamespaceAll).Watch(context.TODO(), options)
				},
			},
			&v1.Endpoints{},
			0,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		)

		_, err = endpointsInformer.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				newEndpoint := obj.(*v1.Endpoints)
				if newEndpoint.Name == "kube-scheduler" || newEndpoint.Name == "kube-controller-manager" ||
					newEndpoint.Name == "cloud-controller-manager" {
					return
				}
				klog.V(6).Infof("New Endpoints added, namespace: %s, name: %s", newEndpoint.Namespace, newEndpoint.Name)

				e.goroutinePool.Submit(func() {
					klog.V(6).Infof("process endpoints: %s / %s", newEndpoint.Namespace, newEndpoint.Name)
					e.dispatcher(newEndpoint.Namespace, newEndpoint.Name, endpointAdded, handle)
				})
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				newEndpoint := newObj.(*v1.Endpoints)
				if newEndpoint.Name == "kube-scheduler" || newEndpoint.Name == "kube-controller-manager" ||
					newEndpoint.Name == "cloud-controller-manager" {
					return
				}
				klog.V(6).Infof("Endpoint update, namespace: %s, name: %s", newEndpoint.Namespace, newEndpoint.Name)

				e.goroutinePool.Submit(func() {
					klog.V(6).Infof("process endpoints: %s / %s", newEndpoint.Namespace, newEndpoint.Name)
					e.dispatcher(newEndpoint.Namespace, newEndpoint.Name, endpointUpdate, handle)
				})
			},
			DeleteFunc: func(obj interface{}) {},
		}, 5*time.Second)
		if err != nil {
			klog.Errorf("failed to start EventHandler, try again later, error: %s", err)
			time.Sleep(5 * time.Second)
			continue
		}
		go endpointsInformer.Run(e.stopChannel)

		serviceInformer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					if options.ResourceVersion == "" || options.ResourceVersion == "0" {
						options.ResourceVersion = endpointsList.ResourceVersion
					}
					return e.kubeClient.Services(metav1.NamespaceAll).List(context.TODO(), options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					if options.ResourceVersion == "" || options.ResourceVersion == "0" {
						options.ResourceVersion = endpointsList.ResourceVersion
					}
					return e.kubeClient.Services(metav1.NamespaceAll).Watch(context.TODO(), options)
				},
			},
			&v1.Service{},
			0,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		)
		_, err = serviceInformer.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {},
			UpdateFunc: func(oldObj, newObj interface{}) {
				svs, _ := newObj.(*v1.Service)
				if svs.Spec.LoadBalancerClass == nil || !e.isSupportedSvc(svs) {
					return
				}

				klog.Infof("Found service was updated, namespace: %s, name: %s", svs.Namespace, svs.Name)

				e.goroutinePool.Submit(func() {
					klog.V(4).Infof("process endpoints: %s / %s", svs.Namespace, svs.Name)
					handle(svs, false)
				})
			},
			DeleteFunc: func(obj interface{}) {
				svs, _ := obj.(*v1.Service)
				if svs.Spec.LoadBalancerClass == nil || !e.isSupportedSvc(svs) {
					return
				}

				klog.Infof("Found service was deleted, namespace: %s, name: %s", svs.Namespace, svs.Name)
				e.goroutinePool.Submit(func() {
					klog.V(4).Infof("process endpoints: %s / %s", svs.Namespace, svs.Name)
					handle(svs, true)
				})
			},
		}, 5*time.Second)
		if err != nil {
			klog.Errorf("failed to start EventHandler, try again later, error: %s", err)
			time.Sleep(5 * time.Second)
			continue
		}
		go serviceInformer.Run(e.stopChannel)

		break
	}
	klog.Infof("EndpointListener started")
}

func (e *LoadBalancerServiceListener) dispatcher(namespace, name, eType string, handle func(*v1.Service, bool)) {
	key := fmt.Sprintf("%s/%s", namespace, name)
	if v, ok := e.invalidServiceCache.Get(key); ok {
		klog.V(6).Infof("Service %s/%s not found, will not try again within 10 minutes: %s", namespace, name, v)
		return
	}

	svc, err := e.kubeClient.Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to query service, error: %s", err)
		if strings.Contains(err.Error(), "not found") {
			e.invalidServiceCache.Set(key, err.Error(), 10*time.Minute)
		}
		return
	}

	if svc.Spec.Type != v1.ServiceTypeLoadBalancer || !e.isSupportedSvc(svc) {
		return
	}

	klog.Infof("Dispatcher service, namespace: %s, name: %s", namespace, name)

	if eType == endpointAdded && (svc.Spec.LoadBalancerClass == nil || *svc.Spec.LoadBalancerClass != LoadBalancerClass) {
		return
	}
	handle(svc, false)
}

// nolint: revive
func (h *CloudProvider) listenerDeploy() error {
	listener := LoadBalancerServiceListener{
		Basic:       h.Basic,
		kubeClient:  h.kubeClient,
		stopChannel: make(chan struct{}, 1),

		goroutinePool:       common.NewExecutePool(5),
		serviceCache:        make(map[string]*v1.Service),
		invalidServiceCache: gocache.New(5*time.Minute, 10*time.Minute),
	}

	secListener := &SecurityGroupListener{
		kubeClient:      h.kubeClient,
		ecsClient:       h.ecsClient,
		securityGroupID: h.cloudConfig.VpcOpts.SecurityGroupID,

		stopChannel: make(chan struct{}, 1),
	}

	clusterName := h.cloudControllerManagerOpts.KubeCloudShared.ClusterName
	id, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to elect leader in listener EndpointSlice : %s", err)
	}

	go leaderElection(id, h.restConfig, h.eventRecorder, func(_ context.Context) {
		go secListener.startSecurityGroupListener()

		listener.startEndpointListener(func(service *v1.Service, isDelete bool) {
			klog.Infof("Got service %s/%s using loadbalancer class %s",
				service.Namespace, service.Name, utils.ToString(service.Spec.LoadBalancerClass))

			if !h.isSupportedSvc(service) {
				return
			}

			if isDelete {
				err := h.EnsureLoadBalancerDeleted(context.TODO(), clusterName, service)
				if err != nil {
					klog.Errorf("failed to delete loadBalancer, service: %s/%s, error: %s", service.Namespace, service.Name, err)
					eventMsg := fmt.Sprintf("failed to clean listener for service: %s/%s, error: %s", service.Namespace, service.Name, err)
					h.sendEvent("DeleteLoadBalancer", eventMsg, service)
				}
				return
			}

			nodeList, err := h.kubeClient.Nodes().List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				klog.Errorf("failed to query node list: %s", err)
			}
			nodes := make([]*v1.Node, 0, len(nodeList.Items))
			for _, n := range nodeList.Items {
				node := n
				nodes = append(nodes, &node)
			}

			h.sendEvent("UpdateLoadBalancer", "Endpoints changed, start updating", service)

			err = h.UpdateLoadBalancer(context.TODO(), clusterName, service, nodes)
			if err == nil {
				lbStatus, exists, err := h.GetLoadBalancer(context.TODO(), clusterName, service)
				if err != nil || !exists {
					klog.Errorf("failed to get loadBalancer, service: %s/%s, exists: %v, error: %s", service.Namespace, service.Name, exists, err)
					return
				}
				h.updateService(service, lbStatus)
				return
			}

			// An error occurred while updating.
			if common.IsNotFound(err) || strings.Contains(err.Error(), "error, can not find a listener matching") {
				lbStatus, err := h.EnsureLoadBalancer(context.TODO(), clusterName, service, nodes)
				if err != nil {
					klog.Errorf("failed to ensure loadBalancer, service: %s/%s, error: %s", service.Namespace, service.Name, err)
					return
				}
				h.updateService(service, lbStatus)
				return
			}

			klog.Errorf("failed to synchronization endpoint, service: %s/%s, error: %s",
				service.Namespace, service.Name, err)
		})
	}, func() {
		listener.goroutinePool.Stop()
		listener.stopListenerSlice()
		secListener.stopSecurityGroupListener()
	})

	return nil
}

func leaderElection(id string, restConfig *rest.Config, recorder record.EventRecorder, onSuccess func(context.Context), onStop func()) {
	leaseName := "endpoint-slice-listener"
	leaseDuration := 60 * time.Second
	renewDeadline := 50 * time.Second
	retryPeriod := 30 * time.Second

	configmapLock, err := resourcelock.NewFromKubeconfig(resourcelock.LeasesResourceLock,
		kubeSystemNamespace,
		leaseName,
		resourcelock.ResourceLockConfig{
			Identity:      fmt.Sprintf("%s_%s", id, string(uuid.NewUUID())),
			EventRecorder: recorder,
		},
		restConfig,
		renewDeadline)
	if err != nil {
		klog.Fatalf("error creating lock: %v", err)
	}

	leaderelection.RunOrDie(context.TODO(), leaderelection.LeaderElectionConfig{
		Lock:          configmapLock,
		LeaseDuration: leaseDuration,
		RenewDeadline: renewDeadline,
		RetryPeriod:   retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.V(6).Infof("[Listener EndpointSlices] leader election got: %s", id)
				onSuccess(ctx)
			},
			OnStoppedLeading: func() {
				klog.Infof("[Listener EndpointSlices] leader election lost: %s", id)
				onStop()
			},
			OnNewLeader: func(identity string) {
				klog.Infof("[Listener EndpointSlices] leader chenged to %s", identity)
				if strings.Contains(identity, id) {
					return
				}
				onStop()
			},
		},
		Name: leaseName,
	})
}
