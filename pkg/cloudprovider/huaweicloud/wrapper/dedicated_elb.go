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
	"fmt"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
	elb "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v3"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v3/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/common"
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

/** ELB Instances **/

func (s *DedicatedLoadBalanceClient) CreateInstance(opt *model.CreateLoadBalancerOption) (*model.LoadBalancer, error) {
	var rsp *model.LoadBalancer
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.CreateLoadBalancer(&model.CreateLoadBalancerRequest{
			Body: &model.CreateLoadBalancerRequestBody{
				Loadbalancer: opt,
			},
		})
	}, "Loadbalancer", &rsp)
	return rsp, err
}

func (s *DedicatedLoadBalanceClient) CreateInstanceCompleted(req *model.CreateLoadBalancerOption) (*model.LoadBalancer, error) {
	instance, err := s.CreateInstance(req)
	if err != nil {
		return nil, err
	}
	return s.WaitStatusActive(instance.Id)
}

func (s *DedicatedLoadBalanceClient) WaitStatusActive(id string) (*model.LoadBalancer, error) {
	var instance *model.LoadBalancer

	err := common.WaitForCompleted(func() (bool, error) {
		ins, err := s.GetInstance(id)
		if err != nil {
			return false, err
		}
		instance = ins

		if instance.ProvisioningStatus == "ACTIVE" {
			return true, nil
		}

		if instance.ProvisioningStatus == "ERROR" {
			return false, status.Error(codes.Unavailable, "LoadBalancer has gone into ERROR provisioning status")
		}

		return false, nil
	})

	return instance, err
}

func (s *DedicatedLoadBalanceClient) GetInstance(id string) (*model.LoadBalancer, error) {
	var rsp *model.LoadBalancer
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ShowLoadBalancer(&model.ShowLoadBalancerRequest{
			LoadbalancerId: id,
		})
	}, "Loadbalancer", &rsp)

	return rsp, err
}

func (s *DedicatedLoadBalanceClient) ListInstances(req *model.ListLoadBalancersRequest) ([]model.LoadBalancer, error) {
	var rst []model.LoadBalancer
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ListLoadBalancers(req)
	}, "Loadbalancers", &rst)
	return rst, err
}

func (s *DedicatedLoadBalanceClient) UpdateInstance(id, name, description string) (*model.LoadBalancer, error) {
	var rst *model.LoadBalancer
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.UpdateLoadBalancer(&model.UpdateLoadBalancerRequest{
			LoadbalancerId: id,
			Body: &model.UpdateLoadBalancerRequestBody{
				Loadbalancer: &model.UpdateLoadBalancerOption{
					Name:        &name,
					Description: &description,
				},
			},
		})
	}, "Loadbalancer", &rst)
	return rst, err
}

func (s *DedicatedLoadBalanceClient) DeleteInstance(id string) error {
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		req := model.DeleteLoadBalancerRequest{
			LoadbalancerId: id,
		}
		klog.Infof("Delete Req: %#v", req)
		return c.DeleteLoadBalancer(&req)
	})
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

func (s *DedicatedLoadBalanceClient) GetListener(id string) (*model.Listener, error) {
	var rst *model.Listener
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ShowListener(&model.ShowListenerRequest{ListenerId: id})
	}, "Listener", &rst)

	return rst, err
}

func (s *DedicatedLoadBalanceClient) ListListeners(req *model.ListListenersRequest) ([]model.Listener, error) {
	var rst []model.Listener
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ListListeners(req)
	}, "Listeners", &rst)

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

func (s *DedicatedLoadBalanceClient) DeleteListener(elbID string, listenerID string) error {
	// Check pools bound to this listener
	ids := []string{elbID}
	lisIDs := []string{listenerID}
	pools, err := s.ListPools(&model.ListPoolsRequest{
		LoadbalancerId: &ids,
		ListenerId:     &lisIDs,
	})
	if err != nil {
		return err
	}
	if len(pools) > 0 {
		return fmt.Errorf("delete failed, %d pools are using the listener: %s", len(pools), listenerID)
	}

	err = s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.DeleteListener(&model.DeleteListenerRequest{
			ListenerId: listenerID,
		})
	})

	if err != nil && common.IsNotFound(err) {
		return nil
	}

	return err
}

/** Pools **/

func (s *DedicatedLoadBalanceClient) CreatePool(req *model.CreatePoolOption) (*model.Pool, error) {
	var rst *model.Pool
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.CreatePool(&model.CreatePoolRequest{
			Body: &model.CreatePoolRequestBody{
				Pool: req,
			},
		})
	}, "Pool", &rst)

	return rst, err
}

func (s *DedicatedLoadBalanceClient) GetPool(id string) (*model.Pool, error) {
	var rst *model.Pool
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ShowPool(&model.ShowPoolRequest{
			PoolId: id,
		})
	}, "Pool", &rst)

	return rst, err
}

