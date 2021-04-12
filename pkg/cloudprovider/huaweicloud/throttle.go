/*
Copyright 2017 The Kubernetes Authors.

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

// nolint:golint // stop check lint issues as this file will be refactored
package huaweicloud

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"sync"

	"github.com/fsnotify/fsnotify"

	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog"
)

// TODO: 一代ELB和NAT网关API限流尚未支持

const (
	ThrottleConfigFile = "THROTTLE_CONFIG_FILE"

	/* 二代ELB限流环境变量配置 */
	MaxInstanceGetQPS      = "PUBLIC_ELB_INSTANCE_GET_MAX_QPS"
	MaxInstanceGetBurst    = "PUBLIC_ELB_INSTANCE_GET_MAX_BURST"
	MaxInstanceListQPS     = "PUBLIC_ELB_INSTANCE_LIST_MAX_QPS"
	MaxInstanceListBurst   = "PUBLIC_ELB_INSTANCE_LIST_MAX_BURST"
	MaxInstanceCreateQPS   = "PUBLIC_ELB_INSTANCE_CREATE_MAX_QPS"
	MaxInstanceCreateBurst = "PUBLIC_ELB_INSTANCE_CREATE_MAX_BURST"
	MaxInstanceDeleteQPS   = "PUBLIC_ELB_INSTANCE_DELETE_MAX_QPS"
	MaxInstanceDeleteBurst = "PUBLIC_ELB_INSTANCE_DELETE_MAX_BURST"

	MaxListenerGetQPS      = "PUBLIC_ELB_LISTENER_GET_MAX_QPS"
	MaxListenerGetBurst    = "PUBLIC_ELB_LISTENER_GET_MAX_BURST"
	MaxListenerListQPS     = "PUBLIC_ELB_LISTENER_LIST_MAX_QPS"
	MaxListenerListBurst   = "PUBLIC_ELB_LISTENER_LIST_MAX_BURST"
	MaxListenerCreateQPS   = "PUBLIC_ELB_LISTENER_CREATE_MAX_QPS"
	MaxListenerCreateBurst = "PUBLIC_ELB_LISTENER_CREATE_MAX_BURST"
	MaxListenerUpdateQPS   = "PUBLIC_ELB_LISTENER_UPDATE_MAX_QPS"
	MaxListenerUpdateBurst = "PUBLIC_ELB_LISTENER_UPDATE_MAX_BURST"
	MaxListenerDeleteQPS   = "PUBLIC_ELB_LISTENER_DELETE_MAX_QPS"
	MaxListenerDeleteBurst = "PUBLIC_ELB_LISTENER_DELETE_MAX_BURST"

	MaxPoolGetQPS      = "PUBLIC_ELB_POOL_GET_MAX_QPS"
	MaxPoolGetBurst    = "PUBLIC_ELB_POOL_GET_MAX_BURST"
	MaxPoolListQPS     = "PUBLIC_ELB_POOL_LIST_MAX_QPS"
	MaxPoolListBurst   = "PUBLIC_ELB_POOL_LIST_MAX_BURST"
	MaxPoolCreateQPS   = "PUBLIC_ELB_POOL_CREATE_MAX_QPS"
	MaxPoolCreateBurst = "PUBLIC_ELB_POOL_CREATE_MAX_BURST"
	MaxPoolUpdateQPS   = "PUBLIC_ELB_POOL_UPDATE_MAX_QPS"
	MaxPoolUpdateBurst = "PUBLIC_ELB_POOL_UPDATE_MAX_BURST"
	MaxPoolDeleteQPS   = "PUBLIC_ELB_POOL_DELETE_MAX_QPS"
	MaxPoolDeleteBurst = "PUBLIC_ELB_POOL_DELETE_MAX_BURST"

	MaxMemberGetQPS      = "PUBLIC_ELB_MEMBER_GET_MAX_QPS"
	MaxMemberGetBurst    = "PUBLIC_ELB_MEMBER_GET_MAX_BURST"
	MaxMemberListQPS     = "PUBLIC_ELB_MEMBER_LIST_MAX_QPS"
	MaxMemberListBurst   = "PUBLIC_ELB_MEMBER_LIST_MAX_BURST"
	MaxMemberCreateQPS   = "PUBLIC_ELB_MEMBER_CREATE_MAX_QPS"
	MaxMemberCreateBurst = "PUBLIC_ELB_MEMBER_CREATE_MAX_BURST"
	MaxMemberUpdateQPS   = "PUBLIC_ELB_MEMBER_UPDATE_MAX_QPS"
	MaxMemberUpdateBurst = "PUBLIC_ELB_MEMBER_UPDATE_MAX_BURST"
	MaxMemberDeleteQPS   = "PUBLIC_ELB_MEMBER_DELETE_MAX_QPS"
	MaxMemberDeleteBurst = "PUBLIC_ELB_MEMBER_DELETE_MAX_BURST"

	MaxHealthzGetQPS      = "PUBLIC_ELB_HEALTHZ_GET_MAX_QPS"
	MaxHealthzGetBurst    = "PUBLIC_ELB_HEALTHZ_GET_MAX_BURST"
	MaxHealthzListQPS     = "PUBLIC_ELB_HEALTHZ_LIST_MAX_QPS"
	MaxHealthzListBurst   = "PUBLIC_ELB_HEALTHZ_LIST_MAX_BURST"
	MaxHealthzCreateQPS   = "PUBLIC_ELB_HEALTHZ_CREATE_MAX_QPS"
	MaxHealthzCreateBurst = "PUBLIC_ELB_HEALTHZ_CREATE_MAX_BURST"
	MaxHealthzUpdateQPS   = "PUBLIC_ELB_HEALTHZ_UPDATE_MAX_QPS"
	MaxHealthzUpdateBurst = "PUBLIC_ELB_HEALTHZ_UPDATE_MAX_BURST"
	MaxHealthzDeleteQPS   = "PUBLIC_ELB_HEALTHZ_DELETE_MAX_QPS"
	MaxHealthzDeleteBurst = "PUBLIC_ELB_HEALTHZ_DELETE_MAX_BURST"
	/* 二代ELB限流环境变量配置 */

	/* NAT网关限流环境变量配置 */
	MaxNatGatewayGetQPS    = "NAT_GATEWAY_GET_MAX_QPS"
	MaxNatGatewayGetBurst  = "NAT_GATEWAY_GET_MAX_BURST"
	MaxNatGatewayListQPS   = "NAT_GATEWAY_LIST_MAX_QPS"
	MaxNatGatewayListBurst = "NAT_GATEWAY_LIST_MAX_BURST"

	MaxNatRuleGetQPS      = "NAT_RULE_GET_MAX_QPS"
	MaxNatRuleGetBurst    = "NAT_RULE_GET_MAX_BURST"
	MaxNatRuleListQPS     = "NAT_RULE_LIST_MAX_QPS"
	MaxNatRuleListBurst   = "NAT_RULE_LIST_MAX_BURST"
	MaxNatRuleCreateQPS   = "NAT_RULE_CREATE_MAX_QPS"
	MaxNatRuleCreateBurst = "NAT_RULE_CREATE_MAX_BURST"
	MaxNatRuleDeleteQPS   = "NAT_RULE_DELETE_MAX_QPS"
	MaxNatRuleDeleteBurst = "NAT_RULE_DELETE_MAX_BURST"
	/* NAT网关限流环境变量配置 */

	MaxEipBindQPS     = "EIP_BIND_MAX_QPS"
	MaxEipBindBurst   = "EIP_BIND_MAX_BURST"
	MaxEipCreateQPS   = "EIP_CREATE_MAX_QPS"
	MaxEipCreateBurst = "EIP_CREATE_MAX_BURST"
	MaxEipDeleteQPS   = "EIP_DELETE_MAX_QPS"
	MaxEipDeleteBurst = "EIP_DELETE_MAX_BURST"
	MaxEipListQPS     = "EIP_LIST_MAX_QPS"
	MaxEipListBurst   = "EIP_LIST_MAX_BURST"

	MaxSubnetGetQPS    = "SUBNET_GET_MAX_QPS"
	MaxSubnetGetBurst  = "SUBNET_GET_MAX_BURST"
	MaxSubnetListQPS   = "SUBNET_LIST_MAX_QPS"
	MaxSubnetListBurst = "SUBNET_LIST_MAX_BURST"
)

