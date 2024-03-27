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
	"net"
	"reflect"
	"sort"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"

	wpmodel "sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/model"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/utils"
)

var OKCodes = []int{200, 201, 204}

type EcsClient struct {
	AuthOpts *config.AuthOptions
}

func (e *EcsClient) Get(id string) (*model.ServerDetail, error) {
	var rst *model.ServerDetail
	err := e.wrapper(func(c *ecs.EcsClient) (interface{}, error) {
		return c.ShowServer(&model.ShowServerRequest{ServerId: id})
	}, "Server", &rst)
	return rst, err
}

func (e *EcsClient) GetByNodeName(name string) (*model.ServerDetail, error) {
	privateIP := ""
	if net.ParseIP(name).To4() != nil {
		privateIP = name
	} else if ips := utils.LookupHost(name); len(ips) > 0 {
		for _, ip := range ips {
			if ip != "127.0.0.1" {
				privateIP = ip
				break
			}
		}
	}

	if privateIP == "" {
		klog.V(6).Infof("query ECS detail by name: %s", name)
		return e.GetByName(name)
	}

	klog.V(6).Infof("query ECS detail by private IP: %s, NodeName: %s", privateIP, name)

	rsp, err := e.List(&model.ListServersDetailsRequest{
		IpEq: &privateIP,
	})
	if err != nil {
		return nil, err
	}

	notFound := fmt.Errorf("not found any ECS, node: %s, PrivateIP: %s", name, privateIP)
	if rsp.Servers == nil || len(*rsp.Servers) == 0 {
		return nil, notFound
	}

	for _, sv := range *rsp.Servers {
		for _, addresses := range sv.Addresses {
			for _, addr := range addresses {
				if addr.Addr == privateIP {
					return &sv, nil
				}
			}
		}
	}

	return nil, notFound
}

func (e *EcsClient) GetByNodeIP(privateIP string) (*model.ServerDetail, error) {
	if privateIP == "" {
		return nil, fmt.Errorf("privateIP can be empty")
	}

	rsp, err := e.List(&model.ListServersDetailsRequest{
		IpEq: &privateIP,
	})
	if err != nil {
		return nil, err
	}

	notFound := fmt.Errorf("not found any ECS, PrivateIP: %s", privateIP)
	if rsp.Servers == nil || len(*rsp.Servers) == 0 {
		return nil, notFound
	}

	for _, sv := range *rsp.Servers {
		for _, addresses := range sv.Addresses {
			for _, addr := range addresses {
				if addr.Addr == privateIP {
					return &sv, nil
				}
			}
		}
	}

	return nil, notFound
}

func (e *EcsClient) GetByNodeIPNew(privateIP string) (*wpmodel.ServerDetail, error) {
	if privateIP == "" {
		return nil, fmt.Errorf("privateIP can be empty")
	}

	var rsp *wpmodel.ListServersDetailsResponse
	err := e.wrapper(func(c *ecs.EcsClient) (interface{}, error) {
		requestDef := wpmodel.GenReqDefForListServersDetails()
		resp, err := c.HcClient.Sync(&model.ListServersDetailsRequest{
			IpEq: &privateIP,
		}, requestDef)

		if err != nil {
			return nil, err
		}
		return resp.(*wpmodel.ListServersDetailsResponse), nil
	}, &rsp)
	if err != nil {
		return nil, err
	}

	notFound := fmt.Errorf("not found any ECS, PrivateIP: %s", privateIP)
	if rsp.Servers == nil || len(*rsp.Servers) == 0 {
		return nil, notFound
	}

	for _, sv := range *rsp.Servers {
		for _, addresses := range sv.Addresses {
			for _, addr := range addresses {
				if addr.Addr == privateIP {
					return &sv, nil
				}
			}
		}
	}

	return nil, notFound
}

func (e *EcsClient) GetByName(name string) (*model.ServerDetail, error) {
	name = fmt.Sprintf("^%s$", name)

	rsp, err := e.List(&model.ListServersDetailsRequest{Name: &name})
	if err != nil {
		return nil, err
	}
	serverList := *rsp.Servers
	if len(serverList) == 0 {
		return nil, status.Errorf(codes.NotFound, "Error, not found any servers matched name: %s", name)
	}

	return &serverList[0], nil
}

func (e *EcsClient) List(req *model.ListServersDetailsRequest) (*model.ListServersDetailsResponse, error) {
	var rst *model.ListServersDetailsResponse
	err := e.wrapper(func(c *ecs.EcsClient) (interface{}, error) {
		return c.ListServersDetails(req)
	}, &rst)
	return rst, err
}

