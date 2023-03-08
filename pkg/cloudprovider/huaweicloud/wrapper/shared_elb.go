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
	"fmt"

	elb "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v2/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/common"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
)

type SharedLoadBalanceClient struct {
	AuthOpts *config.AuthOptions
}

/** ELB Instances **/

func (s *SharedLoadBalanceClient) CreateInstance(req *model.CreateLoadbalancerReq) (*model.LoadbalancerResp, error) {
	var rsp *model.LoadbalancerResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.CreateLoadbalancer(&model.CreateLoadbalancerRequest{
			Body: &model.CreateLoadbalancerRequestBody{
				Loadbalancer: req,
			},
		})
	}, "Loadbalancer", &rsp)
	return rsp, err
}

func (s *SharedLoadBalanceClient) CreateInstanceCompleted(req *model.CreateLoadbalancerReq) (*model.LoadbalancerResp, error) {
	instance, err := s.CreateInstance(req)
	if err != nil {
		return nil, err
	}
	return s.WaitStatusActive(instance.Id)
}

func (s *SharedLoadBalanceClient) WaitStatusActive(id string) (*model.LoadbalancerResp, error) {
	var instance *model.LoadbalancerResp

	err := common.WaitForCompleted(func() (bool, error) {
		ins, err := s.GetInstance(id)
		instance = ins
		if err != nil {
			return false, err
		}

		statusEnum := model.GetLoadbalancerRespProvisioningStatusEnum()
		if instance.ProvisioningStatus == statusEnum.ACTIVE {
			return true, nil
		}

		if instance.ProvisioningStatus == statusEnum.ERROR {
			return false, status.Error(codes.Unavailable, "LoadBalancer has gone into ERROR provisioning status")
		}

		return false, nil
	})

	return instance, err
}

func (s *SharedLoadBalanceClient) GetInstance(id string) (*model.LoadbalancerResp, error) {
	var rsp *model.LoadbalancerResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ShowLoadbalancer(&model.ShowLoadbalancerRequest{
			LoadbalancerId: id,
		})
	}, "Loadbalancer", &rsp)

	return rsp, err
}

func (s *SharedLoadBalanceClient) ListInstances(req *model.ListLoadbalancersRequest) ([]model.LoadbalancerResp, error) {
	var rst []model.LoadbalancerResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ListLoadbalancers(req)
	}, "Loadbalancers", &rst)
	return rst, err
}

func (s *SharedLoadBalanceClient) UpdateInstance(id, name, description string) (*model.LoadbalancerResp, error) {
	var rst *model.LoadbalancerResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.UpdateLoadbalancer(&model.UpdateLoadbalancerRequest{
			LoadbalancerId: id,
			Body: &model.UpdateLoadbalancerRequestBody{
				Loadbalancer: &model.UpdateLoadbalancerReq{
					Name:        &name,
					Description: &description,
				},
			},
		})
	}, "Loadbalancer", &rst)
	return rst, err
}

func (s *SharedLoadBalanceClient) DeleteInstance(id string) error {
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		req := model.DeleteLoadbalancerRequest{
			LoadbalancerId: id,
		}
		klog.Infof("Delete Req: %#v", req)
		return c.DeleteLoadbalancer(&req)
	})
}

/** Listeners **/

func (s *SharedLoadBalanceClient) CreateListener(req *model.CreateListenerReq) (*model.ListenerResp, error) {
	var rst *model.ListenerResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.CreateListener(&model.CreateListenerRequest{
			Body: &model.CreateListenerRequestBody{
				Listener: req,
			},
		})
	}, "Listener", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) GetListener(id string) (*model.ListenerResp, error) {
	var rst *model.ListenerResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ShowListener(&model.ShowListenerRequest{ListenerId: id})
	}, "Listener", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) ListListeners(req *model.ListListenersRequest) ([]model.ListenerResp, error) {
	//rst := make([]model.ListenerResp, 0)
	var rst []model.ListenerResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ListListeners(req)
	}, "Listeners", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) UpdateListener(id string, req *model.UpdateListenerReq) error {
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.UpdateListener(&model.UpdateListenerRequest{
			ListenerId: id,
			Body: &model.UpdateListenerRequestBody{
				Listener: req,
			},
		})
	})
}

func (s *SharedLoadBalanceClient) DeleteListener(elbID string, listenerID string) error {
	// Check pools bound to this listener
	pools, err := s.ListPools(&model.ListPoolsRequest{LoadbalancerId: pointer.String(elbID)})
	if err != nil {
		return err
	}
	count := 0
	for _, p := range pools {
		for _, l := range p.Listeners {
			if l.Id == listenerID {
				count++
				break
			}
		}
	}
	if count > 0 {
		return fmt.Errorf("delete failed, %d pools are using the listener: %s", count, listenerID)
	}

	err = s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.DeleteListener(&model.DeleteListenerRequest{
			ListenerId: listenerID,
		})
	})

	if err != nil && common.IsNotFound(err) {
		_, err = s.GetListener(listenerID)
		if err != nil && common.IsNotFound(err) {
			return nil
		}
	}

	return err
}