type ThrottleType string

const (
	/* 二代ELB限流器配置 */
	ELB_INSTANCE_GET    ThrottleType = "ELB_INSTANCE_GET"
	ELB_INSTANCE_LIST   ThrottleType = "ELB_INSTANCE_LIST"
	ELB_INSTANCE_CREATE ThrottleType = "ELB_INSTANCE_CREATE"
	ELB_INSTANCE_DELETE ThrottleType = "ELB_INSTANCE_DELETE"

	ELB_LISTENER_GET    ThrottleType = "ELB_LISTENER_GET"
	ELB_LISTENER_LIST   ThrottleType = "ELB_LISTENER_LIST"
	ELB_LISTENER_CREATE ThrottleType = "ELB_LISTENER_CREATE"
	ELB_LISTENER_UPDATE ThrottleType = "ELB_LISTENER_UPDATE"
	ELB_LISTENER_DELETE ThrottleType = "ELB_LISTENER_DELETE"

	ELB_POOL_GET    ThrottleType = "ELB_POOL_GET"
	ELB_POOL_LIST   ThrottleType = "ELB_POOL_LIST"
	ELB_POOL_CREATE ThrottleType = "ELB_POOL_CREATE"
	ELB_POOL_UPDATE ThrottleType = "ELB_POOL_UPDATE"
	ELB_POOL_DELETE ThrottleType = "ELB_POOL_DELETE"

	ELB_MEMBER_GET    ThrottleType = "ELB_MEMBER_GET"
	ELB_MEMBER_LIST   ThrottleType = "ELB_MEMBER_LIST"
	ELB_MEMBER_CREATE ThrottleType = "ELB_MEMBER_CREATE"
	ELB_MEMBER_UPDATE ThrottleType = "ELB_MEMBER_UPDATE"
	ELB_MEMBER_DELETE ThrottleType = "ELB_MEMBER_DELETE"

	ELB_WHITELIST_GET    ThrottleType = "ELB_WHITELIST_GET"
	ELB_WHITELIST_LIST   ThrottleType = "ELB_WHITELIST_LIST"
	ELB_WHITELIST_CREATE ThrottleType = "ELB_WHITELIST_CREATE"
	ELB_WHITELIST_UPDATE ThrottleType = "ELB_WHITELIST_UPDATE"
	ELB_WHITELIST_DELETE ThrottleType = "ELB_WHITELIST_DELETE"

	ELB_HEALTHZ_GET    ThrottleType = "ELB_HEALTHZ_GET"
	ELB_HEALTHZ_LIST   ThrottleType = "ELB_HEALTHZ_LIST"
	ELB_HEALTHZ_CREATE ThrottleType = "ELB_HEALTHZ_CREATE"
	ELB_HEALTHZ_UPDATE ThrottleType = "ELB_HEALTHZ_UPDATE"
	ELB_HEALTHZ_DELETE ThrottleType = "ELB_HEALTHZ_DELETE"
	/* 二代ELB限流器配置 */

	/* NAT网关限流器配置 */
	NAT_GATEWAY_GET  ThrottleType = "NAT_GATEWAY_GET"
	NAT_GATEWAY_LIST ThrottleType = "NAT_GATEWAY_LIST"

	NAT_RULE_GET    ThrottleType = "NAT_RULE_GET"
	NAT_RULE_LIST   ThrottleType = "NAT_RULE_LIST"
	NAT_RULE_CREATE ThrottleType = "NAT_RULE_CREATE"
	NAT_RULE_DELETE ThrottleType = "NAT_RULE_DELETE"
	/* NAT网关限流器配置 */

	EIP_BIND   ThrottleType = "EIP_BIND"
	EIP_CREATE ThrottleType = "EIP_CREATE"
	EIP_DELETE ThrottleType = "EIP_DELETE"
	EIP_LIST   ThrottleType = "EIP_LIST"

	SUBNET_GET  ThrottleType = "SUBNET_GET"
	SUBNET_LIST ThrottleType = "SUBNET_LIST"

	/* ECS限流器配置 */
	ECS_LIST ThrottleType = "ECS_LIST"
)

type Throttler struct {
	lock      sync.RWMutex
	throttles map[ThrottleType]flowcontrol.RateLimiter
}

type ThrottleConfig struct {
	Throttles map[string]ThrottleParam `json:"throttles"`
}

type ThrottleParam struct {
	QPS   float32 `json:"qps"`
	Burst int     `json:"burst"`
}

func InitialThrottler() (*Throttler, error) {
	t := &Throttler{
		throttles: make(map[ThrottleType]flowcontrol.RateLimiter),
	}

	err := t.getInstanceThrottle()
	if err != nil {
		return nil, err
	}

	err = t.getListenerThrottle()
	if err != nil {
		return nil, err
	}

	err = t.getPoolThrottle()
	if err != nil {
		return nil, err
	}

	err = t.getMemberThrottle()
	if err != nil {
		return nil, err
	}

	err = t.getHealthzThrottle()
	if err != nil {
		return nil, err
	}

	err = t.getNatGatewayThrottle()
	if err != nil {
		return nil, err
	}

	err = t.getNatRuleThrottle()
	if err != nil {
		return nil, err
	}

	err = t.getEipThrottle()
	if err != nil {
		return nil, err
	}

	err = t.getSubnetThrottle()
	if err != nil {
		return nil, err
	}

	go t.syncThrottleFromConfig()

	return t, nil
}

