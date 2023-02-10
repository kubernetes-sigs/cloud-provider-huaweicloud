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

package config

import (
	"context"
	"encoding/json"
	"fmt"

	elbmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v2/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/utils/metadata"
)

const (
	providerNamespace     = "huawei-cloud-provider"
	loadbalancerConfigMap = "loadbalancer-config"

	HealthCheckTimeout    = 3
	HealthCheckMaxRetries = 3
	HealthCheckDelay      = 5
)

type LoadbalancerConfig struct {
	LoadBalancerOpts LoadBalancerOptions `json:"loadBalancerOption"`
	NetworkingOpts   NetworkingOptions   `json:"networkingOption"`
	MetadataOpts     MetadataOptions     `json:"metadataOption"`
}

type LoadBalancerOptions struct {
	LBAlgorithm string `json:"lb-algorithm"`
	LBProvider  string `json:"lb-provider"`
	KeepEIP     bool   `json:"keep-eip"`

	EnableCrossVpc bool   `json:"enable-cross-vpc"`
	L4FlavorID     string `json:"l4-flavor-id"`
	L7FlavorID     string `json:"l7-flavor-id"`

	SessionAffinityOption elbmodel.SessionPersistence `json:"session-affinity-option"`
	SessionAffinityFlag   string                      `json:"session-affinity-flag"`

	EnableTransparentClientIP bool `json:"enable-transparent-client-ip"`

	IdleTimeout     int `json:"idle-timeout"`
	RequestTimeout  int `json:"request-timeout"`
	ResponseTimeout int `json:"response-timeout"`

	HealthCheckFlag   string            `json:"health-check-flag"`
	HealthCheckOption HealthCheckOption `json:"health-check-option"`
}

type HealthCheckOption struct {
	Enable     bool   `json:"enable"`
	Delay      int32  `json:"delay"`
	Timeout    int32  `json:"timeout"`
	MaxRetries int32  `json:"max_retries"`
	Protocol   string `json:"protocol"`
	Path       string `json:"path"`
}

// NetworkingOptions is used for networking settings
type NetworkingOptions struct {
	PublicNetworkName   []string `json:"public-network-name"`
	InternalNetworkName []string `json:"internal-network-name"`
}

// MetadataOptions is used for configuring how to talk to metadata service or authConfig drive
type MetadataOptions struct {
	SearchOrder string `json:"search-order"`
}

func NewDefaultELBConfig() *LoadbalancerConfig {
	cfg := &LoadbalancerConfig{}
	cfg.MetadataOpts.initDefaultValue()
	cfg.LoadBalancerOpts.initDefaultValue()
	return cfg
}

func LoadElbConfigFromCM() (*LoadbalancerConfig, error) {
	defaultCfg := NewDefaultELBConfig()
	kubeClient, err := getKubeClient()
	if err != nil {
		return defaultCfg, err
	}

	configMap, err := kubeClient.ConfigMaps(providerNamespace).
		Get(context.TODO(), loadbalancerConfigMap, metav1.GetOptions{})
	if err != nil {
		return defaultCfg, err
	}

	klog.Infof("get loadbalancer options: %v", configMap.Data)

	return LoadELBConfig(configMap.Data), nil
}

func LoadELBConfig(data map[string]string) *LoadbalancerConfig {
	cfg := NewDefaultELBConfig()

	loadBalancerOptions := []byte(data["loadBalancerOption"])
	if err := json.Unmarshal(loadBalancerOptions, &cfg.LoadBalancerOpts); err != nil {
		klog.Errorf("error parsing loadbalancer config: %s", err)
	}
	networkingOptions := []byte(data["networkingOption"])
	if err := json.Unmarshal(networkingOptions, &cfg.NetworkingOpts); err != nil {
		klog.Errorf("error parsing networkingOption config: %s", err)
	}
	metadataOption := []byte(data["metadataOption"])
	if err := json.Unmarshal(metadataOption, &cfg.MetadataOpts); err != nil {
		klog.Errorf("error parsing metadataOption config: %s", err)
	}
	return cfg
}

func getKubeClient() (*corev1.CoreV1Client, error) {
	clusterCfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Errorf("Initial cluster configuration failed with error: %v", err)
		return nil, err
	}

	kubeClient, err := corev1.NewForConfig(clusterCfg)
	if err != nil {
		return nil, err
	}
	return kubeClient, err
}

func (l *LoadBalancerOptions) initDefaultValue() {
	if l.LBProvider == "" {
		l.LBProvider = "vlb"
	}
	l.HealthCheckOption = HealthCheckOption{
		Timeout:    HealthCheckTimeout,
		MaxRetries: HealthCheckMaxRetries,
		Delay:      HealthCheckDelay,
	}
}

func (m *MetadataOptions) initDefaultValue() {
	if m.SearchOrder == "" {
		m.SearchOrder = fmt.Sprintf("%s,%s", metadata.MetadataID, metadata.ConfigDriveID)
	}
}
