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

// nolint: revive
package wrapper

import (
	eip "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eip/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eip/v2/model"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
)

type EIpClient struct {
	AuthOpts *config.AuthOptions
}

func (e *EIpClient) Create(req *model.CreatePublicipRequestBody) (*model.PublicipCreateResp, error) {
	var rst *model.PublicipCreateResp
	err := e.wrapper(func(c *eip.EipClient) (interface{}, error) {
		return c.CreatePublicip(&model.CreatePublicipRequest{
			Body: req,
		})
	}, "Publicip", &rst)

	return rst, err
}

func (e *EIpClient) Get(id string) (*model.PublicipShowResp, error) {
	var rst *model.PublicipShowResp
	err := e.wrapper(func(c *eip.EipClient) (interface{}, error) {
		return c.ShowPublicip(&model.ShowPublicipRequest{PublicipId: id})
	}, "Publicip", &rst)

	return rst, err
}
func (e *EIpClient) List(req *model.ListPublicipsRequest) ([]model.PublicipShowResp, error) {
	var rst []model.PublicipShowResp
	err := e.wrapper(func(c *eip.EipClient) (interface{}, error) {
		return c.ListPublicips(req)
	}, "Publicips", &rst)

	return rst, err
}

func (e *EIpClient) Update(id string, opts *model.UpdatePublicipOption) error {
	return e.wrapper(func(c *eip.EipClient) (interface{}, error) {
		return c.UpdatePublicip(&model.UpdatePublicipRequest{
			PublicipId: id,
			Body: &model.UpdatePublicipsRequestBody{
				Publicip: opts,
			},
		})
	})
}

func (e *EIpClient) Bind(id, portID string) error {
	return e.Update(id, &model.UpdatePublicipOption{PortId: &portID})
}

func (e *EIpClient) Unbind(id string) error {
	portID := ""
	return e.Update(id, &model.UpdatePublicipOption{PortId: &portID})
}

func (e *EIpClient) Delete(id string) error {
	return e.wrapper(func(c *eip.EipClient) (interface{}, error) {
		return c.DeletePublicip(&model.DeletePublicipRequest{PublicipId: id})
	})
}

func (e *EIpClient) wrapper(handler func(*eip.EipClient) (interface{}, error), args ...interface{}) error {
	return commonWrapper(func() (interface{}, error) {
		hc := e.AuthOpts.GetHcClient("vpc")
		return handler(eip.NewEipClient(hc))
	}, OKCodes, args...)
}
