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
	"fmt"
	"testing"

	huaweicloudsdkecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
)

func TestAddressesFromServer(t *testing.T) {
	fixed := huaweicloudsdkecsmodel.GetServerAddressOSEXTIPStypeEnum().FIXED
	var addr1 = huaweicloudsdkecsmodel.ServerAddress{
		Version:      "4",
		Addr:         "192.168.1.122",
		OSEXTIPStype: &fixed,
	}
	floating := huaweicloudsdkecsmodel.GetServerAddressOSEXTIPStypeEnum().FLOATING
	var addr2 = huaweicloudsdkecsmodel.ServerAddress{
		Version:      "4",
		Addr:         "159.138.131.176",
		OSEXTIPStype: &floating,
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
	serverInfo.Flavor = &huaweicloudsdkecsmodel.ServerFlavor{
		Id: "s3.xlarge.4",
	}

	instance := Instances{}
	instanceType, err := instance.parseInstanceTypeFromServerInfo(&serverInfo)
	if err != nil {
		t.Fatalf("parse instance type failed with error: %v", err)
	}

	if instanceType != serverInfo.Flavor.Id {
		t.Fatalf("expect instance type: %s, but got %s.", serverInfo.Flavor.Id, instanceType)
	}
}

func TestIsNonExistError(t *testing.T) {
	var tests = []struct {
		name       string
		error      error
		isNonExist bool
	}{
		{
			name:       "not a non exist error",
			error:      fmt.Errorf("{\"status_code\":404,\"request_id\":\"0dca522c65f45fd2cc56d28986c05fee\",\"error_code\":\"non non-exist\",\"error_message\":\"Instance[a44af098-7548-4519-8243-a88ba3e5de4fnoexist] could not be found.\"}"),
			isNonExist: false,
		},
		{
			name:       "non-exist error",
			error:      fmt.Errorf("{\"status_code\":404,\"request_id\":\"0dca522c65f45fd2cc56d28986c05fee\",\"error_code\":\"Ecs.0114\",\"error_message\":\"Instance[a44af098-7548-4519-8243-a88ba3e5de4fnoexist] could not be found.\"}"),
			isNonExist: true,
		},
	}

	instance := Instances{}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			if instance.isNonExistError(tc.error) != tc.isNonExist {
				t.Fatalf("expect isNonExist: %v, but got: %v", tc.isNonExist, instance.isNonExistError(tc.error))
			}
		})
	}
}
