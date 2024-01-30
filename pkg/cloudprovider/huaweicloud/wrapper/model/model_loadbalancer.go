// nolint: golint
package model

import (
	"errors"
	"strings"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/utils"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/converter"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v2/model"
)

// 负载均衡器响应体
type Loadbalancer struct {

	// 负载均衡器ID
	Id string `json:"id"`

	// 负载均衡器所在的项目ID。
	TenantId string `json:"tenant_id"`

	// 负载均衡器名称。
	Name string `json:"name"`

	// 负载均衡器的描述信息
	Description string `json:"description"`

	// 负载均衡器所在的子网的IPv4网络ID。仅支持内网类型。
	VipSubnetId string `json:"vip_subnet_id"`

	// 负载均衡器虚拟IP对应的端口ID
	VipPortId string `json:"vip_port_id"`

	// 负载均衡器的虚拟IP。
	VipAddress string `json:"vip_address"`

	// 负载均衡器关联的监听器ID的列表
	Listeners []model.ResourceList `json:"listeners"`

	// 负载均衡器关联的后端云服务器组ID的列表。
	Pools []model.ResourceList `json:"pools"`

	// 负载均衡器的供应者名称。只支持vlb
	Provider string `json:"provider"`

	// 负载均衡器的操作状态
	OperatingStatus LoadbalancerOperatingStatus `json:"operating_status"`

	// 负载均衡器的配置状态
	ProvisioningStatus LoadbalancerProvisioningStatus `json:"provisioning_status"`

	// 负载均衡器的管理状态。只支持设定为true，该字段的值无实际意义。
	AdminStateUp bool `json:"admin_state_up"`

	// 负载均衡器的创建时间
	CreatedAt string `json:"created_at"`

	// 负载均衡器的更新时间
	UpdatedAt string `json:"updated_at"`

	// 负载均衡器的企业项目ID。
	EnterpriseProjectId string `json:"enterprise_project_id"`

	// 负载均衡器所在的项目ID。
	ProjectId string `json:"project_id"`

	// 负载均衡器的标签列表
	Tags []string `json:"tags"`

	PublicIPs []PublicIP `json:"publicips"`
}

func (o Loadbalancer) String() string {
	data, err := utils.Marshal(o)
	if err != nil {
		return "Loadbalancer struct{}"
	}

	return strings.Join([]string{"Loadbalancer", string(data)}, " ")
}

type LoadbalancerOperatingStatus struct {
	value string
}

type LoadbalancerOperatingStatusEnum struct {
	ONLINE     LoadbalancerOperatingStatus
	OFFLINE    LoadbalancerOperatingStatus
	DEGRADED   LoadbalancerOperatingStatus
	DISABLED   LoadbalancerOperatingStatus
	NO_MONITOR LoadbalancerOperatingStatus
}

func GetLoadbalancerOperatingStatusEnum() LoadbalancerOperatingStatusEnum {
	return LoadbalancerOperatingStatusEnum{
		ONLINE: LoadbalancerOperatingStatus{
			value: "ONLINE",
		},
		OFFLINE: LoadbalancerOperatingStatus{
			value: "OFFLINE",
		},
		DEGRADED: LoadbalancerOperatingStatus{
			value: "DEGRADED",
		},
		DISABLED: LoadbalancerOperatingStatus{
			value: "DISABLED",
		},
		NO_MONITOR: LoadbalancerOperatingStatus{
			value: "NO_MONITOR",
		},
	}
}

func (c LoadbalancerOperatingStatus) Value() string {
	return c.value
}

func (c LoadbalancerOperatingStatus) MarshalJSON() ([]byte, error) {
	return utils.Marshal(c.value)
}

func (c *LoadbalancerOperatingStatus) UnmarshalJSON(b []byte) error {
	myConverter := converter.StringConverterFactory("string")
	if myConverter != nil {
		val, err := myConverter.CovertStringToInterface(strings.Trim(string(b[:]), "\""))
		if err == nil {
			c.value = val.(string)
			return nil
		}
		return err
	} else {
		return errors.New("convert enum data to string error")
	}
}

type LoadbalancerProvisioningStatus struct {
	value string
}

type LoadbalancerProvisioningStatusEnum struct {
	ACTIVE         LoadbalancerProvisioningStatus
	PENDING_CREATE LoadbalancerProvisioningStatus
	ERROR          LoadbalancerProvisioningStatus
}

func GetLoadbalancerProvisioningStatusEnum() LoadbalancerProvisioningStatusEnum {
	return LoadbalancerProvisioningStatusEnum{
		ACTIVE: LoadbalancerProvisioningStatus{
			value: "ACTIVE",
		},
		PENDING_CREATE: LoadbalancerProvisioningStatus{
			value: "PENDING_CREATE",
		},
		ERROR: LoadbalancerProvisioningStatus{
			value: "ERROR",
		},
	}
}

func (c LoadbalancerProvisioningStatus) Value() string {
	return c.value
}

func (c LoadbalancerProvisioningStatus) MarshalJSON() ([]byte, error) {
	return utils.Marshal(c.value)
}

func (c *LoadbalancerProvisioningStatus) UnmarshalJSON(b []byte) error {
	myConverter := converter.StringConverterFactory("string")
	if myConverter != nil {
		val, err := myConverter.CovertStringToInterface(strings.Trim(string(b[:]), "\""))
		if err == nil {
			c.value = val.(string)
			return nil
		}
		return err
	} else {
		return errors.New("convert enum data to string error")
	}
}

type PublicIP struct {
	ID        string `json:"publicip_id"`
	Address   string `json:"publicip_address"`
	IPVersion int    `json:"ip_version"`
}
