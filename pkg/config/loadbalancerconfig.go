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
	"fmt"

	"gopkg.in/gcfg.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/utils/metadata"
)

const (
	providerNamespace     = "huawei-cloud-provider"
	loadbalancerConfigMap = "loadbalancer-config"
)

type LoadbalancerConfig struct {
	LoadBalancerOpts LoadBalancerOptions `gcfg:"LoadBalancerOptions"`
	NetworkingOpts   NetworkingOptions   `gcfg:"NetworkingOptions"`
	MetadataOpts     MetadataOptions     `gcfg:"MetadataOptions"`
}

type LoadBalancerOptions struct {
	NetworkID string `gcfg:"network-id"`
	SubnetID  string `gcfg:"subnet-id"`

	LBAlgorithm           string `gcfg:"lb-algorithm"`
	SessionAffinityMode   string `gcfg:"session-affinity-mode"`
	SessionAffinityOption string `gcfg:"session-affinity-option"`

	LBProvider string `gcfg:"lb-provider"`
	FlavorID   string `gcfg:"flavor-id"`
	KeepEIP    bool   `gcfg:"keep-eip"`

	HealthCheck       string `gcfg:"health-check"`
	HealthCheckOption string `gcfg:"health-check-option"`
}

// NetworkingOptions is used for networking settings
type NetworkingOptions struct {
	PublicNetworkName   []string `gcfg:"public-network-name"`
	InternalNetworkName []string `gcfg:"internal-network-name"`
}

// MetadataOptions is used for configuring how to talk to metadata service or authConfig drive
type MetadataOptions struct {
	SearchOrder string `gcfg:"search-order"`
}

func NewDefaultELBConfig() *LoadbalancerConfig {
	cfg := &LoadbalancerConfig{}
	cfg.MetadataOpts.initDefaultValue()
	cfg.LoadBalancerOpts.initDefaultValue()
	return cfg
}

func LoadELBConfig(str string) (*LoadbalancerConfig, error) {
	cfg := NewDefaultELBConfig()
	if err := gcfg.ReadStringInto(cfg, str); err != nil {
		return nil, err
	}
	return cfg, nil
}

func LoadElbConfigFromCM() (*LoadbalancerConfig, error) {
	kubeClient, err := getKubeClient()
	if err != nil {
		return nil, err
	}

	configMap, err := kubeClient.ConfigMaps(providerNamespace).
		Get(context.TODO(), loadbalancerConfigMap, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	klog.Infof("get loadbalancer options: %s", configMap.Data["options"])
	return LoadELBConfig(configMap.Data["options"])
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
	if l.SessionAffinityMode == "" {
		l.SessionAffinityMode = "ROUND_ROBIN"
	}
}

func (m *MetadataOptions) initDefaultValue() {
	if m.SearchOrder == "" {
		m.SearchOrder = fmt.Sprintf("%s,%s", metadata.MetadataID, metadata.ConfigDriveID)
	}
}
