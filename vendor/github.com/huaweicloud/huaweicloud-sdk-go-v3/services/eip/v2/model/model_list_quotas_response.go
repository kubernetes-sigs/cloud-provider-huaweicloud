package model

import (
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/utils"

	"strings"
)

// ListQuotasResponse Response Object
type ListQuotasResponse struct {
	Quotas         *ResourceResp `json:"quotas,omitempty"`
	HttpStatusCode int           `json:"-"`
}

func (o ListQuotasResponse) String() string {
	data, err := utils.Marshal(o)
	if err != nil {
		return "ListQuotasResponse struct{}"
	}

	return strings.Join([]string{"ListQuotasResponse", string(data)}, " ")
}
