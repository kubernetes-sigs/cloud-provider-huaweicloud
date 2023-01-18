package model

import (
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/utils"

	"strings"
)

// Request Object
type ListAvailabilityZonesRequest struct {

	// AZ组。
	PublicBorderGroup *string `json:"public_border_group,omitempty"`
}

func (o ListAvailabilityZonesRequest) String() string {
	data, err := utils.Marshal(o)
	if err != nil {
		return "ListAvailabilityZonesRequest struct{}"
	}

	return strings.Join([]string{"ListAvailabilityZonesRequest", string(data)}, " ")
}
