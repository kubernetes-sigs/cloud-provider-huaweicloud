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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"time"

	"github.com/hashicorp/golang-lru"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/cloud-provider"
	"k8s.io/klog"
)

const (
	providerName = "huaweicloud"
)

// CloudConfig is used to read and store information from the cloud configuration file
type CloudConfig struct {
	// TODO(RainbowMango): Auth options shall be split from LoadBalancer.
	LoadBalancer LoadBalancerOpts
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, newCloud)
}

func readConfig(config io.Reader) (*CloudConfig, error) {
	if config == nil {
		return nil, fmt.Errorf("no cloud provider config given")
	}

	configBytes, err := ioutil.ReadAll(config)
	if err != nil {
		return nil, fmt.Errorf("can not read cloud provider config: %v", err)
	}

	var cloudConfig CloudConfig
	err = json.Unmarshal(configBytes, &cloudConfig)
	if err != nil {
		return nil, fmt.Errorf("cloud config format is not expected: %v", err)
	}

	return &cloudConfig, nil
}

func newCloud(config io.Reader) (cloudprovider.Interface, error) {
	cloudConfig, err := readConfig(config)
	if err != nil {
		klog.Errorf("Create cloud provider failed: %v.", err)
		return nil, err
	}

	return NewHuaweiCloud(cloudConfig)
}

func NewHuaweiCloud(config *CloudConfig) (*HuaweiCloud, error) {
	clientConfig, err := clientcmd.BuildConfigFromFlags(config.LoadBalancer.Apiserver, "")
	if err != nil {
		return nil, err
	}

	kubeClient, err := corev1.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: corev1.New(kubeClient.RESTClient()).Events("")})
	recorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "hws-cloudprovider"})
	lrucache, err := lru.New(200)
	if err != nil {
		return nil, err
	}

	secretInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return kubeClient.Secrets(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.Secrets(metav1.NamespaceAll).Watch(options)
			},
		},
		&v1.Secret{},
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	secretInformer.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			kubeSecret := obj.(*v1.Secret)
			if kubeSecret.Name == config.LoadBalancer.SecretName {
				// TODO(RainbowMango): remove namespace from key
				key := kubeSecret.Namespace + "/" + kubeSecret.Name
				lrucache.Add(key, kubeSecret)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSecret := oldObj.(*v1.Secret)
			newSecret := newObj.(*v1.Secret)
			if newSecret.Name == config.LoadBalancer.SecretName {
				if reflect.DeepEqual(oldSecret.Data, newSecret.Data) {
					return
				}
				key := newSecret.Namespace + "/" + newSecret.Name
				lrucache.Add(key, newSecret)
			}
		},
		DeleteFunc: func(obj interface{}) {
			deleteSecret(obj, lrucache, config.LoadBalancer.SecretName)
		},
	}, 30*time.Second)

	go secretInformer.Run(nil)

	if !cache.WaitForCacheSync(nil, secretInformer.HasSynced) {
		klog.Errorf("failed to wait for HWSCloud to be synced")
	}

	hws := &HuaweiCloud{
		lrucache:      lrucache,
		config:        config,
		kubeClient:    kubeClient,
		eventRecorder: recorder,
	}

	return hws, nil
}

// HuaweiCloud is an implementation of cloud provider Interface for Huawei Cloud.
type HuaweiCloud struct {
	lrucache      *lru.Cache
	config        *CloudConfig
	kubeClient    corev1.CoreV1Interface
	eventRecorder record.EventRecorder
}

var _ cloudprovider.Interface = &HuaweiCloud{}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping or run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (h *HuaweiCloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {

}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (h *HuaweiCloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return NewLoadBalancer(h.lrucache, &h.config.LoadBalancer, h.kubeClient, h.eventRecorder), true
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (h *HuaweiCloud) Instances() (cloudprovider.Instances, bool) {
	// TODO(RainbowMango): waiting a solution about how to share openstack implementation and do minimum changes here.
	return nil, false
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
func (h *HuaweiCloud) Zones() (cloudprovider.Zones, bool) {
	// TODO(RainbowMango): waiting a solution about how to share openstack implementation and do minimum changes here.
	return nil, false
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (h *HuaweiCloud) Clusters() (cloudprovider.Clusters, bool) {
	// TODO(RainbowMango): waiting a solution about how to share openstack implementation and do minimum changes here.
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (h *HuaweiCloud) Routes() (cloudprovider.Routes, bool) {
	// TODO(RainbowMango): waiting a solution about how to share openstack implementation and do minimum changes here.
	return nil, false
}

// HuaweiCloudProviderName returns the cloud provider ID.
func (h *HuaweiCloud) ProviderName() string {
	return providerName
}

// HasClusterID returns true if a ClusterID is required and set
func (h *HuaweiCloud) HasClusterID() bool {
	return true
}

func deleteSecret(obj interface{}, lrucache *lru.Cache, secretName string) {
	kubeSecret, ok := obj.(*v1.Secret)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %#v", obj)
			return
		}
		kubeSecret, ok = tombstone.Obj.(*v1.Secret)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a secret %#v", obj)
			return
		}
	}

	if kubeSecret.Name == secretName {
		key := kubeSecret.Namespace + "/" + kubeSecret.Name
		lrucache.Remove(key)
	}
}
