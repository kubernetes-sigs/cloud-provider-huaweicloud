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

	"github.com/RainbowMango/huaweicloud-sdk-go/openstack/compute/v2/servers"
)

func TestParseNodeAddressFromServerInfo(t *testing.T) {
	var serverInfo servers.Server
	var addresses = make(map[string]interface{}, 1)

	addresses["cc24f1c9-9357-465a-bcc2-329d17001824"] = []Address{
		{
			Addr: "192.168.1.122",
			Type: "fixed",
		},
		{
			Addr: "159.138.131.176",
			Type: "floating",
		},
	}

	serverInfo.Addresses = addresses

	instance := Instances{}
	addrs, err := instance.parseNodeAddressFromServerInfo(&serverInfo)
	if err != nil {
		t.Fatalf("parse node address failed with error: %v", err)
	}

	if len(addrs) != 2 {
		t.Fatalf("expect 2 address, but got %d. addrs: %v", len(addrs), addrs)
	}
}

func TestParseInstanceTypeFromServerInfo(t *testing.T) {
	var serverInfo servers.Server
	var flavor = make(map[string]interface{}, 1)

	flavor["id"] = "s3.xlarge.4"

	serverInfo.Flavor = flavor

	instance := Instances{}
	instanceType, err := instance.parseInstanceTypeFromServerInfo(&serverInfo)
	if err != nil {
		t.Fatalf("parse instance type failed with error: %v", err)
	}

	if instanceType != flavor["id"] {
		t.Fatalf("expect instance type: %s, but got %s.", flavor["id"], instanceType)
	}
}