// sync throttle params from config file
func (t *Throttler) syncThrottleFromConfig() {
	conf := os.Getenv(ThrottleConfigFile)
	if conf == "" {
		conf = "/srv/throttle"
	}

	fi, err := os.Stat(conf)
	if err != nil && os.IsNotExist(err) {
		klog.V(2).Info("Throttle config file is not exist.")
		return
	}

	if !fi.IsDir() {
		conf, _ = path.Split(conf)
	}

	t.resetThrottles(conf)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		klog.Warningf("New file watcher error: %v", err)
		return
	}
	defer watcher.Close()

	err = watcher.Add(conf)
	if err != nil {
		klog.Errorf("Throttle watcher add error: %v", err)
		return
	}

	for {
		select {
		case event := <-watcher.Events:
			klog.V(4).Infof("Throttle watcher event: %v", event)
			if event.Op&fsnotify.Write == fsnotify.Write {
				// file has been modified
				klog.Infof("Throttle config file has been modified, will reset throttles.")
				t.resetThrottles(conf)
			}
		case err := <-watcher.Errors:
			klog.Warningf("Throttle watch error: %v", err)
		}
	}
}

func (t *Throttler) resetThrottles(conf string) {
	b, err := ioutil.ReadFile(path.Join(conf, "throttle.json"))
	if err != nil {
		klog.Errorf("Read throttle config file error: %v", err)
		return
	}

	var cfg ThrottleConfig
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		klog.Errorf("Json umarshal throttle config file error: %v", err)
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()
	for key, params := range cfg.Throttles {
		if params.QPS > float32(params.Burst) {
			klog.Warningf("Throttle key(%s)'s flow is not right, qps(%f) > burst(%d)", key, params.QPS, params.Burst)
			continue
		}

		klog.Infof("Throttle key(%s) with flow(%f/%d)", key, params.QPS, params.Burst)
		t.throttles[ThrottleType(key)] = flowcontrol.NewTokenBucketRateLimiter(params.QPS, params.Burst)
	}
}

func (t *Throttler) getInstanceThrottle() error {
	maxInstanceGetQPS := os.Getenv(MaxInstanceGetQPS)
	maxInstanceGetBurst := os.Getenv(MaxInstanceGetBurst)
	maxInstanceListQPS := os.Getenv(MaxInstanceListQPS)
	maxInstanceListBurst := os.Getenv(MaxInstanceListBurst)
	maxInstanceCreateQPS := os.Getenv(MaxInstanceCreateQPS)
	maxInstanceCreateBurst := os.Getenv(MaxInstanceCreateBurst)
	maxInstanceDeleteQPS := os.Getenv(MaxInstanceDeleteQPS)
	maxInstanceDeleteBurst := os.Getenv(MaxInstanceDeleteBurst)
	if maxInstanceGetQPS == "" || maxInstanceGetBurst == "" {
		maxInstanceGetQPS = "10.0"
		maxInstanceGetBurst = "15"
	}
	if maxInstanceListQPS == "" || maxInstanceListBurst == "" {
		maxInstanceListQPS = "10.0"
		maxInstanceListBurst = "15"
	}
	if maxInstanceCreateQPS == "" || maxInstanceCreateBurst == "" {
		maxInstanceCreateQPS = "1.5"
		maxInstanceCreateBurst = "2"
	}
	if maxInstanceDeleteQPS == "" || maxInstanceDeleteBurst == "" {
		maxInstanceDeleteQPS = "1.5"
		maxInstanceDeleteBurst = "2"
	}

	instanceGetQPS, err := strconv.ParseFloat(maxInstanceGetQPS, 32)
	if err != nil {
		return err
	}

	instanceGetBurst, err := strconv.Atoi(maxInstanceGetBurst)
	if err != nil {
		return err
	}

	if float32(instanceGetBurst) < float32(instanceGetQPS) {
		return fmt.Errorf("elb maxInstanceGetQPS(%s) should not be larger than maxInstanceGetBurst(%s)",
			maxInstanceGetQPS, maxInstanceGetBurst)
	}

	t.throttles[ELB_INSTANCE_GET] = flowcontrol.NewTokenBucketRateLimiter(float32(instanceGetQPS), instanceGetBurst)

	instanceListQPS, err := strconv.ParseFloat(maxInstanceListQPS, 32)
	if err != nil {
		return err
	}

	instanceListBurst, err := strconv.Atoi(maxInstanceListBurst)
	if err != nil {
		return err
	}

	if float32(instanceListBurst) < float32(instanceListQPS) {
		return fmt.Errorf("elb maxInstanceListQPS(%s) should not be larger than maxInstanceListBurst(%s)",
			maxInstanceListQPS, maxInstanceListBurst)
	}

	t.throttles[ELB_INSTANCE_LIST] = flowcontrol.NewTokenBucketRateLimiter(float32(instanceListQPS), instanceListBurst)

	instanceCreateQPS, err := strconv.ParseFloat(maxInstanceCreateQPS, 32)
	if err != nil {
		return err
	}

	instanceCreateBurst, err := strconv.Atoi(maxInstanceCreateBurst)
	if err != nil {
		return err
	}

	if float32(instanceCreateBurst) < float32(instanceCreateQPS) {
		return fmt.Errorf("elb maxInstanecCreateQPS(%s) should not be larger than maxInstanceCreateBurst(%s)",
			maxInstanceCreateQPS, maxInstanceCreateBurst)
	}

	t.throttles[ELB_INSTANCE_CREATE] = flowcontrol.NewTokenBucketRateLimiter(float32(instanceCreateQPS), instanceCreateBurst)

	instanceDeleteQPS, err := strconv.ParseFloat(maxInstanceDeleteQPS, 32)
	if err != nil {
		return err
	}

	instanceDeleteBurst, err := strconv.Atoi(maxInstanceDeleteBurst)
	if err != nil {
		return err
	}

	if float32(instanceDeleteBurst) < float32(instanceDeleteQPS) {
		return fmt.Errorf("elb maxInstanecDeleteQPS(%s) should not be larger than maxInstanceDeleteBurst(%s)",
			maxInstanceDeleteQPS, maxInstanceDeleteBurst)
	}

	t.throttles[ELB_INSTANCE_DELETE] = flowcontrol.NewTokenBucketRateLimiter(float32(instanceDeleteQPS), instanceDeleteBurst)
	return nil
}

