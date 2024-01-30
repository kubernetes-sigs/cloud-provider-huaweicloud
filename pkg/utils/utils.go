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

package utils

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"k8s.io/klog"
)

// IsStrSliceContains searches if a string list contains the given string or not.
func IsStrSliceContains(list []string, strToSearch string) bool {
	for _, item := range list {
		if item == strToSearch {
			return true
		}
	}
	return false
}

func CutString(original string, length int) string {
	rst := original
	if len(original) > length {
		rst = original[:length]
	}
	return rst
}

func ToString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case *string:
		if v == nil {
			return ""
		}
		return *v
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%#v", val)
		}
		return string(b)
	}
}

func LookupHost(domain string) []string {
	ns, err := net.LookupHost(domain)
	if err != nil {
		klog.Warningf("failed to looks up the given host using the local resolver: %s", err)
		return nil
	}

	klog.Infof("lookup host %s: %s", domain, strings.Join(ns, ", "))
	return ns
}
