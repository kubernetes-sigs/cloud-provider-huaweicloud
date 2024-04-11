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
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/klog/v2"

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

type JobHandle func()
type ExecutePool struct {
	workerNum int
	queueCh   chan JobHandle
	stopCh    chan struct{}
}

func NewExecutePool(size int) *ExecutePool {
	return &ExecutePool{
		workerNum: size,
		queueCh:   make(chan JobHandle, 2000),
	}
}

func (w *ExecutePool) Start() {
	// Make sure it is not started repeatedly.
	if w.stopCh != nil {
		w.stopCh <- struct{}{}
		close(w.stopCh)
	}
	stopCh := make(chan struct{}, 1)
	w.stopCh = stopCh

	for i := 0; i < w.workerNum; i++ {
		klog.Infof("start goroutine pool handler: %v/%v", i, w.workerNum)
		go func() {
			for {
				select {
				case handler, ok := <-w.queueCh:
					if !ok {
						klog.Errorf("goroutine pool exiting")
						return
					}
					handler()
				case <-stopCh:
					klog.Info("goroutine pool stopping")
					return
				}
			}
		}()
	}

	go func() {
		exit := make(chan os.Signal, 1)
		signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
		<-exit
		w.Stop()
	}()
}

func (w *ExecutePool) Stop() {
	w.stopCh <- struct{}{}
	close(w.stopCh)
	close(w.queueCh)
}

func (w *ExecutePool) Submit(work JobHandle) {
	w.queueCh <- work
}
