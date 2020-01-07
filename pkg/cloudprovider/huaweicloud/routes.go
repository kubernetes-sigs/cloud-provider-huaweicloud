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

	"github.com/prometheus/common/log"
	cloudprovider "k8s.io/cloud-provider"
)

// NewRoutes creates a new route handler.
func NewRoutes() *Routes {
	return &Routes{}
}

// Routes implements the cloudprovider.Routes for Huawei Cloud.
type Routes struct {
}

// Check if our struct implements necessary interface
var _ cloudprovider.Routes = &Routes{}

// ListRoutes lists all managed routes that belong to the specified clusterName
func (r *Routes) ListRoutes(ctx context.Context, clusterName string) ([]*cloudprovider.Route, error) {
	log.Warnf("ListRoutes is called but not implemented. clusterName: %s", clusterName)

	return nil, nil
}

// CreateRoute creates the described managed route
// route.Name will be ignored, although the cloud-provider may use nameHint
// to create a more user-meaningful name.
func (r *Routes) CreateRoute(ctx context.Context, clusterName string, nameHint string, route *cloudprovider.Route) error {
	log.Warnf("CreateRoute is called but not implemented. clusterName: %s, nameHint: %s", clusterName, nameHint)

	return nil
}

// DeleteRoute deletes the specified managed route
// Route should be as returned by ListRoutes
func (r *Routes) DeleteRoute(ctx context.Context, clusterName string, route *cloudprovider.Route) error {
	log.Warnf("ListRoutes is called but not implemented. clusterName: %s", clusterName)

	return nil
}
