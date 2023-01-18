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
			name:     "test2",
			data:     data,
			val:      "abc",
			expected: true,
		},
		{
			name:     "test3",
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

func TestCutString(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		length   int
		expected string
	}{
		{
			name:     "test1",
			origin:   "abcd",
			length:   3,
			expected: "abc",
		},
		{
			name:     "test2",
			origin:   "abcd",
			length:   4,
			expected: "abcd",
		},
		{
			name:     "test3",
			origin:   "abcd",
			length:   5,
			expected: "abcd",
		},
		{
			name:     "test4",
			origin:   "_12%&*()%$#@abcd123",
			length:   12,
			expected: "_12%&*()%$#@",
		},
		{
			name:     "test5",
			origin:   "",
			length:   12,
			expected: "",
		},
	}

	for _, te := range tests {
		t.Run(te.name, func(t *testing.T) {
			isContain := CutString(te.origin, te.length)
			if isContain != te.expected {
				t.Fatalf("expected: %v, got : %v", te.expected, isContain)
			}
		})
	}
}