/** Pools **/

func (s *SharedLoadBalanceClient) CreatePool(req *model.CreatePoolReq) (*model.PoolResp, error) {
	var rst *model.PoolResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.CreatePool(&model.CreatePoolRequest{
			Body: &model.CreatePoolRequestBody{
				Pool: req,
			},
		})
	}, "Pool", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) GetPool(id string) (*model.PoolResp, error) {
	var rst *model.PoolResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ShowPool(&model.ShowPoolRequest{
			PoolId: id,
		})
	}, "Pool", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) ListPools(req *model.ListPoolsRequest) ([]model.PoolResp, error) {
	var rst []model.PoolResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ListPools(req)
	}, "Pools", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) UpdatePool(id string, req *model.UpdatePoolReq) (*model.PoolResp, error) {
	var rst *model.PoolResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.UpdatePool(&model.UpdatePoolRequest{
			PoolId: id,
			Body: &model.UpdatePoolRequestBody{
				Pool: req,
			},
		})
	}, "", &rst)
	return rst, err
}

func (s *SharedLoadBalanceClient) DeletePool(id string) error {
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.DeletePool(&model.DeletePoolRequest{
			PoolId: id,
		})
	})
}

/** Health Monitor **/

func (s *SharedLoadBalanceClient) CreateHealthMonitor(req *model.CreateHealthmonitorReq) (*model.HealthmonitorResp, error) {
	var rst *model.HealthmonitorResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.CreateHealthmonitor(&model.CreateHealthmonitorRequest{
			Body: &model.CreateHealthmonitorRequestBody{
				Healthmonitor: req,
			},
		})
	}, "Healthmonitor", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) GetHealthMonitor(id string) (*model.HealthmonitorResp, error) {
	var rst *model.HealthmonitorResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ShowHealthmonitors(&model.ShowHealthmonitorsRequest{HealthmonitorId: id})
	}, "Healthmonitor", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) UpdateHealthMonitor(id string, req *model.UpdateHealthmonitorReq) error {
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.UpdateHealthmonitor(&model.UpdateHealthmonitorRequest{
			HealthmonitorId: id,
			Body: &model.UpdateHealthmonitorRequestBody{
				Healthmonitor: req,
			},
		})
	})
}

func (s *SharedLoadBalanceClient) DeleteHealthMonitor(id string) error {
	if id == "" {
		return nil
	}
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.DeleteHealthmonitor(&model.DeleteHealthmonitorRequest{HealthmonitorId: id})
	})
}

/** Member **/

func (s *SharedLoadBalanceClient) AddMember(poolID string, req *model.CreateMemberReq) (*model.MemberResp, error) {
	var rst *model.MemberResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.CreateMember(&model.CreateMemberRequest{
			PoolId: poolID,
			Body: &model.CreateMemberRequestBody{
				Member: req,
			},
		})
	}, "Member", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) GetMember(id string) (*model.MemberResp, error) {
	var rst *model.MemberResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ShowMember(&model.ShowMemberRequest{
			MemberId: id,
		})
	}, "Member", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) ListMembers(req *model.ListMembersRequest) ([]model.MemberResp, error) {
	var rst []model.MemberResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ListMembers(req)
	}, "Members", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) UpdateMember(id string, req *model.UpdateMemberReq) (*model.MemberResp, error) {
	var rst *model.MemberResp
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.UpdateMember(&model.UpdateMemberRequest{
			MemberId: id,
			Body: &model.UpdateMemberRequestBody{
				Member: req,
			},
		})
	}, "Member", &rst)

	return rst, err
}

func (s *SharedLoadBalanceClient) DeleteMember(poolID, memberID string) error {
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.DeleteMember(&model.DeleteMemberRequest{
			PoolId:   poolID,
			MemberId: memberID,
		})
	})
}

func (s *SharedLoadBalanceClient) DeleteAllPoolMembers(poolID string) error {
	members, err := s.ListMembers(&model.ListMembersRequest{PoolId: poolID})
	if err != nil {
		return nil
	}

	errs := make([]error, 0)
	for _, m := range members {
		if err := s.DeleteMember(poolID, m.Id); err != nil && !common.IsNotFound(err) {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return status.Errorf(codes.Internal, "failed to clean pool members: %s", errors.NewAggregate(errs))
	}
	return nil
}

func (s *SharedLoadBalanceClient) wrapper(handler func(*elb.ElbClient) (interface{}, error), args ...interface{}) error {
	return commonWrapper(func() (interface{}, error) {
		hc := s.AuthOpts.GetHcClient("elb")
		return handler(elb.NewElbClient(hc))
	}, OKCodes, args...)
}
