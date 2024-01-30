// nolint: golint
package model

import (
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/utils"

	"strings"
)

// Response Object
type ListLoadbalancersRsp struct {

	// 负载均衡器对象列表
	Loadbalancers  *[]Loadbalancer `json:"loadbalancers,omitempty"`
	HttpStatusCode int             `json:"-"`
}

func (o ListLoadbalancersRsp) String() string {
	data, err := utils.Marshal(o)
	if err != nil {
		return "ListLoadbalancersResponse struct{}"
	}

	return strings.Join([]string{"ListLoadbalancersResponse", string(data)}, " ")
}

type ShowLoadbalancerResponse struct {
	Loadbalancer   *Loadbalancer `json:"loadbalancer,omitempty"`
	HttpStatusCode int           `json:"-"`
}

func (o ShowLoadbalancerResponse) String() string {
	data, err := utils.Marshal(o)
	if err != nil {
		return "ShowLoadbalancerResponse struct{}"
	}

	return strings.Join([]string{"ShowLoadbalancerResponse", string(data)}, " ")
}
