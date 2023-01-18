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
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/sdkerr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	DefaultInitDelay = 2 * time.Second
	DefaultFactor    = 1.02
	DefaultSteps     = 30
)

func IsNotFound(err error) bool {
	if status.Code(err) == codes.NotFound {
		return true
	}
	if e, ok := err.(sdkerr.ServiceResponseError); ok {
		return e.StatusCode == 404
	}
	if e, ok := err.(*sdkerr.ServiceResponseError); ok {
		return e.StatusCode == 404
	}
	return false
}

// WaitForCompleted wait for completion, interval 2s+, up to 30 pols
func WaitForCompleted(condition wait.ConditionFunc) error {
	backoff := wait.Backoff{
		Duration: DefaultInitDelay,
		Factor:   DefaultFactor,
		Steps:    DefaultSteps,
	}
	return wait.ExponentialBackoff(backoff, condition)
}