func (t *Throttler) getListenerThrottle() error {
	maxListenerGetQPS := os.Getenv(MaxListenerGetQPS)
	maxListenerGetBurst := os.Getenv(MaxListenerGetBurst)
	maxListenerListQPS := os.Getenv(MaxListenerListQPS)
	maxListenerListBurst := os.Getenv(MaxListenerListBurst)
	maxListenerCreateQPS := os.Getenv(MaxListenerCreateQPS)
	maxListenerCreateBurst := os.Getenv(MaxListenerCreateBurst)
	maxListenerUpdateQPS := os.Getenv(MaxListenerUpdateQPS)
	maxListenerUpdateBurst := os.Getenv(MaxListenerUpdateBurst)
	maxListenerDeleteQPS := os.Getenv(MaxListenerDeleteQPS)
	maxListenerDeleteBurst := os.Getenv(MaxListenerDeleteBurst)
	if maxListenerGetQPS == "" || maxListenerGetBurst == "" {
		maxListenerGetQPS = "10.0"
		maxListenerGetBurst = "15"
	}
	if maxListenerListQPS == "" || maxListenerListBurst == "" {
		maxListenerListQPS = "10.0"
		maxListenerListBurst = "15"
	}
	if maxListenerCreateQPS == "" || maxListenerCreateBurst == "" {
		maxListenerCreateQPS = "1.5"
		maxListenerCreateBurst = "2"
	}
	if maxListenerUpdateQPS == "" || maxListenerUpdateBurst == "" {
		maxListenerUpdateQPS = "1.5"
		maxListenerUpdateBurst = "2"
	}
	if maxListenerDeleteQPS == "" || maxListenerDeleteBurst == "" {
		maxListenerDeleteQPS = "1.5"
		maxListenerDeleteBurst = "2"
	}

	listenerGetQPS, err := strconv.ParseFloat(maxListenerGetQPS, 32)
	if err != nil {
		return err
	}

	listenerGetBurst, err := strconv.Atoi(maxListenerGetBurst)
	if err != nil {
		return err
	}

	if float32(listenerGetBurst) < float32(listenerGetQPS) {
		return fmt.Errorf("elb maxListenerGetQPS(%s) should not be larger than maxListenerGetBurst(%s)",
			maxListenerGetQPS, maxListenerGetBurst)
	}

	t.throttles[ELB_LISTENER_GET] = flowcontrol.NewTokenBucketRateLimiter(float32(listenerGetQPS), listenerGetBurst)

	listenerListQPS, err := strconv.ParseFloat(maxListenerListQPS, 32)
	if err != nil {
		return err
	}

	listenerListBurst, err := strconv.Atoi(maxListenerListBurst)
	if err != nil {
		return err
	}

	if float32(listenerListBurst) < float32(listenerListQPS) {
		return fmt.Errorf("elb maxListenerListQPS(%s) should not be larger than maxListenerListBurst(%s)",
			maxListenerListQPS, maxListenerListBurst)
	}

	t.throttles[ELB_LISTENER_LIST] = flowcontrol.NewTokenBucketRateLimiter(float32(listenerListQPS), listenerListBurst)

	listenerCreateQPS, err := strconv.ParseFloat(maxListenerCreateQPS, 32)
	if err != nil {
		return err
	}

	listenerCreateBurst, err := strconv.Atoi(maxListenerCreateBurst)
	if err != nil {
		return err
	}

	if float32(listenerCreateBurst) < float32(listenerCreateQPS) {
		return fmt.Errorf("elb maxListenerCreateQPS(%s) should not be larger than maxListenerCreateBurst(%s)",
			maxListenerCreateQPS, maxListenerCreateBurst)
	}

	t.throttles[ELB_LISTENER_CREATE] = flowcontrol.NewTokenBucketRateLimiter(float32(listenerCreateQPS), listenerCreateBurst)

	listenerUpdateQPS, err := strconv.ParseFloat(maxListenerUpdateQPS, 32)
	if err != nil {
		return err
	}

	listenerUpdateBurst, err := strconv.Atoi(maxListenerUpdateBurst)
	if err != nil {
		return err
	}

	if float32(listenerUpdateBurst) < float32(listenerUpdateQPS) {
		return fmt.Errorf("elb maxListenerUpdateQPS(%s) should not be larger than maxListenerUpdateBurst(%s)",
			maxListenerUpdateQPS, maxListenerUpdateBurst)
	}

	t.throttles[ELB_LISTENER_UPDATE] = flowcontrol.NewTokenBucketRateLimiter(float32(listenerUpdateQPS), listenerUpdateBurst)

	listenerDeleteQPS, err := strconv.ParseFloat(maxListenerDeleteQPS, 32)
	if err != nil {
		return err
	}

	listenerDeleteBurst, err := strconv.Atoi(maxListenerDeleteBurst)
	if err != nil {
		return err
	}

	if float32(listenerDeleteBurst) < float32(listenerDeleteQPS) {
		return fmt.Errorf("elb maxListenerDeleteQPS(%s) should not be larger than maxListenerDeleteBurst(%s)",
			maxListenerDeleteQPS, maxListenerDeleteBurst)
	}

	t.throttles[ELB_LISTENER_DELETE] = flowcontrol.NewTokenBucketRateLimiter(float32(listenerDeleteQPS), listenerDeleteBurst)

	return nil
}