func (e *EcsClient) ListInterfaces(req *model.ListServerInterfacesRequest) ([]model.InterfaceAttachment, error) {
	var rst []model.InterfaceAttachment
	err := e.wrapper(func(c *ecs.EcsClient) (interface{}, error) {
		return c.ListServerInterfaces(req)
	}, "InterfaceAttachments", &rst)
	return rst, err
}

func (e *EcsClient) BuildAddresses(server *model.ServerDetail, interfaces []model.InterfaceAttachment,
	networkingOpts *config.NetworkingOptions) ([]v1.NodeAddress, error) {
	nodeAddresses := make([]v1.NodeAddress, 0)

	// parse private IP addresses first in an ordered manner
	for _, inter := range interfaces {
		if *inter.PortState == "ACTIVE" {
			for _, fixedIP := range *inter.FixedIps {
				if net.ParseIP(*fixedIP.IpAddress).To4() != nil {
					addToNodeAddresses(&nodeAddresses,
						v1.NodeAddress{
							Type:    v1.NodeInternalIP,
							Address: *fixedIP.IpAddress,
						},
					)
				}
			}
		}
	}

	// process public IP addresses
	if server.AccessIPv4 != "" {
		addToNodeAddresses(&nodeAddresses,
			v1.NodeAddress{
				Type:    v1.NodeExternalIP,
				Address: server.AccessIPv4,
			},
		)
	}

	nicIDs := make([]string, 0)
	for nicID := range server.Addresses {
		nicIDs = append(nicIDs, nicID)
	}
	sort.Strings(nicIDs)

	for _, nicID := range nicIDs {
		for _, serverAddr := range server.Addresses[nicID] {
			var addressType v1.NodeAddressType
			if serverAddr.OSEXTIPStype != nil && serverAddr.OSEXTIPStype.Value() == "floating" {
				addressType = v1.NodeExternalIP
			} else if utils.IsStrSliceContains(networkingOpts.PublicNetworkName, nicID) {
				addressType = v1.NodeExternalIP
				// removing already added address to avoid listing it as both ExternalIP and InternalIP
				// may happen due to listing "private" network as "public" in CCM's CloudConfig
				removeFromNodeAddresses(&nodeAddresses,
					v1.NodeAddress{
						Address: serverAddr.Addr,
					},
				)
			} else {
				if len(networkingOpts.InternalNetworkName) == 0 ||
					utils.IsStrSliceContains(networkingOpts.InternalNetworkName, nicID) {
					addressType = v1.NodeInternalIP
				} else {
					klog.V(4).Infof("[DEBUG] Node '%s' address '%s' ignored due to 'internal-network-name' option",
						server.Name, serverAddr.Addr)

					removeFromNodeAddresses(&nodeAddresses,
						v1.NodeAddress{
							Address: serverAddr.Addr,
						},
					)
					continue
				}
			}

			if net.ParseIP(serverAddr.Addr).To4() != nil {
				addToNodeAddresses(&nodeAddresses,
					v1.NodeAddress{
						Type:    addressType,
						Address: serverAddr.Addr,
					},
				)
			}
		}
	}
	klog.V(6).Infof("server: %s/%s, network addresses: %s", server.Name, server.Id, utils.ToString(nodeAddresses))
	return nodeAddresses, nil
}

func (e *EcsClient) ListSecurityGroups(instanceID string) ([]model.NovaSecurityGroup, error) {
	var rst []model.NovaSecurityGroup
	err := e.wrapper(func(c *ecs.EcsClient) (interface{}, error) {
		return c.NovaListServerSecurityGroups(&model.NovaListServerSecurityGroupsRequest{
			ServerId: instanceID,
		})
	}, "SecurityGroups", &rst)
	return rst, err
}

func (e *EcsClient) AssociateSecurityGroup(instanceID, securityGroupID string) error {
	return e.wrapper(func(c *ecs.EcsClient) (interface{}, error) {
		return c.NovaAssociateSecurityGroup(&model.NovaAssociateSecurityGroupRequest{
			ServerId: instanceID,
			Body: &model.NovaAssociateSecurityGroupRequestBody{
				AddSecurityGroup: &model.NovaAddSecurityGroupOption{
					Name: securityGroupID,
				},
			},
		})
	})
}

func (e *EcsClient) DisassociateSecurityGroup(instanceID, securityGroupID string) error {
	err := e.wrapper(func(c *ecs.EcsClient) (interface{}, error) {
		return c.NovaDisassociateSecurityGroup(&model.NovaDisassociateSecurityGroupRequest{
			ServerId: instanceID,
			Body: &model.NovaDisassociateSecurityGroupRequestBody{
				RemoveSecurityGroup: &model.NovaRemoveSecurityGroupOption{
					Name: securityGroupID,
				},
			},
		})
	})

	if err != nil {
		notAssociated := "not associated with the instance"
		notFound := "is not found for project"
		if strings.Contains(err.Error(), notAssociated) || strings.Contains(err.Error(), notFound) {
			klog.Errorf("failed to disassociate security group %v from instance %v: %v",
				securityGroupID, instanceID, err)
			return nil
		}
	}

	return err
}

