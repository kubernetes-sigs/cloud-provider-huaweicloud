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
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/huaweicloud/golangsdk"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog"
)

const (
	HwsHeaderXHwsDate   string = "X-Hws-Date"
	Authorization       string = "Authorization"
	HwsHost             string = "iam.hws.com"
	TransportHttp       string = "http"
	TransportHttps      string = "https"
	HeaderSecurityToken string = "X-Security-Token"
	HeaderProject       string = "X-Project-Id"

	// longThrottleLatency defines threshold for logging requests. All requests being
	// throttle for more than longThrottleLatency will be logged.
	longThrottleLatency = 50 * time.Millisecond
)

type AccessInfo struct {
	Region        string
	AccessKey     string
	SecretKey     string
	SecurityToken string
	ServiceType   string
}

// request is used to help build up a request
type request struct {
	method  string
	url     string
	params  url.Values
	obj     interface{}
	headers map[string]string
}

var httpClient *http.Client

var throttler *Throttler

func init() {
	httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			Dial: func(netw, addr string) (net.Conn, error) {
				c, err := net.DialTimeout(netw, addr, time.Second*15)
				if err != nil {
					return nil, err
				}
				return c, nil
			},
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: time.Second * 15,
		},
	}

	var err error
	throttler, err = InitialThrottler()
	if err != nil {
		panic(err)
	}
}

// newRequest is used to create a new request
// if accessIn == nil mean not to sign header
func NewRequest(method, url string, headersIn map[string]string, obj interface{}) *request {
	r := &request{
		method:  method,
		url:     url,
		params:  make(map[string][]string),
		headers: headersIn,
		obj:     obj,
	}
	return r
}

// decodeBody is used to JSON decode a body
func DecodeBody(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	resBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("request failed: %s, status code: %d", string(resBody), resp.StatusCode)
	}

	if len(strings.Replace(string(resBody), " ", "", -1)) <= 2 {
		return nil
	}

	dec := json.NewDecoder(bytes.NewReader(resBody))

	err = dec.Decode(out)
	if err != nil {
		return fmt.Errorf("Decode failed: %s, err: %v", string(resBody), err)
	}

	return nil
}

// encodeBody is used to encode a request body
func encodeBody(obj interface{}) (io.Reader, error) {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(obj); err != nil {
		return nil, fmt.Errorf("encode obj error")
	}
	return buf, nil
}

// doRequest runs a request with our client
func DoRequest(service *ServiceClient, throttle flowcontrol.RateLimiter, r *request) (*http.Response, error) {
	//client := service.Client
	var body io.Reader
	// Check if we should encode the body
	if r.obj != nil {
		if b, err := encodeBody(r.obj); err != nil {
			return nil, err
		} else {
			body = b
		}
	}

	tryThrottle(throttle, r)

	url := service.Endpoint + r.url
	// Create the HTTP request
	req, err := http.NewRequest(r.method, url, body)
	if err != nil {
		return nil, fmt.Errorf("http new request error")
	}
	req.Close = true

	// add the sign to request header if needed.
	if service.Access != nil {
		req.Header.Set(HeaderProject, service.TenantId)

		// distinguish 'Permanent Security Credentials' and 'Temporary Security Credentials'
		// HeaderSecurityToken only be set in case of 'Temporary Security Credentials'.
		// TODO(RainbowMango): Remove this ugly code and refactor later.
		if service.Access.SecurityToken != "" {
			req.Header.Set(HeaderSecurityToken, service.Access.SecurityToken)
		}

		golangsdk.Sign(req, golangsdk.SignOptions{
			AccessKey: service.Access.AccessKey,
			SecretKey: service.Access.SecretKey,
		})
	}

	resp, err := service.Client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("http client do request error. %v", err)
	}

	return resp, nil
}

func tryThrottle(throttle flowcontrol.RateLimiter, r *request) {
	now := time.Now()
	if throttle != nil {
		throttle.Accept()
	}
	if latency := time.Since(now); latency > longThrottleLatency {
		klog.V(2).Infof("Throttling request took %v, request: %s:%s", latency, r.method, r.url)
	}
}