func (t *Throttler) getPoolThrottle() error {
	maxPoolGetQPS := os.Getenv(MaxPoolGetQPS)
	maxPoolGetBurst := os.Getenv(MaxPoolGetBurst)
	maxPoolListQPS := os.Getenv(MaxPoolListQPS)
	maxPoolListBurst := os.Getenv(MaxPoolListBurst)
	maxPoolCreateQPS := os.Getenv(MaxPoolCreateQPS)
	maxPoolCreateBurst := os.Getenv(MaxPoolCreateBurst)
	maxPoolUpdateQPS := os.Getenv(MaxPoolUpdateQPS)
	maxPoolUpdateBurst := os.Getenv(MaxPoolUpdateBurst)
	maxPoolDeleteQPS := os.Getenv(MaxPoolDeleteQPS)
	maxPoolDeleteBurst := os.Getenv(MaxPoolDeleteBurst)
	if maxPoolGetQPS == "" || maxPoolGetBurst == "" {
		maxPoolGetQPS = "10.0"
		maxPoolGetBurst = "15"
	}
	if maxPoolListQPS == "" || maxPoolListBurst == "" {
		maxPoolListQPS = "10.0"
		maxPoolListBurst = "15"
	}
	if maxPoolCreateQPS == "" || maxPoolCreateBurst == "" {
		maxPoolCreateQPS = "1.5"
		maxPoolCreateBurst = "2"
	}
	if maxPoolUpdateQPS == "" || maxPoolUpdateBurst == "" {
		maxPoolUpdateQPS = "1.5"
		maxPoolUpdateBurst = "2"
	}
	if maxPoolDeleteQPS == "" || maxPoolDeleteBurst == "" {
		maxPoolDeleteQPS = "1.5"
		maxPoolDeleteBurst = "2"
	}

	poolGetQPS, err := strconv.ParseFloat(maxPoolGetQPS, 32)
	if err != nil {
		return err
	}

	poolGetBurst, err := strconv.Atoi(maxPoolGetBurst)
	if err != nil {
		return err
	}

	if float32(poolGetBurst) < float32(poolGetQPS) {
		return fmt.Errorf("elb maxPoolGetQPS(%s) should not be larger than maxPoolGetBurst(%s)",
			maxPoolGetQPS, maxPoolGetBurst)
	}

	t.throttles[ELB_POOL_GET] = flowcontrol.NewTokenBucketRateLimiter(float32(poolGetQPS), poolGetBurst)

	poolListQPS, err := strconv.ParseFloat(maxPoolListQPS, 32)
	if err != nil {
		return err
	}

	poolListBurst, err := strconv.Atoi(maxPoolListBurst)
	if err != nil {
		return err
	}

	if float32(poolListBurst) < float32(poolListQPS) {
		return fmt.Errorf("elb maxPoolListQPS(%s) should not be larger than maxPoolListBurst(%s)",
			maxPoolListQPS, maxPoolListBurst)
	}

	t.throttles[ELB_POOL_LIST] = flowcontrol.NewTokenBucketRateLimiter(float32(poolListQPS), poolListBurst)

	poolCreateQPS, err := strconv.ParseFloat(maxPoolCreateQPS, 32)
	if err != nil {
		return err
	}

	poolCreateBurst, err := strconv.Atoi(maxPoolCreateBurst)
	if err != nil {
		return err
	}

	if float32(poolCreateBurst) < float32(poolCreateQPS) {
		return fmt.Errorf("elb maxPoolCreateQPS(%s) should not be larger than maxPoolCreateBurst(%s)",
			maxPoolCreateQPS, maxPoolCreateBurst)
	}

	t.throttles[ELB_POOL_CREATE] = flowcontrol.NewTokenBucketRateLimiter(float32(poolCreateQPS), poolCreateBurst)

	poolUpdateQPS, err := strconv.ParseFloat(maxPoolUpdateQPS, 32)
	if err != nil {
		return err
	}

	poolUpdateBurst, err := strconv.Atoi(maxPoolUpdateBurst)
	if err != nil {
		return err
	}

	if float32(poolUpdateBurst) < float32(poolUpdateQPS) {
		return fmt.Errorf("elb maxPoolUpdateQPS(%s) should not be larger than maxPoolUpdateBurst(%s)",
			maxPoolUpdateQPS, maxPoolUpdateBurst)
	}

	t.throttles[ELB_POOL_UPDATE] = flowcontrol.NewTokenBucketRateLimiter(float32(poolUpdateQPS), poolUpdateBurst)

	poolDeleteQPS, err := strconv.ParseFloat(maxPoolDeleteQPS, 32)
	if err != nil {
		return err
	}

	poolDeleteBurst, err := strconv.Atoi(maxPoolDeleteBurst)
	if err != nil {
		return err
	}

	if float32(poolDeleteBurst) < float32(poolDeleteQPS) {
		return fmt.Errorf("elb maxPoolDeleteQPS(%s) should not be larger than maxPoolDeleteBurst(%s)",
			maxPoolDeleteQPS, maxPoolDeleteBurst)
	}

	t.throttles[ELB_POOL_DELETE] = flowcontrol.NewTokenBucketRateLimiter(float32(poolDeleteQPS), poolDeleteBurst)

	return nil
}

func (t *Throttler) getMemberThrottle() error {
	maxMemberGetQPS := os.Getenv(MaxMemberGetQPS)
	maxMemberGetBurst := os.Getenv(MaxMemberGetBurst)
	maxMemberListQPS := os.Getenv(MaxMemberListQPS)
	maxMemberListBurst := os.Getenv(MaxMemberListBurst)
	maxMemberCreateQPS := os.Getenv(MaxMemberCreateQPS)
	maxMemberCreateBurst := os.Getenv(MaxMemberCreateBurst)
	maxMemberUpdateQPS := os.Getenv(MaxMemberUpdateQPS)
	maxMemberUpdateBurst := os.Getenv(MaxMemberUpdateBurst)
	maxMemberDeleteQPS := os.Getenv(MaxMemberDeleteQPS)
	maxMemberDeleteBurst := os.Getenv(MaxMemberDeleteBurst)
	if maxMemberGetQPS == "" || maxMemberGetBurst == "" {
		maxMemberGetQPS = "10.0"
		maxMemberGetBurst = "15"
	}
	if maxMemberListQPS == "" || maxMemberListBurst == "" {
		maxMemberListQPS = "10.0"
		maxMemberListBurst = "15"
	}
	if maxMemberCreateQPS == "" || maxMemberCreateBurst == "" {
		maxMemberCreateQPS = "3.0"
		maxMemberCreateBurst = "4"
	}
	if maxMemberUpdateQPS == "" || maxMemberUpdateBurst == "" {
		maxMemberUpdateQPS = "3.0"
		maxMemberUpdateBurst = "4"
	}
	if maxMemberDeleteQPS == "" || maxMemberDeleteBurst == "" {
		maxMemberDeleteQPS = "3.0"
		maxMemberDeleteBurst = "4"
	}

	memberGetQPS, err := strconv.ParseFloat(maxMemberGetQPS, 32)
	if err != nil {
		return err
	}

	memberGetBurst, err := strconv.Atoi(maxMemberGetBurst)
	if err != nil {
		return err
	}

	if float32(memberGetBurst) < float32(memberGetQPS) {
		return fmt.Errorf("elb maxMemberGetQPS(%s) should not be larger than maxMemberGetBurst(%s)",
			maxMemberGetQPS, maxMemberGetBurst)
	}

	t.throttles[ELB_MEMBER_GET] = flowcontrol.NewTokenBucketRateLimiter(float32(memberGetQPS), memberGetBurst)

	memberListQPS, err := strconv.ParseFloat(maxMemberListQPS, 32)
	if err != nil {
		return err
	}

	memberListBurst, err := strconv.Atoi(maxMemberListBurst)
	if err != nil {
		return err
	}

	if float32(memberListBurst) < float32(memberListQPS) {
		return fmt.Errorf("elb maxMemberListQPS(%s) should not be larger than maxMemberListBurst(%s)",
			maxMemberListQPS, maxMemberListBurst)
	}

	t.throttles[ELB_MEMBER_LIST] = flowcontrol.NewTokenBucketRateLimiter(float32(memberListQPS), memberListBurst)

	memberCreateQPS, err := strconv.ParseFloat(maxMemberCreateQPS, 32)
	if err != nil {
		return err
	}

	memberCreateBurst, err := strconv.Atoi(maxMemberCreateBurst)
	if err != nil {
		return err
	}

	if float32(memberCreateBurst) < float32(memberCreateQPS) {
		return fmt.Errorf("elb maxMemberCreateQPS(%s) should not be larger than maxMemberCreateBurst(%s)",
			maxMemberCreateQPS, maxMemberCreateBurst)
	}

	t.throttles[ELB_MEMBER_CREATE] = flowcontrol.NewTokenBucketRateLimiter(float32(memberCreateQPS), memberCreateBurst)

	memberUpdateQPS, err := strconv.ParseFloat(maxMemberUpdateQPS, 32)
	if err != nil {
		return err
	}

	memberUpdateBurst, err := strconv.Atoi(maxMemberUpdateBurst)
	if err != nil {
		return err
	}

	if float32(memberUpdateBurst) < float32(memberUpdateQPS) {
		return fmt.Errorf("elb maxMemberUpdateQPS(%s) should not be larger than maxMemberUpdateBurst(%s)",
			maxMemberUpdateQPS, maxMemberUpdateBurst)
	}

	t.throttles[ELB_MEMBER_UPDATE] = flowcontrol.NewTokenBucketRateLimiter(float32(memberUpdateQPS), memberUpdateBurst)

	memberDeleteQPS, err := strconv.ParseFloat(maxMemberDeleteQPS, 32)
	if err != nil {
		return err
	}

	memberDeleteBurst, err := strconv.Atoi(maxMemberDeleteBurst)
	if err != nil {
		return err
	}

	if float32(memberDeleteBurst) < float32(memberDeleteQPS) {
		return fmt.Errorf("elb maxMemberDeleteQPS(%s) should not be larger than maxMemberDeleteBurst(%s)",
			maxMemberDeleteQPS, maxMemberDeleteBurst)
	}

	t.throttles[ELB_MEMBER_DELETE] = flowcontrol.NewTokenBucketRateLimiter(float32(memberDeleteQPS), memberDeleteBurst)

	return nil
}

