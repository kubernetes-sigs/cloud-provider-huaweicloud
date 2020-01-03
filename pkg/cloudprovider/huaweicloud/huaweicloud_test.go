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
	"os"
	"testing"
)

func TestReadConfig(t *testing.T) {
	var tests = []struct {
		name        string
		configFile  string
		expectError bool
	}{
		{
			name:        "valid config should success",
			configFile:  "./testdata/valid-cloud-provider.json",
			expectError: false,
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			fileHandler, err := os.Open(tc.configFile)
			if err != nil {
				t.Fatalf("Unexpected error happends when opening file(%s): %v", tc.configFile, err)
			}
			_, err = readConfig(fileHandler)
			if tc.expectError && err == nil {
				t.Fatalf("expect error but got none")
			}
			if !tc.expectError && err != nil {
				t.Fatalf("no expect error but got: %v", err)
			}
		})
	}
}
