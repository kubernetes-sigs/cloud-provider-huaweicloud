package wrapper

import (
	vpc "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpc/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/vpc/v2/model"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
)

type VpcClient struct {
	AuthOpts *config.AuthOptions
}

func (c *VpcClient) ListSecurityGroupRules(securityGroupID string) ([]model.SecurityGroupRule, error) {
	var rst []model.SecurityGroupRule
	err := c.wrapper(func(c *vpc.VpcClient) (interface{}, error) {
		return c.ListSecurityGroupRules(&model.ListSecurityGroupRulesRequest{
			SecurityGroupId: &securityGroupID,
		})
	}, "SecurityGroupRules", &rst)
	return rst, err
}

func (c *VpcClient) CreateSecurityGroupRule(rule *model.CreateSecurityGroupRuleOption) (*model.SecurityGroupRule, error) {
	var rst *model.SecurityGroupRule
	err := c.wrapper(func(c *vpc.VpcClient) (interface{}, error) {
		return c.CreateSecurityGroupRule(&model.CreateSecurityGroupRuleRequest{
			Body: &model.CreateSecurityGroupRuleRequestBody{
				SecurityGroupRule: rule,
			},
		})
	}, "SecurityGroupRule", &rst)
	return rst, err
}

func (c *VpcClient) DeleteSecurityGroupRule(ruleID string) error {
	return c.wrapper(func(c *vpc.VpcClient) (interface{}, error) {
		return c.DeleteSecurityGroupRule(&model.DeleteSecurityGroupRuleRequest{
			SecurityGroupRuleId: ruleID,
		})
	})
}

func (c *VpcClient) wrapper(handler func(*vpc.VpcClient) (interface{}, error), args ...interface{}) error {
	return commonWrapper(func() (interface{}, error) {
		hc := c.AuthOpts.GetHcClient("vpc")
		return handler(vpc.NewVpcClient(hc))
	}, OKCodes, args...)
}
