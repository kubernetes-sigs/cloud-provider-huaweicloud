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

// nolint:golint // stop check lint issues as this file will be refactored
package huaweicloud

import (
	"fmt"
	"io"
	"net/http"
)

type NATProtocol string

const (
	NATProtocolTCP NATProtocol = "TCP"
	NATProtocolUDP NATProtocol = "UDP"
)

type NATStatus string

const (
	NATStatusActive        NATStatus = "ACTIVE"
	NATStatusError         NATStatus = "ERROR"
	NATStatusPendingCreate NATStatus = "PENDING_CREATE"
	NATStatusPendingUpdate NATStatus = "PENDING_UPDATE"
	NATStatusPendingDelete NATStatus = "PENDING_DELETE"
)

type NATSpec string

const (
	NATSpecSmall      NATSpec = "1"
	NATSpecMedium     NATSpec = "2"
	NATSpecLarge      NATSpec = "3"
	NATSpecExtraLarge NATSpec = "4"
)

type DNATRuleStatus string

const (
	DNATRuleStatusActive        DNATRuleStatus = "ACTIVE"
	DNATRuleStatusError         DNATRuleStatus = "ERROR"
	DNATRuleStatusPendingCreate DNATRuleStatus = "PENDING_CREATE"
	DNATRuleStatusPendingUpdate DNATRuleStatus = "PENDING_UPDATE"
	DNATRuleStatusPendingDelete DNATRuleStatus = "PENDING_DELETE"
)

type PortStatus string

const (
	PortStatusActive PortStatus = "ACTIVE"
	PortStatusBuild  PortStatus = "BUILD"
	PortStatusDown   PortStatus = "DOWN"
)

type FloatingIpStatus string

const (
	FloatingIpStatusActive FloatingIpStatus = "ACTIVE"
	FloatingIpStatusError  FloatingIpStatus = "ERROR"
	FloatingIpStatusDown   FloatingIpStatus = "DOWN"
)

// NAT Gateway
type NATGateway struct {
	Id                string    `json:"id,omitempty"`
	Name              string    `json:"name,omitempty"`
	Description       string    `json:"description,omitempty"`
	RouterId          string    `json:"router_id,omitempty"`
	InternalNetWorkId string    `json:"internal_network_id,omitempty"`
	Status            NATStatus `json:"status,omitempty"`
	Spec              NATSpec   `json:"spec,omitempty"`
	TenantId          string    `json:"tenant_id,omitempty"`
	AdminStateUp      bool      `json:"admin_state_up,omitempty"`
}

type NATArr struct {
	NATGateway NATGateway `json:"nat_gateway"`
}

// list type
type NATGatewayList struct {
	NATGateways []NATGateway `json:"nat_gateways"`
}

type DNATRuleDescription struct {
	ClusterID   string `json:"cluster_id,omitempty"`
	Description string `json:"description,omitempty"`
}

// DNA Rule
type DNATRule struct {
	Id                  string         `json:"id,omitempty"`
	TenantId            string         `json:"tenant_id,omitempty"`
	NATGatewayId        string         `json:"nat_gateway_id,omitempty"`
	PortId              string         `json:"port_id,omitempty"`
	InternalServicePort int32          `json:"internal_service_port,omitempty"`
	FloatingIpId        string         `json:"floating_ip_id,omitempty"`
	ExternalServicePort int32          `json:"external_service_port,omitempty"`
	FloatingIpAddress   string         `json:"floating_ip_address,omitempty"`
	Protocol            NATProtocol    `json:"protocol,omitempty"`
	Status              DNATRuleStatus `json:"status,omitempty"`
	AdminStateUp        bool           `json:"admin_state_up,omitempty"`
	Description         string         `json:"description,omitempty"`
}

type DNATRuleArr struct {
	DNATRule DNATRule `json:"dnat_rule"`
}

// list type
type DNATRuleList struct {
	DNATRules []DNATRule `json:"dnat_rules"`
}

type Port struct {
	Id                  string              `json:"id,omitempty"`
	Name                string              `json:"name,omitempty"`
	NetworkId           string              `json:"network_id,omitempty"`
	AdminStateUp        bool                `json:"admin_state_up,omitempty"`
	MacAddress          string              `json:"mac_address,omitempty"`
	FixedIps            []*FixedIp          `json:"fixed_ips,omitempty"`
	DeviceId            string              `json:"device_id,omitempty"`
	DeviceOwner         string              `json:"device_owner,omitempty"`
	TenantId            string              `json:"tenant_id,omitempty"`
	Status              PortStatus          `json:"status,omitempty"`
	SecurityGroups      []string            `json:"security_groups,omitempty"`
	AllowedAddressPairs []*AllowAddressPair `json:"allow_address_pairs,omitempty"`
}

