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

// NewCluster creates a new cluster instance.
func NewCluster() *Cluster {
	return &Cluster{}
}

// Cluster is a cluster handler.
type Cluster struct {
}

// Check if our struct implements necessary interface
var _ cloudprovider.Clusters = &Cluster{}

// ListClusters lists the names of the available clusters.
func (c *Cluster) ListClusters(ctx context.Context) ([]string, error) {
	log.Warnf("ListClusters is called but not implemented.")
	return nil, nil
}

// Master gets back the address (either DNS name or IP address) of the master node for the cluster.
func (c *Cluster) Master(ctx context.Context, clusterName string) (string, error) {
	log.Warnf("ListClusters is called but not implemented. cluster name: %s", clusterName)
	return "", nil
}