func (t *Throttler) getHealthzThrottle() error {
	maxHealthzGetQPS := os.Getenv(MaxHealthzGetQPS)
	maxHealthzGetBurst := os.Getenv(MaxHealthzGetBurst)
	maxHealthzListQPS := os.Getenv(MaxHealthzListQPS)
	maxHealthzListBurst := os.Getenv(MaxHealthzListBurst)
	maxHealthzCreateQPS := os.Getenv(MaxHealthzCreateQPS)
	maxHealthzCreateBurst := os.Getenv(MaxHealthzCreateBurst)
	maxHealthzUpdateQPS := os.Getenv(MaxHealthzUpdateQPS)
	maxHealthzUpdateBurst := os.Getenv(MaxHealthzUpdateBurst)
	maxHealthzDeleteQPS := os.Getenv(MaxHealthzDeleteQPS)
	maxHealthzDeleteBurst := os.Getenv(MaxHealthzDeleteBurst)
	if maxHealthzGetQPS == "" || maxHealthzGetBurst == "" {
		maxHealthzGetQPS = "10.0"
		maxHealthzGetBurst = "15"
	}
	if maxHealthzListQPS == "" || maxHealthzListBurst == "" {
		maxHealthzListQPS = "10.0"
		maxHealthzListBurst = "15"
	}
	if maxHealthzCreateQPS == "" || maxHealthzCreateBurst == "" {
		maxHealthzCreateQPS = "1.5"
		maxHealthzCreateBurst = "2"
	}
	if maxHealthzUpdateQPS == "" || maxHealthzUpdateBurst == "" {
		maxHealthzUpdateQPS = "1.5"
		maxHealthzUpdateBurst = "2"
	}
	if maxHealthzDeleteQPS == "" || maxHealthzDeleteBurst == "" {
		maxHealthzDeleteQPS = "1.5"
		maxHealthzDeleteBurst = "2"
	}

	healthzGetQPS, err := strconv.ParseFloat(maxHealthzGetQPS, 32)
	if err != nil {
		return err
	}

	healthzGetBurst, err := strconv.Atoi(maxHealthzGetBurst)
	if err != nil {
		return err
	}

	if float32(healthzGetBurst) < float32(healthzGetQPS) {
		return fmt.Errorf("elb maxHealthzGetQPS(%s) should not be larger than maxHealthzGetBurst(%s)",
			maxHealthzGetQPS, maxHealthzGetBurst)
	}

	t.throttles[ELB_HEALTHZ_GET] = flowcontrol.NewTokenBucketRateLimiter(float32(healthzGetQPS), healthzGetBurst)

	healthzListQPS, err := strconv.ParseFloat(maxHealthzListQPS, 32)
	if err != nil {
		return err
	}

	healthzListBurst, err := strconv.Atoi(maxHealthzListBurst)
	if err != nil {
		return err
	}

	if float32(healthzListBurst) < float32(healthzListQPS) {
		return fmt.Errorf("elb maxHealthzListQPS(%s) should not be larger than maxHealthzListBurst(%s)",
			maxHealthzListQPS, maxHealthzListBurst)
	}

	t.throttles[ELB_HEALTHZ_LIST] = flowcontrol.NewTokenBucketRateLimiter(float32(healthzListQPS), healthzListBurst)

	healthzCreateQPS, err := strconv.ParseFloat(maxHealthzCreateQPS, 32)
	if err != nil {
		return err
	}

	healthzCreateBurst, err := strconv.Atoi(maxHealthzCreateBurst)
	if err != nil {
		return err
	}

	if float32(healthzCreateBurst) < float32(healthzCreateQPS) {
		return fmt.Errorf("elb maxHealthzCreateQPS(%s) should not be larger than maxHealthzCreateBurst(%s)",
			maxHealthzCreateQPS, maxHealthzCreateBurst)
	}

	t.throttles[ELB_HEALTHZ_CREATE] = flowcontrol.NewTokenBucketRateLimiter(float32(healthzCreateQPS), healthzCreateBurst)

	healthzUpdateQPS, err := strconv.ParseFloat(maxHealthzUpdateQPS, 32)
	if err != nil {
		return err
	}

	healthzUpdateBurst, err := strconv.Atoi(maxHealthzUpdateBurst)
	if err != nil {
		return err
	}

	if float32(healthzUpdateBurst) < float32(healthzUpdateQPS) {
		return fmt.Errorf("elb maxHealthzUpdateQPS(%s) should not be larger than maxHealthzUpdateBurst(%s)",
			maxHealthzUpdateQPS, maxHealthzUpdateBurst)
	}

	t.throttles[ELB_HEALTHZ_UPDATE] = flowcontrol.NewTokenBucketRateLimiter(float32(healthzUpdateQPS), healthzUpdateBurst)

	healthzDeleteQPS, err := strconv.ParseFloat(maxHealthzDeleteQPS, 32)
	if err != nil {
		return err
	}

	healthzDeleteBurst, err := strconv.Atoi(maxHealthzDeleteBurst)
	if err != nil {
		return err
	}

	if float32(healthzDeleteBurst) < float32(healthzDeleteQPS) {
		return fmt.Errorf("elb maxHealthzDeleteQPS(%s) should not be larger than maxHealthzDeleteBurst(%s)",
			maxHealthzDeleteQPS, maxHealthzDeleteBurst)
	}

	t.throttles[ELB_HEALTHZ_DELETE] = flowcontrol.NewTokenBucketRateLimiter(float32(healthzDeleteQPS), healthzDeleteBurst)

	return nil
}