type PortArr struct {
	Port Port `json:"port"`
}

type PortList struct {
	Ports []Port `json:"ports,omitempty"`
}

type FixedIp struct {
	SubnetId  string `json:"subnet_id,omitempty"`
	IpAddress string `json:"ip_address"`
}

type AllowAddressPair struct {
	IpAddress  string `json:"ip_address,omitempty"`
	MacAddress string `json:"mac_address,omitempty"`
	OptName    string `json:"opt_name,omitempty"`
	OptValue   string `json:"opt_value,omitempty"`
}

type FloatingIp struct {
	Id                string           `json:"id,omitempty"`
	Status            FloatingIpStatus `json:"status,omitempty"`
	FloatingIpAddress string           `json:"floating_ip_address,omitempty"`
	FloatingNetworkId string           `json:"floating_network_id,omitempty"`
	RouterId          string           `json:"router_id,omitempty"`
	PortId            string           `json:"port_id,omitempty"`
	FixedIpAddress    string           `json:"fixed_ip_address,omitempty"`
	TenantId          string           `json:"tenant_id,omitempty"`
}

type FloatingIpArr struct {
	FloatingIp FloatingIp `json:"floatingip"`
}

type FloatingIpList struct {
	FloatingIps []FloatingIp `json:"floatingips,omitempty"`
}

// NAT client has two parts:
// ecsClient connect EcsEndpoint
// natClient connect natClient
type NATClient struct {
	// ServiceClient is a general service client defines a client used to connect an Endpoint defined in elb_connection.go
	natClient *ServiceClient
	vpcClient *ServiceClient
	throttler *Throttler
}

func NewNATClient(natEndpoint, vpcEndpoint, id, accessKey, secretKey, securityToken, region, serviceType string) *NATClient {
	access := &AccessInfo{
		AccessKey:     accessKey,
		SecretKey:     secretKey,
		SecurityToken: securityToken,
		Region:        region,
		ServiceType:   serviceType,
	}
	natClient := &ServiceClient{
		Client:   httpClient,
		Endpoint: natEndpoint,
		Access:   access,
		TenantId: id,
	}
	vpcClient := &ServiceClient{
		Client:   httpClient,
		Endpoint: vpcEndpoint,
		Access:   access,
		TenantId: id,
	}

	return &NATClient{
		natClient: natClient,
		vpcClient: vpcClient,
		throttler: throttler,
	}
}

/*
 *    >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
 *               NAT implement of functions regrding NAT gateway
 *    >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
 */
func (nat *NATClient) GetNATGateway(natGatewayId string) (*NATGateway, error) {
	url := "/v2/" + nat.natClient.TenantId + "/nat_gateways/" + natGatewayId
	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(nat.natClient, nat.throttler.GetThrottleByKey(NAT_GATEWAY_GET), req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Failed to GetNATGateway: No Nat Gateway exist with id %v", natGatewayId)
	}

	var natResp NATArr
	err = DecodeBody(resp, &natResp)
	if err != nil {
		return nil, fmt.Errorf("Failed to GetNATGateway : %v", err)
	}

	return &(natResp.NATGateway), nil
}

func (nat *NATClient) ListNATGateways(params map[string]string) (*NATGatewayList, error) {
	url := "/v2/" + nat.natClient.TenantId + "/nat_gateways"
	var query string
	if len(params) != 0 {
		query += "?"

		for key, value := range params {
			query += fmt.Sprintf("%s=%s&", key, value)
		}

		query = query[0 : len(query)-1]
	}

	url += query
	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(nat.natClient, nat.throttler.GetThrottleByKey(NAT_GATEWAY_LIST), req)
	if err != nil {
		return nil, err
	}

	var natList NATGatewayList
	err = DecodeBody(resp, &natList)
	if err != nil {
		return nil, fmt.Errorf("Failed to get NATList : %v", err)
	}

	return &natList, nil
}

