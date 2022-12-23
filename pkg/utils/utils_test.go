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

package utils

import (
	"testing"
)

func TestIsStrSliceContains(t *testing.T) {
	data := []string{"a", "b", "c", "abc"}

	tests := []struct {
		name     string
		data     []string
		val      string
		expected bool
	}{
		{
			name:     "test1",
			data:     data,
			val:      "a",
			expected: true,
		},
		{
			name:     "test1",
			data:     data,
			val:      "abc",
			expected: true,
		},
		{
			name:     "test1",
			data:     data,
			val:      "ab",
			expected: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			isContain := IsStrSliceContains(testCase.data, testCase.val)
			if isContain != testCase.expected {
				t.Fatalf("expected: %v, got : %v", testCase.expected, isContain)
			}
		})
	}
}
