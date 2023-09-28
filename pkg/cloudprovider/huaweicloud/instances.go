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
	"regexp"
	"strings"

	ecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/common"
)

const (
	instanceShutoffStatus = "SHUTOFF"
)

var providerIDRegexp = regexp.MustCompile(`^` + ProviderName + `://([^/]+)$`)

type Instances struct {
	Basic
}

// NodeAddresses returns the addresses of the specified instance.
func (i *Instances) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	klog.Infof("NodeAddresses is called with name %s", name)
	instance, err := i.ecsClient.GetByNodeName(string(name))
	if err != nil {
		return nil, err
	}
	return i.NodeAddressesByProviderID(ctx, instance.Id)
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
func (i *Instances) NodeAddressesByProviderID(_ context.Context, providerID string) ([]v1.NodeAddress, error) {
	klog.Infof("NodeAddressesByProviderID is called witd provider ID %s", providerID)
	instanceID, err := parseInstanceID(providerID)
	if err != nil {
		return nil, err
	}

	interfaces, err := i.ecsClient.ListInterfaces(&ecsmodel.ListServerInterfacesRequest{ServerId: instanceID})
	if err != nil {
		return nil, err
	}

	instance, err := i.ecsClient.Get(instanceID)
	if err != nil {
		return nil, err
	}

	addresses, err := i.ecsClient.BuildAddresses(instance, interfaces, i.networkingOpts)
	if err != nil {
		return nil, err
	}

	klog.Infof("NodeAddresses(ID: %v) => %v", providerID, addresses)
	return addresses, nil
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
func (i *Instances) InstanceID(_ context.Context, name types.NodeName) (string, error) {
	klog.Infof("InstanceID is called with name %s", name)
	server, err := i.ecsClient.GetByNodeName(string(name))
	if err != nil {
		return "", err
	}
	return server.Id, nil
}

// InstanceType returns the type of the specified instance.
func (i *Instances) InstanceType(_ context.Context, name types.NodeName) (string, error) {
	klog.Infof("InstanceType is called with name %s", name)
	instance, err := i.ecsClient.GetByNodeName(string(name))
	if err != nil {
		return "", err
	}

	return getInstanceFlavor(instance)
}

func getInstanceFlavor(instance *ecsmodel.ServerDetail) (string, error) {
	if len(instance.Flavor.Name) > 0 {
		return instance.Flavor.Name, nil
	}
	if len(instance.Flavor.Id) > 0 {
		return instance.Flavor.Name, nil
	}

	return "", fmt.Errorf("flavor name/id not found")
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *Instances) InstanceTypeByProviderID(_ context.Context, providerID string) (string, error) {
	klog.Infof("InstanceTypeByProviderID is called with provider ID %s", providerID)
	instanceID, err := parseInstanceID(providerID)
	if err != nil {
		return "", err
	}

	instance, err := i.ecsClient.Get(instanceID)
	if err != nil {
		return "", err
	}

	return getInstanceFlavor(instance)
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (i *Instances) AddSSHKeyToAllInstances(_ context.Context, _ string, _ []byte) error {
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (i *Instances) CurrentNodeName(_ context.Context, hostname string) (types.NodeName, error) {
	klog.Infof("CurrentNodeName is called, hostname: %s", hostname)
	return types.NodeName(hostname), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
func (i *Instances) InstanceExistsByProviderID(_ context.Context, providerID string) (bool, error) {
	klog.Infof("InstanceExistsByProviderID is called with provider ID %s", providerID)
	instanceID, err := parseInstanceID(providerID)
	if err != nil {
		return false, err
	}

	_, err = i.ecsClient.Get(instanceID)
	if err != nil {
		if common.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (i *Instances) InstanceShutdownByProviderID(_ context.Context, providerID string) (bool, error) {
	klog.Infof("InstanceShutdownByProviderID is called with provider ID %s", providerID)
	instanceID, err := parseInstanceID(providerID)
	if err != nil {
		return false, err
	}
	server, err := i.ecsClient.Get(instanceID)
	if err != nil {
		return false, err
	}

	return server.Status == instanceShutoffStatus, nil
}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
func (i *Instances) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	klog.Infof("InstanceExists is called with node %s/%s", node.Namespace, node.Name)
	return i.InstanceExistsByProviderID(ctx, node.Spec.ProviderID)
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
func (i *Instances) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	klog.Infof("InstanceShutdown is called with node %s/%s", node.Namespace, node.Name)
	return i.InstanceShutdownByProviderID(ctx, node.Spec.ProviderID)
}

// InstanceMetadata returns the instance's metadata. The values returned in InstanceMetadata are
// translated into specific fields in the Node object on registration.
func (i *Instances) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	klog.Infof("InstanceMetadata is called with node %s", node.Name)
	providerID := node.Spec.ProviderID
	if providerID == "" {
		klog.V(4).Infof("node.Spec.ProviderID is empty, query ECS details by hostname: %s", node.Name)
		id, err := i.InstanceID(ctx, types.NodeName(node.Name))
		if err != nil {
			return nil, err
		}
		providerID = id
	}
	instanceID, err := parseInstanceID(providerID)
	if err != nil {
		return nil, err
	}

	instance, err := i.ecsClient.Get(instanceID)
	if err != nil {
		return nil, err
	}

	instanceFlavor, err := getInstanceFlavor(instance)
	if err != nil {
		return nil, err
	}

	interfaces, err := i.ecsClient.ListInterfaces(&ecsmodel.ListServerInterfacesRequest{ServerId: instanceID})
	if err != nil {
		return nil, err
	}

	addresses, err := i.ecsClient.BuildAddresses(instance, interfaces, i.networkingOpts)
	if err != nil {
		return nil, err
	}

	return &cloudprovider.InstanceMetadata{
		ProviderID:    providerID,
		InstanceType:  instanceFlavor,
		NodeAddresses: addresses,
	}, nil
}

func parseInstanceID(providerID string) (string, error) {
	klog.Infof("parseInstanceID is called with providerID %s", providerID)

	if providerID != "" && !strings.Contains(providerID, "://") {
		providerID = ProviderName + "://" + providerID
	}

	matches := providerIDRegexp.FindStringSubmatch(providerID)
	if len(matches) != 2 {
		return "", fmt.Errorf("ProviderID \"%s\" didn't match expected format \"huaweicloud://InstanceID\"",
			providerID)
	}
	return matches[1], nil
}
