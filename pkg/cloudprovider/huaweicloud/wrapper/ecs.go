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

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

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
	addrs := []v1.NodeAddress{}

	// parse private IP addresses first in an ordered manner
	for _, iface := range interfaces {
		for _, fixedIP := range *iface.FixedIps {
			if *iface.PortState == "ACTIVE" {
				if net.ParseIP(*fixedIP.IpAddress).To4() != nil {
					addToNodeAddresses(&addrs,
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
		addToNodeAddresses(&addrs,
			v1.NodeAddress{
				Type:    v1.NodeExternalIP,
				Address: server.AccessIPv4,
			},
		)
	}

	// process the rest
	type Address struct {
		IPType string `mapstructure:"OS-EXT-IPS:type"`
		Addr   string
	}

	var addresses map[string][]Address
	err := mapstructure.Decode(server.Addresses, &addresses)
	if err != nil {
		return nil, err
	}

	var networks []string
	for k := range addresses {
		networks = append(networks, k)
	}
	sort.Strings(networks)

	for _, network := range networks {
		for _, props := range addresses[network] {
			var addressType v1.NodeAddressType
			if props.IPType == "floating" {
				addressType = v1.NodeExternalIP
			} else if utils.IsStrSliceContains(networkingOpts.PublicNetworkName, network) {
				addressType = v1.NodeExternalIP
				// removing already added address to avoid listing it as both ExternalIP and InternalIP
				// may happen due to listing "private" network as "public" in CCM's CloudConfig
				removeFromNodeAddresses(&addrs,
					v1.NodeAddress{
						Address: props.Addr,
					},
				)
			} else {
				if len(networkingOpts.InternalNetworkName) == 0 ||
					utils.IsStrSliceContains(networkingOpts.InternalNetworkName, network) {
					addressType = v1.NodeInternalIP
				} else {
					klog.V(4).Infof("[DEBUG] Node '%s' address '%s' "+
						"ignored due to 'internal-network-name' option", server.Name, props.Addr)

					removeFromNodeAddresses(&addrs,
						v1.NodeAddress{
							Address: props.Addr,
						},
					)
					continue
				}
			}

			if net.ParseIP(props.Addr).To4() != nil {
				addToNodeAddresses(&addrs,
					v1.NodeAddress{
						Type:    addressType,
						Address: props.Addr,
					},
				)
			}
		}
	}

	return addrs, nil
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
