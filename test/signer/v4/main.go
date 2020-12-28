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

package main

import (
	"fmt"
	"os"

	huaweicloudsdkbasic "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	huaweicloudsdkconfig "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	huaweicloudsdkecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	huaweicloudsdkecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud"
)

const (
	configPathEnv = "CCMConfigPath"
	serverIDEnv   = "ServerID"
)

func readConfigFromEnv() (config *huaweicloud.CloudConfig, serverID string) {
	configPath := os.Getenv(configPathEnv)
	if len(configPath) == 0 {
		fmt.Printf("Please set config file path. e.g. 'export CCMConfigPath=/etc/provider.conf'.\n")
		return nil, ""
	}

	fileHandler, err := os.Open(configPath)
	if err != nil {
		fmt.Printf("Failed to open file: %s, error: %v\n", configPath, err)
		return nil, ""
	}
	defer fileHandler.Close()

	config, err = huaweicloud.ReadConf(fileHandler)
	if err != nil {
		fmt.Printf("Failed to parse config file: %s.\n", configPath)
		return nil, ""
	}

	serverID = os.Getenv(serverIDEnv)
	if len(serverID) == 0 {
		fmt.Printf("Please set server ID. e.g. 'export ServerID=a44af098-7548-4519-8243-a88ba3e5de4g'.\n")
		return nil, ""
	}

	return config, serverID
}

func main() {
	config, serverID := readConfigFromEnv()
	if config == nil || len(serverID) == 0 {
		return
	}

	credentials := huaweicloudsdkbasic.NewCredentialsBuilder().
		WithAk(config.Auth.AccessKey).
		WithSk(config.Auth.SecretKey).
		WithProjectId(config.Auth.ProjectID).
		Build()

	hcClient := huaweicloudsdkecs.EcsClientBuilder().
		WithEndpoint(config.Auth.ECSEndpoint).
		WithCredential(credentials).
		WithHttpConfig(huaweicloudsdkconfig.DefaultHttpConfig()).
		Build()

	ecsClient := huaweicloudsdkecs.NewEcsClient(hcClient)

	reqOpt := &huaweicloudsdkecsmodel.ShowServerRequest{
		ServerId: serverID,
	}

	response, err := ecsClient.ShowServer(reqOpt)
	if err != nil {
		fmt.Printf("request failed with error: %v\n", err)
		return
	}

	serverName := response.Server.Name
	fmt.Printf("Congratulations! Your authentication information is suitable. server name: %s\n", serverName)
}