// addToNodeAddresses appends the NodeAddresses to the passed-by-pointer slice, only if they do not already exist.
func addToNodeAddresses(addresses *[]v1.NodeAddress, addAddresses ...v1.NodeAddress) {
	for _, add := range addAddresses {
		exists := false
		for _, existing := range *addresses {
			if existing.Address == add.Address && existing.Type == add.Type {
				exists = true
				break
			}
		}
		if !exists {
			*addresses = append(*addresses, add)
		}
	}
}

// removeFromNodeAddresses removes the NodeAddresses from the passed-by-pointer slice if they already exist.
func removeFromNodeAddresses(addresses *[]v1.NodeAddress, removeAddresses ...v1.NodeAddress) {
	var indexesToRemove []int
	for _, remove := range removeAddresses {
		for i := len(*addresses) - 1; i >= 0; i-- {
			existing := (*addresses)[i]
			if existing.Address == remove.Address && (existing.Type == remove.Type || remove.Type == "") {
				indexesToRemove = append(indexesToRemove, i)
			}
		}
	}
	for _, i := range indexesToRemove {
		if i < len(*addresses) {
			*addresses = append((*addresses)[:i], (*addresses)[i+1:]...)
		}
	}
}

func (e *EcsClient) wrapper(handler func(*ecs.EcsClient) (interface{}, error), args ...interface{}) error {
	return commonWrapper(func() (interface{}, error) {
		hc := e.AuthOpts.GetHcClient("ecs")
		return handler(ecs.NewEcsClient(hc))
	}, OKCodes, args...)
}

// commonWrapper wrapper common steps.
// args[0]: string, keys
// args[1]: interface, result
func commonWrapper(handler func() (interface{}, error), okCodes []int, args ...interface{}) error {
	response, err := handler()
	if err != nil {
		klog.ErrorDepth(2, fmt.Sprintf("Error in wrapper handler(), args: %#v, error: %s", args, err))
		return err
	}
	if err = checkStatusCode(response, okCodes); err != nil {
		return err
	}

	// Check if need to set the return
	if len(args) == 0 {
		return nil
	}
	// Check return parameters
	if len(args) > 2 {
		klog.ErrorDepth(2, fmt.Sprintf("`args` length is wrong, expected 2, got: %d, args: %#v", len(args), args))
		return fmt.Errorf("`args` length is wrong, expected 2, got: %d, args: %#v", len(args), args)
	}

	key, ok := args[0].(string)
	if !ok {
		refVal := reflect.ValueOf(response)
		return setResultValue(args[0], &refVal)
	}

	result := args[1]
	if len(key) == 0 || strings.TrimSpace(key) == "" {
		return nil
	}
	// Get and set value to result
	refVal, err := utils.GetStructField(response, key)
	if err != nil {
		return err
	}
	return setResultValue(result, &refVal)
}

func setResultValue(result interface{}, val *reflect.Value) error {
	refVal := *val
	rstVal := reflect.ValueOf(result)
	if rstVal.Kind() != reflect.Pointer {
		klog.ErrorDepth(3, "`result` must be a pointer type, otherwise the data cannot be set")
		return fmt.Errorf("`result` must be a pointer type, otherwise the data cannot be set")
	}

	rstVal = rstVal.Elem()
	if refVal.Kind() == reflect.Pointer && refVal.Elem().Kind() == reflect.Slice {
		refVal = refVal.Elem()
	}
	if !rstVal.CanConvert(refVal.Type()) {
		klog.ErrorDepth(3, fmt.Sprintf("error, cannot convert %s to %s", refVal.Type(), rstVal.Type()))
		return fmt.Errorf("error, cannot convert %s to %s", refVal.Type(), rstVal.Type())
	}
	rstVal.Set(refVal)

	return nil
}

func checkStatusCode(response interface{}, okCodes []int) error {
	value, err := utils.GetStructField(response, "HttpStatusCode")
	if err != nil {
		return err
	}
	statusCode := int(value.Int())

	for _, code := range okCodes {
		if code == int(value.Int()) {
			return nil
		}
	}

	return sdkerr.ServiceResponseError{
		StatusCode:   int(value.Int()),
		ErrorMessage: fmt.Sprintf("error: unexpected StatusCode: %d, response: %#v", statusCode, response),
	}
}
