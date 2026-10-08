package clients

import (
	"fmt"

	model "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eip/v2/model"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/wrapper"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/config"
)

func CreateEip(authOpts *config.AuthOptions) *model.PublicipCreateResp {
	eipClient := wrapper.EIpClient{AuthOpts: authOpts}

	name := fmt.Sprintf("e2e_test_%s", rand.String(5))
	size := int32(5)
	shareType := model.GetCreatePublicipBandwidthOptionShareTypeEnum()
	eip, err := eipClient.Create(&model.CreatePublicipRequestBody{
		Bandwidth: &model.CreatePublicipBandwidthOption{
			Name:      &name,
			Size:      &size,
			ShareType: shareType.PER,
		},
		Publicip: &model.CreatePublicipOption{Type: "5_bgp"},
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return eip
}

func DeleteEip(authOpts *config.AuthOptions, id string) {
	eipClient := wrapper.EIpClient{AuthOpts: authOpts}
	err := eipClient.Delete(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}

func CreateSharedBandwidth(authOpts *config.AuthOptions) *model.BandwidthResp {
	eipClient := wrapper.EIpClient{AuthOpts: authOpts}

	name := fmt.Sprintf("e2e_shared_%s", rand.String(5))
	size := int32(5)
	resp, err := eipClient.CreateSharedBandwidth(&model.CreateSharedBandwidthRequest{
		Body: &model.CreateSharedBandwidhRequestBody{
			Bandwidth: &model.CreateSharedBandwidthOption{
				Name: name,
				Size: size,
			},
		},
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	return resp
}

func DeleteSharedBandwidth(authOpts *config.AuthOptions, id string) {
	eipClient := wrapper.EIpClient{AuthOpts: authOpts}
	err := eipClient.DeleteSharedBandwidth(id)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}

func FindEipIDByPortID(authOpts *config.AuthOptions, portID string) string {
	eipClient := wrapper.EIpClient{AuthOpts: authOpts}
	portIDs := []string{portID}
	eips, err := eipClient.List(&model.ListPublicipsRequest{PortId: &portIDs})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(eips).ShouldNot(gomega.BeEmpty())
	return *eips[0].Id
}
