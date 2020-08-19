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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/RainbowMango/huaweicloud-sdk-go"
	"github.com/RainbowMango/huaweicloud-sdk-go/openstack/compute/v2/servers"
	th "github.com/RainbowMango/huaweicloud-sdk-go/testhelper"
	"github.com/RainbowMango/huaweicloud-sdk-go/testhelper/client"
	huaweicloudsdkecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"

	"k8s.io/apimachinery/pkg/types"
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

func TestInstanceID(t *testing.T) {
	tests := []struct {
		name     string
		nodeName types.NodeName
		servers  []servers.Server
		wantID   string
		wantErr  bool
	}{
		{
			name:     "Success case",
			nodeName: types.NodeName("foo"),
			servers: []servers.Server{
				{
					ID: "9e5476bd-a4ec-4653-93d6-72c93aa682bb",
				},
			},
			wantID:  "9e5476bd-a4ec-4653-93d6-72c93aa682bb",
			wantErr: false,
		},
		{
			name:     "Too many servers",
			nodeName: types.NodeName("foo"),
			servers: []servers.Server{
				{
					ID: "9e5476bd-a4ec-4653-93d6-72c93aa682bb",
				},
				{
					ID: "9e5476bd-a4ec-4653-93d6-72c93aa682cc",
				},
			},
			wantErr: true,
		},
		{
			name:     "Not found",
			nodeName: types.NodeName("foo"),
			servers:  []servers.Server{},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th.SetupHTTP()
			defer th.TeardownHTTP()
			MockListServers(t, tt.servers)
			i := Instances{
				GetServerClientFunc: func() (*gophercloud.ServiceClient, error) {
					return client.ServiceClient(), nil
				},
			}
			id, err := i.InstanceID(context.Background(), tt.nodeName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Got error: %v, wantErr %v", err, tt.wantErr)
				return
			}
			if id != tt.wantID {
				t.Errorf("Expected ID %v, but got %v", tt.wantID, id)
			}
		})
	}
}

type ServerResponse struct {
	Servers []servers.Server `json:"servers"`
}

func MockListServers(t *testing.T, servers []servers.Server) {
	res, err := json.Marshal(ServerResponse{Servers: servers})
	if err != nil {
		t.Fatalf("Error occurred while marshaling the response: %v", err)
	}
	// Handle server list requests.
	th.Mux.HandleFunc("/servers/detail", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestHeader(t, r, "X-Auth-Token", client.TokenID)

		w.Header().Add("Content-Type", "application/json")
		r.ParseForm()
		marker := r.Form.Get("marker")
		switch marker {
		case "":
			fmt.Fprintf(w, string(res))
		default:
			t.Fatalf("/servers/detail invoked with unexpected marker=[%s]", marker)
		}
	})
}
