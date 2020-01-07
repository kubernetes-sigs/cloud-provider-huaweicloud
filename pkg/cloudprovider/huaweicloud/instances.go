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
	"errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

var ErrNotImplemented = errors.New("feature not implemented")

// NewInstances creates an instance handler.
func NewInstances() *Instances {
	return &Instances{}
}

// Instances encapsulates an implementation of Instances for Huawei Cloud.
type Instances struct {
}

// Check if our struct implements necessary interface
var _ cloudprovider.Instances = &Instances{}

// NodeAddresses returns the addresses of the specified instance.
func (i *Instances) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	klog.Warning("NodeAddresses is called, but this interface haven't been implemented. node: %s", name)
	return nil, nil
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node. The
// ProviderID is a unique identifier of the node. This will not be called
// from the node whose nodeaddresses are being queried. i.e. local metadata
// services cannot be used in this method to obtain nodeaddresses
func (i *Instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	klog.Warning("NodeAddressesByProviderID is called, but this interface haven't been implemented. providerID: %s", providerID)
	return []v1.NodeAddress{}, ErrNotImplemented
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist, we must return ("", cloudprovider.InstanceNotFound)
// cloudprovider.InstanceNotFound should NOT be returned for instances that exist but are stopped/sleeping
func (i *Instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	klog.Warning("InstanceID is called, but this interface haven't been implemented. node: %s", nodeName)
	return "", nil
}

// InstanceType returns the type of the specified instance.
func (i *Instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	klog.Warning("InstanceType is called, but this interface haven't been implemented. node: %s", name)
	return "", nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *Instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	klog.Warning("InstanceTypeByProviderID is called, but this interface haven't been implemented. providerID: %s", providerID)
	return "", ErrNotImplemented
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (i *Instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	klog.Warning("AddSSHKeyToAllInstances is called, but this interface haven't been implemented. user: %s", user)
	return ErrNotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (i *Instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	klog.Warning("CurrentNodeName is called, but this interface haven't been implemented. hostname: %s", hostname)
	return types.NodeName(hostname), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
// This method should still return true for instances that exist but are stopped/sleeping.
func (i *Instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.Warning("InstanceExistsByProviderID is called, but this interface haven't been implemented. providerID: %s", providerID)
	return false, ErrNotImplemented
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (i *Instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.Warning("InstanceShutdownByProviderID is called, but this interface haven't been implemented. providerID: %s", providerID)
	return false, ErrNotImplemented
}
