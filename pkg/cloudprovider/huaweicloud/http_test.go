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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShortRequestDump(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{
			name: "Nominal case",
			req:  httptest.NewRequest(http.MethodGet, "https://elb.cn-north-7.ulanqab.huawei.com/v1", nil),
			want: "GET /v1 HTTP/1.1 HOST: elb.cn-north-7.ulanqab.huawei.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lrt := &LogRoundTripper{}
			if got := lrt.shortRequestDump(tt.req); string(got) != tt.want {
				t.Errorf("LogRoundTripper.logRequest() = %s, want %s", string(got), string(tt.want))
			}
		})
	}
}