func (nat *NATClient) CreateDNATRule(dnatRuleConf *DNATRule) (*DNATRule, error) {
	var dnatRule DNATRuleArr
	dnatRule.DNATRule = *dnatRuleConf

	url := "/v2/" + nat.natClient.TenantId + "/dnat_rules"
	req := NewRequest(http.MethodPost, url, nil, &dnatRule)

	resp, err := DoRequest(nat.natClient, nat.throttler.GetThrottleByKey(NAT_RULE_CREATE), req)
	if err != nil {
		return nil, err
	}

	var dnatRuleResp DNATRuleArr
	err = DecodeBody(resp, &dnatRuleResp)
	if err != nil {
		return nil, fmt.Errorf("Failed to CreateDNATRule : %v", err)
	}
	return &(dnatRuleResp.DNATRule), nil
}

func (nat *NATClient) DeleteDNATRule(dnatRuleId string, natGatewayId string) error {
	url := "/v2/" + nat.natClient.TenantId + "/nat_gateways/" + natGatewayId + "/dnat_rules/" + dnatRuleId
	req := NewRequest(http.MethodDelete, url, nil, nil)

	resp, err := DoRequest(nat.natClient, nat.throttler.GetThrottleByKey(NAT_RULE_DELETE), req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		resBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Failed to DeleteDNATRule : %s, status code: %d", string(resBody), resp.StatusCode)
	}

	return nil
}

func (nat *NATClient) GetDNATRule(dnatRuleId string) (*DNATRule, error) {
	url := "/v2/" + nat.natClient.TenantId + "/dnat_rules" + dnatRuleId
	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(nat.natClient, nat.throttler.GetThrottleByKey(NAT_RULE_GET), req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Failed to GetDNATRule: No DNAT rule exist with id %v", dnatRuleId)
	}

	var dnatRuleResp DNATRuleArr
	err = DecodeBody(resp, &dnatRuleResp)
	if err != nil {
		return nil, fmt.Errorf("Failed to GetDNATRule : %v", err)
	}

	return &(dnatRuleResp.DNATRule), nil
}

func (nat *NATClient) ListDNATRules(params map[string]string) (*DNATRuleList, error) {
	url := "/v2/" + nat.natClient.TenantId + "/dnat_rules"
	var query string
	if len(params) != 0 {
		query += "?"

		for key, value := range params {
			query += fmt.Sprintf("%s=%s&", key, value)
		}

		query = query[0 : len(query)-1]
	}

	url += query
	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(nat.natClient, nat.throttler.GetThrottleByKey(NAT_RULE_LIST), req)
	if err != nil {
		return nil, err
	}

	var dnatRuleList DNATRuleList
	err = DecodeBody(resp, &dnatRuleList)
	if err != nil {
		return nil, fmt.Errorf("Failed to Get DNATRuleList : %v", err)
	}

	return &dnatRuleList, nil
}

func (nat *NATClient) ListPorts(params map[string]string) (*PortList, error) {
	url := "/v2.0/ports"
	var query string
	if len(params) != 0 {
		query += "?"

		for key, value := range params {
			query += fmt.Sprintf("%s=%s&", key, value)
		}

		query = query[0 : len(query)-1]
	}

	url += query
	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(nat.vpcClient, nil, req)
	if err != nil {
		return nil, err
	}

	var portList PortList
	err = DecodeBody(resp, &portList)
	if err != nil {
		return nil, fmt.Errorf("Failed to Get PortList : %v", err)
	}

	return &portList, nil
}

func (nat *NATClient) GetPort(portId string) (*Port, error) {
	url := "/v2.0/ports/" + portId
	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(nat.vpcClient, nil, req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Failed to GetPort: No port exist with id %v", portId)
	}
	var portResp PortArr
	err = DecodeBody(resp, &portResp)
	if err != nil {
		return nil, fmt.Errorf("Failed to get Port : %v", err)
	}

	return &(portResp.Port), nil
}

func (nat *NATClient) ListFloatings(params map[string]string) (*FloatingIpList, error) {
	url := "/v2.0/floatingips"
	var query string
	if len(params) != 0 {
		query += "?"

		for key, value := range params {
			query += fmt.Sprintf("%s=%s&", key, value)
		}

		query = query[0 : len(query)-1]
	}

	url += query
	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(nat.vpcClient, nil, req)
	if err != nil {
		return nil, err
	}

	var floatingIpList FloatingIpList
	err = DecodeBody(resp, &floatingIpList)
	if err != nil {
		return nil, fmt.Errorf("Failed to Get FloatingIpList : %v", err)
	}

	return &floatingIpList, nil
}
