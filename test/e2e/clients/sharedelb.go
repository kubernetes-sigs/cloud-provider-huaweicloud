package clients

import (
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/elb/v2/model"
	"github.com/onsi/gomega"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/wrapper"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
)

func CreateSharedELBInstance(authOpts *config.AuthOptions, subnetID, name string) string {
	sharedElbClient := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}

	instance, err := sharedElbClient.CreateInstance(&model.CreateLoadbalancerReq{
		Name:        &name,
		VipSubnetId: subnetID,
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return instance.Id
}

func DeleteSharedELBInstance(authOpts *config.AuthOptions, id string) {
	sharedElbClient := wrapper.SharedLoadBalanceClient{AuthOpts: authOpts}
	err := sharedElbClient.DeleteInstance(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}
