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
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

func TestGenerateSigningKey(t *testing.T) {
	s := Signer{
		AccessKey: "AKIDEXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		Region:    "cn-north-1",
		Service:   "ec2",
	}
	tt, _ := time.Parse(time.RFC1123, "Mon, 09 Sep 2011 23:36:00 GMT")
	k, err := GenerateSigningKey(s.SecretKey, s.Region, s.Service, tt)
	if err != nil {
		t.Fatal("failed to generate signing key", string(k))
	}
	if fmt.Sprintf("%x", k) != "65c1bdc2d0bbb66a2724952db79ea3d680c6d1d2e2631880f49f3d5a5709d94e" {
		t.Fatal("wrong key")
	}
}

func TestCredentailScope(t *testing.T) {
	s := Signer{
		AccessKey: "AKIDEXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		Region:    "cn-north-1",
		Service:   "ec2",
	}
	tt, _ := time.Parse(time.RFC1123, "Mon, 09 Sep 2011 23:36:00 GMT")
	credentialScope := CredentialScope(tt, s.Region, s.Service)
	if credentialScope != "20110909/cn-north-1/ec2/sdk_request" {
		t.Fatal("wrong credentialscope")
	}
}

func TestCanonicalRequest(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://host.foo.com/%20/foo", nil)
	r.Header.Add("date", "Mon, 09 Sep 2011 23:36:00 GMT")
	v, _ := CanonicalRequest(r)
	expected := `GET
/%20/foo/

date:Mon, 09 Sep 2011 23:36:00 GMT
host:host.foo.com

date;host
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
	if v != expected {
		t.Fatal("wrong canonicalrequest")
	}
}

func TestStringToSign(t *testing.T) {
	s := Signer{
		AccessKey: "AKIDEXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		Region:    "cn-north-1",
		Service:   "host",
	}
	r, _ := http.NewRequest("GET", "http://host.foo.com/%20/foo", nil)
	r.Header.Add("date", "Mon, 09 Sep 2011 23:36:00 GMT")
	canonicalRequest, _ := CanonicalRequest(r)
	tt, _ := time.Parse(time.RFC1123, "Mon, 09 Sep 2011 23:36:00 GMT")
	credentialScope := CredentialScope(tt, s.Region, s.Service)
	stringToSign := StringToSign(canonicalRequest, credentialScope, tt)
	if stringToSign != `SDK-HMAC-SHA256
20110909T233600Z
20110909/cn-north-1/host/sdk_request
69c45fb9fe3fd76442b5086e50b2e9fec8298358da957b293ef26e506fdfb54b` {
		fmt.Println(stringToSign)
	}
}

func TestAuthHeader(t *testing.T) {
	s := Signer{
		AccessKey: "AKIDEXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		Region:    "cn-north-1",
		Service:   "host",
	}
	r, _ := http.NewRequest("GET", "http://host.foo.com/%20/foo", nil)
	r.Header.Add("date", "Mon, 09 Sep 2011 23:36:00 GMT")
	s.Sign(r)
	if r.Header.Get("authorization") != `SDK-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/cn-north-1/host/sdk_request, SignedHeaders=date;host, Signature=acc863b58ef92620f9c63ea64c1e01dec2c46a1d8333066767e1f204c71832f2` {
		t.Fatal(r.Header.Get("authorization"), "miss match")
	}
}

func TestPostHeader(t *testing.T) {
	s := Signer{
		AccessKey: "AKIDEXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		Region:    "cn-north-1",
		Service:   "host",
	}
	r, _ := http.NewRequest("POST", "http://host.foo.com/", ioutil.NopCloser(bytes.NewBuffer([]byte("foo=bar"))))
	r.Header.Add("date", "Mon, 09 Sep 2011 23:36:00 GMT")
	r.Header.Add("content-type", "application/x-www-form-urlencoded; charset=utf8")
	s.Sign(r)
	if r.Header.Get("authorization") != `SDK-HMAC-SHA256 Credential=AKIDEXAMPLE/20110909/cn-north-1/host/sdk_request, SignedHeaders=content-type;date;host, Signature=599f64eab9b3ad635fa19f9927d04f409efba257cd42782791cca7be1b885172` {
		t.Fatal(r.Header.Get("authorization"), "miss match")
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Fatal("http body error")
	}
	if string(b) != "foo=bar" {
		t.Fatal("wrong body")
	}
}