func (t *Throttler) getNatGatewayThrottle() error {
	maxNatGatewayGetQPS := os.Getenv(MaxNatGatewayGetQPS)
	maxNatGatewayGetBurst := os.Getenv(MaxNatGatewayGetBurst)
	maxNatGatewayListQPS := os.Getenv(MaxNatGatewayListQPS)
	maxNatGatewayListBurst := os.Getenv(MaxNatGatewayListBurst)
	if maxNatGatewayGetQPS == "" || maxNatGatewayGetBurst == "" {
		maxNatGatewayGetQPS = "8.0"
		maxNatGatewayGetBurst = "10"
	}
	if maxNatGatewayListQPS == "" || maxNatGatewayListBurst == "" {
		maxNatGatewayListQPS = "8.0"
		maxNatGatewayListBurst = "10"
	}

	natGatewayGetQPS, err := strconv.ParseFloat(maxNatGatewayGetQPS, 32)
	if err != nil {
		return err
	}

	natGatewayGetBurst, err := strconv.Atoi(maxNatGatewayGetBurst)
	if err != nil {
		return err
	}

	if float32(natGatewayGetBurst) < float32(natGatewayGetQPS) {
		return fmt.Errorf("elb maxNatGatewayGetQPS(%s) should not be larger than maxNatGatewayGetBurst(%s)",
			maxNatGatewayGetQPS, maxNatGatewayGetBurst)
	}

	t.throttles[NAT_GATEWAY_GET] = flowcontrol.NewTokenBucketRateLimiter(float32(natGatewayGetQPS), natGatewayGetBurst)

	natGatewayListQPS, err := strconv.ParseFloat(maxNatGatewayListQPS, 32)
	if err != nil {
		return err
	}

	natGatewayListBurst, err := strconv.Atoi(maxNatGatewayListBurst)
	if err != nil {
		return err
	}

	if float32(natGatewayListBurst) < float32(natGatewayListQPS) {
		return fmt.Errorf("elb maxNatGatewayListQPS(%s) should not be larger than maxNatGatewayListBurst(%s)",
			maxNatGatewayListQPS, maxNatGatewayListBurst)
	}

	t.throttles[NAT_GATEWAY_LIST] = flowcontrol.NewTokenBucketRateLimiter(float32(natGatewayListQPS), natGatewayListBurst)
	return nil
}

func (t *Throttler) getNatRuleThrottle() error {
	maxNatRuleGetQPS := os.Getenv(MaxNatRuleGetQPS)
	maxNatRuleGetBurst := os.Getenv(MaxNatRuleGetBurst)
	maxNatRuleListQPS := os.Getenv(MaxNatRuleListQPS)
	maxNatRuleListBurst := os.Getenv(MaxNatRuleListBurst)
	maxNatRuleCreateQPS := os.Getenv(MaxNatRuleCreateQPS)
	maxNatRuleCreateBurst := os.Getenv(MaxNatRuleCreateBurst)
	maxNatRuleDeleteQPS := os.Getenv(MaxNatRuleDeleteQPS)
	maxNatRuleDeleteBurst := os.Getenv(MaxNatRuleDeleteBurst)
	if maxNatRuleGetQPS == "" || maxNatRuleGetBurst == "" {
		maxNatRuleGetQPS = "8.0"
		maxNatRuleGetBurst = "10"
	}
	if maxNatRuleListQPS == "" || maxNatRuleListBurst == "" {
		maxNatRuleListQPS = "8.0"
		maxNatRuleListBurst = "10"
	}
	if maxNatRuleCreateQPS == "" || maxNatRuleCreateBurst == "" {
		maxNatRuleCreateQPS = "0.8"
		maxNatRuleCreateBurst = "1"
	}
	if maxNatRuleDeleteQPS == "" || maxNatRuleDeleteBurst == "" {
		maxNatRuleDeleteQPS = "0.8"
		maxNatRuleDeleteBurst = "1"
	}

	natRuleGetQPS, err := strconv.ParseFloat(maxNatRuleGetQPS, 32)
	if err != nil {
		return err
	}

	natRuleGetBurst, err := strconv.Atoi(maxNatRuleGetBurst)
	if err != nil {
		return err
	}

	if float32(natRuleGetBurst) < float32(natRuleGetQPS) {
		return fmt.Errorf("elb maxNatRuleGetQPS(%s) should not be larger than maxNatRuleGetBurst(%s)",
			maxNatRuleGetQPS, maxNatRuleGetBurst)
	}

	t.throttles[NAT_RULE_GET] = flowcontrol.NewTokenBucketRateLimiter(float32(natRuleGetQPS), natRuleGetBurst)

	natRuleListQPS, err := strconv.ParseFloat(maxNatRuleListQPS, 32)
	if err != nil {
		return err
	}

	natRuleListBurst, err := strconv.Atoi(maxNatRuleListBurst)
	if err != nil {
		return err
	}

	if float32(natRuleListBurst) < float32(natRuleListQPS) {
		return fmt.Errorf("elb maxNatRuleListQPS(%s) should not be larger than maxNatRuleListBurst(%s)",
			maxNatRuleListQPS, maxNatRuleListBurst)
	}

	t.throttles[NAT_RULE_LIST] = flowcontrol.NewTokenBucketRateLimiter(float32(natRuleListQPS), natRuleListBurst)

	natRuleCreateQPS, err := strconv.ParseFloat(maxNatRuleCreateQPS, 32)
	if err != nil {
		return err
	}

	natRuleCreateBurst, err := strconv.Atoi(maxNatRuleCreateBurst)
	if err != nil {
		return err
	}

	if float32(natRuleCreateBurst) < float32(natRuleCreateQPS) {
		return fmt.Errorf("elb maxNatRuleCreateQPS(%s) should not be larger than maxNatRuleCreateBurst(%s)",
			maxNatRuleCreateQPS, maxNatRuleCreateBurst)
	}

	t.throttles[NAT_RULE_CREATE] = flowcontrol.NewTokenBucketRateLimiter(float32(natRuleCreateQPS), natRuleCreateBurst)

	natRuleDeleteQPS, err := strconv.ParseFloat(maxNatRuleDeleteQPS, 32)
	if err != nil {
		return err
	}

	natRuleDeleteBurst, err := strconv.Atoi(maxNatRuleDeleteBurst)
	if err != nil {
		return err
	}

	if float32(natRuleDeleteBurst) < float32(natRuleDeleteQPS) {
		return fmt.Errorf("elb maxNatRuleDeleteQPS(%s) should not be larger than maxNatRuleDeleteBurst(%s)",
			maxNatRuleDeleteQPS, maxNatRuleDeleteBurst)
	}

	t.throttles[NAT_RULE_DELETE] = flowcontrol.NewTokenBucketRateLimiter(float32(natRuleDeleteQPS), natRuleDeleteBurst)

	return nil
}

