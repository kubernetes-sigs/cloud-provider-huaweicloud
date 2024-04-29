/*
Copyright 2022 The Kubernetes Authors.

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

package config

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	sdkconfig "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/httphandler"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/region"
	"gopkg.in/gcfg.v1"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/utils"
)

// CloudConfig define
type CloudConfig struct {
	AuthOpts AuthOptions `gcfg:"Global"`
	VpcOpts  VpcOptions  `gcfg:"Vpc"`
}

type VpcOptions struct {
	ID              string `gcfg:"id"`
	SubnetID        string `gcfg:"subnet-id"`
	SecurityGroupID string `gcfg:"security-group-id"`
}

type AuthOptions struct {
	Cloud     string `gcfg:"cloud"`
	AuthURL   string `gcfg:"auth-url"`
	Region    string `gcfg:"region"`
	AccessKey string `gcfg:"access-key"`
	SecretKey string `gcfg:"secret-key"`
	ProjectID string `gcfg:"project-id"`
}

func (a *AuthOptions) GetCredentials() *basic.Credentials {
	return basic.NewCredentialsBuilder().
		WithAk(a.AccessKey).
		WithSk(a.SecretKey).
		WithProjectId(a.ProjectID).
		Build()
}

func (a *AuthOptions) GetHcClient(catalogName string) *core.HcHttpClient {
	cloud := "myhuaweicloud.com"
	if strings.TrimSpace(a.Cloud) != "" {
		cloud = strings.TrimSpace(a.Cloud)
	}
	r := region.NewRegion(catalogName, fmt.Sprintf("https://%s.%s.%s", catalogName, a.Region, cloud))

	client := core.NewHcHttpClientBuilder().
		WithRegion(r).
		WithCredential(a.GetCredentials()).
		WithHttpConfig(newHTTPConfig()).
		Build()

	client.PreInvoke(map[string]string{
		"User-Agent": "huaweicloud-kubernetes-ccm",
	})
	return client
}

func newHTTPConfig() *sdkconfig.HttpConfig {
	lrt := utils.LogRoundTripper{}
	var err error

	defConfig := sdkconfig.DefaultHttpConfig()
	defConfig.Retries = 3

	httpHandler := httphandler.NewHttpHandler()
	defConfig.HttpHandler = httpHandler

	httpHandler.AddRequestHandler(func(request http.Request) {
		klog.V(6).Infof("Request: [%s] %s\nHeaders: %s",
			request.Method, request.URL, utils.FormatHeaders(request.Header, "\n    "))

		if request.Body != nil {
			request.Body, err = lrt.LogRequest(request.Body, request.Header.Get("Content-Type"))
			if err != nil {
				klog.Errorf("error printing request logs : %s", err)
			}
		}
	})

	httpHandler.AddResponseHandler(func(response http.Response) {
		klog.V(6).Infof("Response:\nStatus Code: %d\nHeaders: %s",
			response.StatusCode, utils.FormatHeaders(response.Header, "\n    "))

		response.Body, err = lrt.LogResponse(response.Body, response.Header.Get("Content-Type"))
		if err != nil {
			klog.Errorf("error printing response logs : %s", err)
		}
	})

	httpHandler.AddMonitorHandler(func(m *httphandler.MonitorMetric) {
		klog.Infof("%s https://%s%s%s %d in %d milliseconds, request ID: %s",
			m.Method, m.Host, m.Path, m.Raw, m.StatusCode, m.Latency.Milliseconds(), m.RequestId)
	})

	return defConfig
}

func ReadConfig(cfg io.Reader) (*CloudConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("Must provide a config file")
	}
	cc := &CloudConfig{}
	// Read configuration
	err := gcfg.FatalOnly(gcfg.ReadInto(cc, cfg))
	if err != nil {
		return nil, err
	}
	// Set default value
	setDefaultConfig(cc)
	return cc, nil
}

func setDefaultConfig(cc *CloudConfig) {
	if cc.AuthOpts.Cloud == "" {
		cc.AuthOpts.Cloud = "myhuaweicloud.com"
	}
	if cc.AuthOpts.AuthURL == "" {
		cc.AuthOpts.AuthURL = fmt.Sprintf("https://iam.%s:443/v3/", cc.AuthOpts.Cloud)
	}
}
