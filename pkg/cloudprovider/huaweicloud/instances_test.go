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

package huaweicloud

import (
	"testing"

	huaweicloudsdkecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
)

func TestAddressesFromServer(t *testing.T) {
	var addr1 = huaweicloudsdkecsmodel.ServerAddress{
		Version:            "4",
		Addr:               "192.168.1.122",
		OSEXTIPStype:       huaweicloudsdkecsmodel.GetServerAddressOSEXTIPStypeEnum().FIXED,
		OSEXTIPSMACmacAddr: "fa:16:3e:c3:85:c2",
		OSEXTIPSportId:     "b0b37c62-2514-4fcd-9dee-47933a7fa668",
	}
	var addr2 = huaweicloudsdkecsmodel.ServerAddress{
		Version:            "4",
		Addr:               "159.138.131.176",
		OSEXTIPStype:       huaweicloudsdkecsmodel.GetServerAddressOSEXTIPStypeEnum().FLOATING,
		OSEXTIPSMACmacAddr: "fa:16:3e:c3:85:c2",
		OSEXTIPSportId:     "b0b37c62-2514-4fcd-9dee-47933a7fa668",
	}

	var server = &huaweicloudsdkecsmodel.ServerDetail{
		Addresses: map[string][]huaweicloudsdkecsmodel.ServerAddress{
			"cc24f1c9-9357-465a-bcc2-329d17001824": {addr1, addr2},
		},
	}

	instance := Instances{}
	addrs, err := instance.parseAddressesFromServer(server)
	if err != nil {
		t.Fatalf("parse node address failed with error: %v", err)
	}

	if len(addrs) != 2 {
		t.Fatalf("expect 2 address, but got %d. addrs: %v", len(addrs), addrs)
	}
}

func TestParseInstanceTypeFromServerInfo(t *testing.T) {
	var serverInfo huaweicloudsdkecsmodel.ServerDetail
	serverInfo.Flavor.Id = "s3.xlarge.4"

	instance := Instances{}
	instanceType, err := instance.parseInstanceTypeFromServerInfo(&serverInfo)
	if err != nil {
		t.Fatalf("parse instance type failed with error: %v", err)
	}

	if instanceType != serverInfo.Flavor.Id {
		t.Fatalf("expect instance type: %s, but got %s.", serverInfo.Flavor.Id, instanceType)
	}
}
