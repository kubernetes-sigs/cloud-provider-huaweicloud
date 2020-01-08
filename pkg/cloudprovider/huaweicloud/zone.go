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

	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

// NewZone creates a zone handler.
func NewZone() *Zone {
	return &Zone{}
}

// Zone represents the location of a particular machine.
type Zone struct {
}

// Check if our struct implements necessary interface
var _ cloudprovider.Zones = &Zone{}

// GetZone returns the Zone containing the current failure zone and locality region that the program is running in
// In most cases, this method is called from the kubelet querying a local metadata service to acquire its zone.
// For the case of external cloud providers, use GetZoneByProviderID or GetZoneByNodeName since GetZone
// can no longer be called from the kubelets.
func (z *Zone) GetZone(ctx context.Context) (cloudprovider.Zone, error) {
	klog.Warningf("GetZone is called but not implemented.")

	return cloudprovider.Zone{}, nil
}

// GetZoneByProviderID returns the Zone containing the current zone and locality region of the node specified by providerID
// This method is particularly used in the context of external cloud providers where node initialization must be done
// outside the kubelets.
func (z *Zone) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	klog.Warningf("GetZoneByProviderID is called but not implemented.")

	return cloudprovider.Zone{}, nil
}

// GetZoneByNodeName returns the Zone containing the current zone and locality region of the node specified by node name
// This method is particularly used in the context of external cloud providers where node initialization must be done
// outside the kubelets.
func (z *Zone) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	klog.Warningf("GetZoneByNodeName is called but not implemented.")

	return cloudprovider.Zone{}, nil
}
