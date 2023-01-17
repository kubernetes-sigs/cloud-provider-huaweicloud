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
	"time"

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
	"k8s.io/klog/v2"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/wrapper"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/utils/mutexkv"
)

// Cloud provider name: PaaS Web Services.
const (
	ProviderName = "huaweicloud"

	ElbClass           = "kubernetes.io/elb.class"
	ElbID              = "kubernetes.io/elb.id"
	ElbConnectionLimit = "kubernetes.io/elb.connection-limit"

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
	DefaultTLSContainerRef = "kubernetes.io/default-tls-container-ref"

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

	ProtocolUDP             = "UDP"
	ProtocolHTTP            = "HTTP"
	ProtocolHTTPS           = "HTTPS"
	ProtocolTerminatedHTTPS = "TERMINATED_HTTPS"
)

type ELBProtocol string
type ELBAlgorithm string

type Basic struct {
	cloudControllerManagerOpts *options.CloudControllerManagerOptions
	cloudConfig                *config.CloudConfig
	loadBalancerConfig         *config.LoadbalancerConfig //nolint: unused

	loadbalancerOpts *config.LoadBalancerOptions
	networkingOpts   *config.NetworkingOptions
	metadataOpts     *config.MetadataOptions

	sharedELBClient *wrapper.SharedLoadBalanceClient
	eipClient       *wrapper.EIpClient
	ecsClient       *wrapper.EcsClient

	restConfig    *rest.Config
	kubeClient    *corev1.CoreV1Client
	eventRecorder record.EventRecorder
}

func (b Basic) listPodsBySelector(ctx context.Context, namespace string, selectors map[string]string) (*v1.PodList, error) {
	labelSelector := labels.SelectorFromSet(selectors)
	opts := metav1.ListOptions{LabelSelector: labelSelector.String()}
	return b.kubeClient.Pods(namespace).List(ctx, opts)
}

func (b Basic) sendEvent(reason, msg string, service *v1.Service) {
	b.eventRecorder.Event(service, v1.EventTypeNormal, reason, msg)
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
	VersionPLB                                 // enhanced load balancer(performance guarantee)
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
	recorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "hws-cloudprovider"})

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

		sharedELBClient: &wrapper.SharedLoadBalanceClient{AuthOpts: &cloudConfig.AuthOpts},
		eipClient:       &wrapper.EIpClient{AuthOpts: &cloudConfig.AuthOpts},
		ecsClient:       &wrapper.EcsClient{AuthOpts: &cloudConfig.AuthOpts},

		restConfig:    restConfig,
		kubeClient:    kubeClient,
		eventRecorder: recorder,
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
	// TODO(RainbowMango): Support PLB later.
	// hws.providers[VersionPLB] = &PLBCloud{lrucache: lrucache, config: &gConfig.LoadBalancer, kubeClient: kubeClient, clientPool: deprecateddynamic.NewDynamicClientPool(clientConfig), eventRecorder: recorder, subnetMap: map[string]string{}}
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

func (h *CloudProvider) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	LBVersion, err := getLoadBalancerVersion(service)
	if err != nil {
		return nil, false, err
	}

	provider, exist := h.providers[LBVersion]
	if !exist {
		return nil, false, nil
	}

	return provider.GetLoadBalancer(ctx, clusterName, service)
}

func (h *CloudProvider) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	LBVersion, err := getLoadBalancerVersion(service)
	if err != nil {
		return ""
	}

	provider, exist := h.providers[LBVersion]
	if !exist {
		return ""
	}

	return provider.GetLoadBalancerName(ctx, clusterName, service)
}

func (h *CloudProvider) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	LBVersion, err := getLoadBalancerVersion(service)
	if err != nil {
		return nil, err
	}

	provider, exist := h.providers[LBVersion]
	if !exist {
		return nil, nil
	}

	return provider.EnsureLoadBalancer(ctx, clusterName, service, nodes)
}

func (h *CloudProvider) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	LBVersion, err := getLoadBalancerVersion(service)
	if err != nil {
		return err
	}

	provider, exist := h.providers[LBVersion]
	if !exist {
		return nil
	}

	return provider.UpdateLoadBalancer(ctx, clusterName, service, nodes)
}

func (h *CloudProvider) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	LBVersion, err := getLoadBalancerVersion(service)
	if err != nil {
		return err
	}

	provider, exist := h.providers[LBVersion]
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
	case "shared", "":
		klog.Infof("Shared load balancer for service %v", service.Name)
		return VersionShared, nil
	case "performance":
		klog.Infof("Load balancer Version III for service %v", service.Name)
		return VersionPLB, nil
	case "dnat":
		klog.Infof("DNAT for service %v", service.Name)
		return VersionNAT, nil
	default:
		return 0, fmt.Errorf("Unknown elb.class: %s", class)
	}
}

// type Instances interface {}

