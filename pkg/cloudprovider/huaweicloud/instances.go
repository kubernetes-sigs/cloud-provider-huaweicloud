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

	"github.com/RainbowMango/huaweicloud-sdk-go"
	"github.com/RainbowMango/huaweicloud-sdk-go/auth/aksk"
	"github.com/RainbowMango/huaweicloud-sdk-go/openstack"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

// Instances encapsulates an implementation of Instances.
type Instances struct {
	Auth *AuthOpts
}

// Check if our Instances implements necessary interface
var _ cloudprovider.Instances = &Instances{}

// NodeAddresses returns the addresses of the specified instance.
// TODO(roberthbailey): This currently is only used in such a way that it
// returns the address of the calling instance. We should do a rename to
// make this clearer.
func (i *Instances) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	klog.Infof("NodeAddresses is called. input name: %s", name)
	return nil, nil
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node. The
// ProviderID is a unique identifier of the node. This will not be called
// from the node whose nodeaddresses are being queried. i.e. local metadata
// services cannot be used in this method to obtain nodeaddresses
func (i *Instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	klog.Infof("NodeAddressesByProviderID is called. input provider ID: %s", providerID)
	return []v1.NodeAddress{}, cloudprovider.NotImplemented
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist, we must return ("", cloudprovider.InstanceNotFound)
// cloudprovider.InstanceNotFound should NOT be returned for instances that exist but are stopped/sleeping
func (i *Instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	klog.Infof("InstanceID is called. input nodeName: %s", nodeName)
	return "", nil
}

// InstanceType returns the type of the specified instance.
func (i *Instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	klog.Infof("InstanceType is called. input name: %s", name)
	return "", nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *Instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	klog.Infof("InstanceTypeByProviderID is called. input provider ID: %s", providerID)
	return "", cloudprovider.NotImplemented
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (i *Instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	klog.Infof("AddSSHKeyToAllInstances is called. input user: %s, keyData: %v", user, keyData)
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (i *Instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	klog.Infof("CurrentNodeName is called. input hostname: %s, output node name: %s", hostname, hostname)
	return types.NodeName(hostname), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
// This method should still return true for instances that exist but are stopped/sleeping.
func (i *Instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.Infof("InstanceExistsByProviderID is called. input provider ID: %s", providerID)
	return false, cloudprovider.NotImplemented
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (i *Instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.Infof("InstanceShutdownByProviderID is called. input provider ID: %s", providerID)
	return false, cloudprovider.NotImplemented
}

func (i *Instances) getAKSKFromSecret() (accessKey string, secretKey string, secretToken string) {
	// TODO(RainbowMango): Get AK/SK as well as secret token from kubernetes.secret.
	return
}

func (i *Instances) getClient() *gophercloud.ServiceClient {
	accessKey := i.Auth.AccessKey
	secretKey := i.Auth.SecretKey
	secretToken := ""

	if len(i.Auth.SecretName) > 0 {
		accessKey, secretKey, secretToken = i.getAKSKFromSecret()
	}
	akskOpts := aksk.AKSKOptions{
		IdentityEndpoint: i.Auth.IAMEndpoint,
		ProjectID:        i.Auth.ProjectID,
		DomainID:         i.Auth.DomainID,
		Region:           i.Auth.Region,
		AccessKey:        accessKey,
		SecretKey:        secretKey,
		SecurityToken:    secretToken,
	}

	providerClient, err := openstack.AuthenticatedClient(akskOpts)
	if err != nil {
		klog.Errorf("init provider client failed with error: %v", err)
		return nil
	}

	serviceClient, err := openstack.NewComputeV2(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		klog.Errorf("init compute service client failed: %v", err)
		return nil
	}

	return serviceClient
}
