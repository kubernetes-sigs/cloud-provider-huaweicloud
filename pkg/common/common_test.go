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

package common

import (
	"fmt"
	"testing"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "test1",
			err:      status.Error(codes.NotFound, "not found"),
			expected: true,
		},
		{
			name:     "test2",
			err:      status.Error(codes.Internal, "Internal"),
			expected: false,
		},
		{
			name:     "test3",
			err:      status.Error(codes.Unavailable, "Unavailable"),
			expected: false,
		},
		{
			name:     "test4",
			err:      sdkerr.ServiceResponseError{StatusCode: 404},
			expected: true,
		},
		{
			name:     "test5",
			err:      sdkerr.ServiceResponseError{StatusCode: 200},
			expected: false,
		},
		{
			name:     "test6",
			err:      fmt.Errorf("404 not found"),
			expected: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			b := IsNotFound(testCase.err)
			if b != testCase.expected {
				t.Fatalf("expected: %v, got : %v", testCase.expected, b)
			}
		})
	}
}

func TestWaitForCompleted(t *testing.T) {
	count := 0
	tests := []struct {
		name      string
		condition wait.ConditionFunc
		hasErr    bool
	}{
		{
			name: "test1",
			condition: func() (done bool, err error) {
				for {
					count++
					return count >= 3, nil
				}
			},
			hasErr: false,
		},
		{
			name: "test2",
			condition: func() (done bool, err error) {
				for {
					count++
					return count >= 3, fmt.Errorf("err")
				}
			},
			hasErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			hasErr := WaitForCompleted(testCase.condition) == nil
			if hasErr == testCase.hasErr {
				t.Fatalf("expected: %v, got : %v", testCase.hasErr, hasErr)
			}
		})
	}
}