// ExternalID returns the cloud provider ID of the specified instance (deprecated).
func (h *CloudProvider) ExternalID(ctx context.Context, instance types.NodeName) (string, error) {
	return "", cloudprovider.NotImplemented
}

// HasClusterID returns true if the cluster has a clusterID
func (h *CloudProvider) HasClusterID() bool {
	return true
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the cloud provider.
func (h *CloudProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

// TCPLoadBalancer returns an implementation of TCPLoadBalancer for Huawei Web Services.
func (h *CloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
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
func (h *CloudProvider) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

// Clusters returns an implementation of Clusters for Huawei Web Services.
func (h *CloudProvider) Clusters() (cloudprovider.Clusters, bool) {
	return h, true
}

// Routes returns an implementation of Routes for Huawei Web Services.
func (h *CloudProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (h *CloudProvider) ProviderName() string {
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
func (h *CloudProvider) ListClusters(ctx context.Context) ([]string, error) {
	return nil, nil
}

// Master is an implementation of Clusters.Master
func (h *CloudProvider) Master(ctx context.Context, clusterName string) (string, error) {
	return "", nil
}

//util functions

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

type EndpointSliceListener struct {
	stopChannel chan struct{}
	kubeClient  *corev1.CoreV1Client
	mutexLock   *mutexkv.MutexKV
}

func (e *EndpointSliceListener) stopListenerSlice() {
	klog.Warningf("Stop listening to Endpoints")
	close(e.stopChannel)
}

func (e *EndpointSliceListener) startEndpointListener(handle func(*v1.Service)) {
	klog.Infof("starting EndpointListener")
	for {
		endpointsList, err := e.kubeClient.Endpoints(metav1.NamespaceAll).
			List(context.TODO(), metav1.ListOptions{Limit: 1})

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

		queue := make(chan v1.Service, 50)
		endpointsInformer.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {},
			UpdateFunc: func(oldObj, newObj interface{}) {
				newEndpoint := newObj.(*v1.Endpoints)
				if newEndpoint.Name == "kube-scheduler" || newEndpoint.Name == "kube-controller-manager" ||
					newEndpoint.Name == "cloud-controller-manager" {
					return
				}
				klog.V(4).Infof("Update Endpoints, namespace: %s, name: %s", newEndpoint.Namespace, newEndpoint.Name)

				queue <- v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: newEndpoint.Namespace,
						Name:      newEndpoint.Name,
					},
				}
				go func() {
					s := <-queue
					klog.V(4).Infof("process endpoints: %s / %s", s.Namespace, s.Name)
					e.dispatcher(s.Namespace, s.Name, handle)
				}()
			},
			DeleteFunc: func(obj interface{}) {},
		}, 5*time.Second)
		go endpointsInformer.Run(e.stopChannel)
		break
	}
	klog.Infof("EndpointListener started")
}

func (e *EndpointSliceListener) dispatcher(namespace, name string, handle func(*v1.Service)) {
	key := fmt.Sprintf("%s/%s", namespace, name)
	e.mutexLock.Lock(key)
	defer e.mutexLock.Unlock(key)
	svc, err := e.kubeClient.Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	klog.Infof("Dispatcher service, namespace: %s, name: %s", namespace, name)
	if err != nil {
		klog.Errorf("failed to query service, error: %s", err)
	}
	handle(svc)
}

func (h *CloudProvider) listenerDeploy() error {
	listener := EndpointSliceListener{
		kubeClient: h.kubeClient,
		mutexLock:  mutexkv.NewMutexKV(),
	}

	clusterName := h.cloudControllerManagerOpts.KubeCloudShared.ClusterName
	id, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to elect leader in listener EndpointSlice : %s", err)
	}

	go leaderElection(id, h.restConfig, h.eventRecorder, func(ctx context.Context) {
		listener.startEndpointListener(func(service *v1.Service) {
			if service.Spec.Type != v1.ServiceTypeLoadBalancer {
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
			if err != nil {
				klog.Errorf("failed to synchronization endpoint, service: %s/%s, error: %s",
					service.Namespace, service.Name, err)
			}
		})
	}, func() {
		listener.stopListenerSlice()
	})
	return nil
}

func leaderElection(id string, restConfig *rest.Config, recorder record.EventRecorder, onSuccess func(context.Context), onStop func()) {
	leaseName := "endpoint-slice-listener"
	leaseDuration := 30 * time.Second
	renewDeadline := 20 * time.Second
	retryPeriod := 10 * time.Second

	configmapLock, err := resourcelock.NewFromKubeconfig(resourcelock.ConfigMapsLeasesResourceLock,
		config.ProviderNamespace,
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
				klog.V(4).Infof("[Listener EndpointSlices] leader election got: %s", id)
				onSuccess(ctx)
			},
			OnStoppedLeading: func() {
				klog.V(4).Infof("[Listener EndpointSlices] leader election lost: %s", id)
				onStop()
			},
		},
		Name: leaseName,
	})
}
