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

package wrapper

import (
	elb "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v3"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v3/model"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
)

/*
DedicatedLoadBalanceClient is the client of the V3 API.
The v3 version API is not fully compatible with the v2 version API,
So we need a transition period.
*/
type DedicatedLoadBalanceClient struct {
	AuthOpts *config.AuthOptions
}

/** Listeners **/

func (s *DedicatedLoadBalanceClient) CreateListener(req *model.CreateListenerOption) (*model.Listener, error) {
	var rst *model.Listener
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.CreateListener(&model.CreateListenerRequest{
			Body: &model.CreateListenerRequestBody{
				Listener: req,
			},
		})
	}, "Listener", &rst)

	return rst, err
}

func (s *DedicatedLoadBalanceClient) UpdateListener(id string, opt *model.UpdateListenerOption) error {
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.UpdateListener(&model.UpdateListenerRequest{
			ListenerId: id,
			Body: &model.UpdateListenerRequestBody{
				Listener: opt,
			},
		})
	})
}

func (s *DedicatedLoadBalanceClient) wrapper(handler func(*elb.ElbClient) (interface{}, error), args ...interface{}) error {
	return commonWrapper(func() (interface{}, error) {
		hc := s.AuthOpts.GetHcClient("elb")
		return handler(elb.NewElbClient(hc))
	}, OKCodes, args...)
}