func (t *Throttler) getEipThrottle() error {
	maxEipBindQPS := os.Getenv(MaxEipBindQPS)
	maxEipBindBurst := os.Getenv(MaxEipBindBurst)
	maxEipCreateQPS := os.Getenv(MaxEipCreateQPS)
	maxEipCreateBurst := os.Getenv(MaxEipCreateBurst)
	maxEipDeleteQPS := os.Getenv(MaxEipDeleteQPS)
	maxEipDeleteBurst := os.Getenv(MaxEipDeleteBurst)
	maxEipListQPS := os.Getenv(MaxEipListQPS)
	maxEipListBurst := os.Getenv(MaxEipListBurst)
	if maxEipBindQPS == "" || maxEipBindBurst == "" {
		maxEipBindQPS = "1.5"
		maxEipBindBurst = "2"
	}
	if maxEipCreateQPS == "" || maxEipCreateBurst == "" {
		maxEipCreateQPS = "1.5"
		maxEipCreateBurst = "2"
	}
	if maxEipDeleteQPS == "" || maxEipDeleteBurst == "" {
		maxEipDeleteQPS = "1.5"
		maxEipDeleteBurst = "2"
	}
	if maxEipListQPS == "" || maxEipListBurst == "" {
		maxEipListQPS = "6.0"
		maxEipListBurst = "9"
	}

	eipBindQPS, err := strconv.ParseFloat(maxEipBindQPS, 32)
	if err != nil {
		return err
	}

	eipBindBurst, err := strconv.Atoi(maxEipBindBurst)
	if err != nil {
		return err
	}

	if float32(eipBindBurst) < float32(eipBindQPS) {
		return fmt.Errorf("elb maxEipBindQPS(%s) should not be larger than maxEipBindBurst(%s)",
			maxEipBindQPS, maxEipBindBurst)
	}

	t.throttles[EIP_BIND] = flowcontrol.NewTokenBucketRateLimiter(float32(eipBindQPS), eipBindBurst)

	eipCreateQPS, err := strconv.ParseFloat(maxEipCreateQPS, 32)
	if err != nil {
		return err
	}

	eipCreateBurst, err := strconv.Atoi(maxEipCreateBurst)
	if err != nil {
		return err
	}

	if float32(eipCreateBurst) < float32(eipCreateQPS) {
		return fmt.Errorf("elb maxEipCreateQPS(%s) should not be larger than maxEipCreateBurst(%s)",
			maxEipCreateQPS, maxEipCreateBurst)
	}

	t.throttles[EIP_CREATE] = flowcontrol.NewTokenBucketRateLimiter(float32(eipCreateQPS), eipCreateBurst)

	eipDeleteQPS, err := strconv.ParseFloat(maxEipDeleteQPS, 32)
	if err != nil {
		return err
	}

	eipDeleteBurst, err := strconv.Atoi(maxEipDeleteBurst)
	if err != nil {
		return err
	}

	if float32(eipDeleteBurst) < float32(eipDeleteQPS) {
		return fmt.Errorf("elb maxEipDeleteQPS(%s) should not be larger than maxEipDeleteBurst(%s)",
			maxEipDeleteQPS, maxEipDeleteBurst)
	}

	t.throttles[EIP_DELETE] = flowcontrol.NewTokenBucketRateLimiter(float32(eipDeleteQPS), eipDeleteBurst)

	eipListQPS, err := strconv.ParseFloat(maxEipListQPS, 32)
	if err != nil {
		return err
	}

	eipListBurst, err := strconv.Atoi(maxEipListBurst)
	if err != nil {
		return err
	}

	if float32(eipListBurst) < float32(eipListQPS) {
		return fmt.Errorf("elb maxEipListQPS(%s) should not be larger than maxEipListBurst(%s)",
			maxEipListQPS, maxEipListBurst)
	}

	t.throttles[EIP_LIST] = flowcontrol.NewTokenBucketRateLimiter(float32(eipListQPS), eipListBurst)

	return nil
}

func (t *Throttler) getSubnetThrottle() error {
	maxSubnetGetQPS := os.Getenv(MaxSubnetGetQPS)
	maxSubnetGetBurst := os.Getenv(MaxSubnetGetBurst)
	maxSubnetListQPS := os.Getenv(MaxSubnetListQPS)
	maxSubnetListBurst := os.Getenv(MaxSubnetListBurst)
	if maxSubnetGetQPS == "" || maxSubnetGetBurst == "" {
		maxSubnetGetQPS = "10.0"
		maxSubnetGetBurst = "15"
	}
	if maxSubnetListQPS == "" || maxSubnetListBurst == "" {
		maxSubnetListQPS = "10.0"
		maxSubnetListBurst = "15"
	}

	subnetGetQPS, err := strconv.ParseFloat(maxSubnetGetQPS, 32)
	if err != nil {
		return err
	}

	subnetGetBurst, err := strconv.Atoi(maxSubnetGetBurst)
	if err != nil {
		return err
	}

	if float32(subnetGetBurst) < float32(subnetGetQPS) {
		return fmt.Errorf("elb maxSubnetGetQPS(%s) should not be larger than maxSubnetGetBurst(%s)",
			maxSubnetGetQPS, maxSubnetGetBurst)
	}

	t.throttles[SUBNET_GET] = flowcontrol.NewTokenBucketRateLimiter(float32(subnetGetQPS), subnetGetBurst)

	subnetListQPS, err := strconv.ParseFloat(maxSubnetListQPS, 32)
	if err != nil {
		return err
	}

	subnetListBurst, err := strconv.Atoi(maxSubnetListBurst)
	if err != nil {
		return err
	}

	if float32(subnetListBurst) < float32(subnetListQPS) {
		return fmt.Errorf("elb maxSubnetListQPS(%s) should not be larger than maxSubnetListBurst(%s)",
			maxSubnetListQPS, maxSubnetListBurst)
	}

	t.throttles[SUBNET_LIST] = flowcontrol.NewTokenBucketRateLimiter(float32(subnetListQPS), subnetListBurst)

	return nil
}

func (t *Throttler) GetThrottleByKey(key ThrottleType) flowcontrol.RateLimiter {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.throttles[key]
}