func (s *DedicatedLoadBalanceClient) ListPools(req *model.ListPoolsRequest) ([]model.Pool, error) {
	var rst []model.Pool
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ListPools(req)
	}, "Pools", &rst)

	return rst, err
}

func (s *DedicatedLoadBalanceClient) UpdatePool(id string, req *model.UpdatePoolOption) (*model.Pool, error) {
	var rst *model.Pool
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.UpdatePool(&model.UpdatePoolRequest{
			PoolId: id,
			Body: &model.UpdatePoolRequestBody{
				Pool: req,
			},
		})
	}, "Pool", &rst)
	return rst, err
}

func (s *DedicatedLoadBalanceClient) DeletePool(id string) error {
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.DeletePool(&model.DeletePoolRequest{
			PoolId: id,
		})
	})
}

/** Health Monitor **/

func (s *DedicatedLoadBalanceClient) CreateHealthMonitor(req *model.CreateHealthMonitorOption) (*model.HealthMonitor, error) {
	var rst *model.HealthMonitor
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.CreateHealthMonitor(&model.CreateHealthMonitorRequest{
			Body: &model.CreateHealthMonitorRequestBody{
				Healthmonitor: req,
			},
		})
	}, "Healthmonitor", &rst)

	return rst, err
}

func (s *DedicatedLoadBalanceClient) GetHealthMonitor(id string) (*model.HealthMonitor, error) {
	var rst *model.HealthMonitor
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ShowHealthMonitor(&model.ShowHealthMonitorRequest{HealthmonitorId: id})
	}, "Healthmonitor", &rst)

	return rst, err
}

func (s *DedicatedLoadBalanceClient) UpdateHealthMonitor(id string, req *model.UpdateHealthMonitorOption) error {
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.UpdateHealthMonitor(&model.UpdateHealthMonitorRequest{
			HealthmonitorId: id,
			Body: &model.UpdateHealthMonitorRequestBody{
				Healthmonitor: req,
			},
		})
	})
}

func (s *DedicatedLoadBalanceClient) DeleteHealthMonitor(id string) error {
	if id == "" {
		return nil
	}
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.DeleteHealthMonitor(&model.DeleteHealthMonitorRequest{HealthmonitorId: id})
	})
}

/** Member **/

func (s *DedicatedLoadBalanceClient) AddMember(poolID string, req *model.CreateMemberOption) (*model.Member, error) {
	var rst *model.Member
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.CreateMember(&model.CreateMemberRequest{
			PoolId: poolID,
			Body: &model.CreateMemberRequestBody{
				Member: req,
			},
		})
	}, "Member", &rst)

	// Ignore existing members
	if err != nil {
		if ne, ok := err.(sdkerr.ServiceResponseError); ok && ne.ErrorCode == "409" {
			return rst, nil
		}
	}
	return rst, err
}

func (s *DedicatedLoadBalanceClient) GetMember(id string) (*model.Member, error) {
	var rst *model.Member
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ShowMember(&model.ShowMemberRequest{
			MemberId: id,
		})
	}, "Member", &rst)

	return rst, err
}

func (s *DedicatedLoadBalanceClient) ListMembers(req *model.ListMembersRequest) ([]model.Member, error) {
	var rst []model.Member
	err := s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.ListMembers(req)
	}, "Members", &rst)

	return rst, err
}

func (s *DedicatedLoadBalanceClient) UpdateMember(id string, req *model.UpdateMemberOption) (*model.Member, error) {
	var rst *model.Member
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

func (s *DedicatedLoadBalanceClient) DeleteMember(poolID, memberID string) error {
	return s.wrapper(func(c *elb.ElbClient) (interface{}, error) {
		return c.DeleteMember(&model.DeleteMemberRequest{
			PoolId:   poolID,
			MemberId: memberID,
		})
	})
}

func (s *DedicatedLoadBalanceClient) DeleteAllPoolMembers(poolID string) error {
	members, err := s.ListMembers(&model.ListMembersRequest{PoolId: poolID})
	if err != nil {
		return nil
	}

	errs := make([]error, 0)
	for _, m := range members {
		if err := s.DeleteMember(poolID, m.Id); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return status.Errorf(codes.Internal, "failed to clean pool members: %s", errors.NewAggregate(errs))
	}
	return nil
}

func (s *DedicatedLoadBalanceClient) wrapper(handler func(*elb.ElbClient) (interface{}, error), args ...interface{}) error {
	return commonWrapper(func() (interface{}, error) {
		hc := s.AuthOpts.GetHcClient("elb")
		return handler(elb.NewElbClient(hc))
	}, OKCodes, args...)
}
