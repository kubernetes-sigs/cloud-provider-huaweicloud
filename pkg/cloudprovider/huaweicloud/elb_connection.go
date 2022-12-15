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

// nolint:golint // stop check lint issues as this file will be refactored
package huaweicloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"k8s.io/klog"

	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/common"
)

const tryJobStatusTimes = 100

type DeleteFunc func(string) error
type DeleteFuncWithObj func(string, interface{}) error

type ELBType string

const (
	ELBTypeInternal ELBType = "Internal"
	ELBTypeExternal ELBType = "External"

	MemberNormal      = "NORMAL"
	MemberAbnormal    = "ABNORMAL"
	MemberUnavailable = "UNAVAILABLE"
)

const (
	QuotaTypeElb       = "elb"
	QuotaTypeListeners = "listeners"
)

type ELB struct {
	// create ELB parameters
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	VpcID        string  `json:"vpc_id"`
	Bandwidth    int     `json:"bandwidth"`
	Type         ELBType `json:"type"`
	AdminStateUp int     `json:"admin_state_up"`
}

type ElbDetail struct {
	ELB
	VipAddress     string `json:"vip_address"`
	CreateTime     string `json:"create_time"`
	Status         string `json:"status"`
	LoadbalancerId string `json:"id"`
	VipSubnetId    string `json:"vip_subnet_id,omitempty"`
}

type ElbList struct {
	InstanceNum   string      `json:"instance_num"`
	Loadbalancers []ElbDetail `json:"loadbalancers,omitempty"`
}

// TODO: unmarshal failed, if return instance_num=0
// body: {"loadbalancers":{},"instance_num":"0"}
type ElbNum struct {
	InstanceNum string `json:"instance_num"`
}

type ListenerModify struct {
	Name         string `json:"name,omitempty"`
	AdminStateUp bool   `json:"admin_state_up,omitempty"`
	Description  string `json:"description,omitempty"`
}

// Listener
type Listener struct {
	// create Listener parameters
	ListenerModify
	LoadbalancerID    string               `json:"loadbalancer_id,omitempty"`
	Protocol          ELBProtocol          `json:"protocol,omitempty"`
	Port              int                  `json:"port,omitempty"`
	BackendProtocol   ELBProtocol          `json:"backend_protocol,omitempty"`
	BackendPort       int                  `json:"backend_port,omitempty"`
	LBAlgorithm       ELBAlgorithm         `json:"lb_algorithm,omitempty"`
	SSLCertificate    string               `json:"ssl_certificate,omitempty"`
	SessionSticky     bool                 `json:"session_sticky"`
	StickySessionType ELBStickySessionType `json:"sticky_session_type,omitempty"`
	CookieTimeout     time.Duration        `json:"cookie_timeout,omitempty"`
	TCPTimeout        int                  `json:"tcp_timeout,omitempty"`
}

type ListenerRsp struct {
	Listener
	ID            string `json:"id"`
	TenantID      string `json:"tenant_id"`
	Status        string `json:"status"`
	CreateTime    string `json:"create_time"`
	HealthcheckID string `json:"healthcheck_id"`
}

type ListenerDetail struct {
	ListenerModify
	LoadbalancerID    string               `json:"loadbalancer_id"`
	Protocol          ELBProtocol          `json:"protocol"`
	Port              int                  `json:"port"`
	BackendProtocol   ELBProtocol          `json:"backend_protocol"`
	BackendPort       int                  `json:"backend_port"`
	LBAlgorithm       ELBAlgorithm         `json:"lb_algorithm"`
	SSLCertificate    string               `json:"ssl_certificate,omitempty"`
	SessionSticky     bool                 `json:"session_sticky,omitempty"`
	StickySessionType ELBStickySessionType `json:"sticky_session_type,omitempty"`
	CookieTimeout     time.Duration        `json:"cookie_timeout,omitempty"`
	ID                string               `json:"id"`
	TenantID          string               `json:"tenant_id"`
	Status            string               `json:"status"`
	CreateTime        string               `json:"create_time"`
	HealthcheckID     string               `json:"healthcheck_id"`
	TCPTimeout        int                  `json:"tcp_timeout,omitempty"`
}

// HealthCheck
type HealthCheck struct {
	// create Listener parameters
	ListenerID             string      `json:"listener_id,omitempty"`
	HealthcheckProtocol    ELBProtocol `json:"healthcheck_protocol"`
	HealthcheckURI         string      `json:"healthcheck_uri,omitempty"`
	HealthcheckConnectPort int         `json:"healthcheck_connect_port"`
	HealthyThreshold       int         `json:"healthy_threshold"`
	UnhealthyThreshold     int         `json:"unhealthy_threshold"`
	HealthcheckTimeout     int         `json:"healthcheck_timeout"`
	HealthcheckInterval    int         `json:"healthcheck_interval"`
}

type HealthCheckRsp struct {
	HealthCheck
	ID       string `json:"id,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
}

type HealthCheckDetail struct {
	HealthCheckRsp
	HealthcheckStatus string `json:"healthcheck_status,omitempty"`
	CreateTime        string `json:"create_time"`
}

// Backend host Member
type Member struct {
	// create Member parameters
	ListenerID string `json:"listener_id,omitempty"`
	ServerID   string `json:"server_id"`
	Address    string `json:"address"`
}

type MemberRm struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}

type MembersDel struct {
	RemoveMember []MemberRm `json:"removeMember"`
}

type MemDetail struct {
	Member
	ServerAddress string              `json:"server_address"`
	ID            string              `json:"id"`
	Status        string              `json:"status"`
	Listeners     []map[string]string `json:"listeners"`
	ServerName    string              `json:"server_name"`
	// NORMAL, ABNORMAL, UNAVAILABLE
	HealthStatus string `json:"health_status"`
}

type QuotaResource struct {
	Type  string `json:"type"`
	Used  int    `json:"used"`
	Quota int    `json:"quota"`
}

// Quota
type Quota struct {
	Quotas struct {
		Resources []QuotaResource `json:"resources"`
	} `json:"quotas"`
}

// sticky_session_type
type ELBStickySessionType string

const (
	ELBStickySessionTypeInsert ELBStickySessionType = "insert"
	ELBStickySessionTypeServer ELBStickySessionType = "server"
)

const (
	ELBJobStatusSuccess = "SUCCESS"
	ELBJobStatusFail    = "FAIL"
	ELBJobStatusRunning = "RUNNING"
	ELBJobStatusInit    = "INIT"
)

const (
	ELBHealthStatusNormal      = "NORMAL"
	ELBHealthStatusAbnormal    = "ABNORMAL"
	ELBHealthStatusUnavailable = "UNAVAILABLE"
)

type ELBClient struct {
	ecsClient *ServiceClient
	elbClient *ServiceClient
}

// Asynchronous job query response
type AsyncJobResp struct {
	Status   string `json:"status"`
	Entities struct {
		Elb struct {
			LoadbalancerId string `json:"id"`
		} `json:"elb"`
		Members []struct {
			Address string `json:"address"`
			ID      string `json:"id"`
		} `json:"members"`
	} `json:"entities"`
	FailReason string `json:"fail_reason"`
	ErrorCode  string `json:"error_code"`
}

// create job response
type JobResp struct {
	JobID string `json:"job_id"`
	Uri   string `json:"uri"`
}

type ErrorRsp struct {
	Error struct {
		Message string      `json:"message"`
		Code    LbErrorCode `json:"code"`
	} `json:"error"`
}

// Ecs address
type EcsAddress struct {
	Addr string `json:"addr"`
}

// EcsServer ecs server info
type Server struct {
	Id        string                  `json:"id"`
	Name      string                  `json:"name"`
	Addresses map[string][]EcsAddress `json:"addresses,omitempty"`
}

// Ecs servicers deatil
type EcsServers struct {
	Servers []Server `json:"servers,omitempty"`
}

func NewELBClient(cloud, region, projectID, accessKey, secretKey string) *ELBClient {
	elbEndpoint := fmt.Sprintf("https://ecs.%s.%s", region, cloud)
	ecsEndpoint := fmt.Sprintf("https://ecs.%s.%s", region, cloud)

	access := &AccessInfo{AccessKey: accessKey,
		SecretKey:   secretKey,
		Region:      region,
		ServiceType: "ec2",
	}

	ecsClient := &ServiceClient{
		Client:   httpClient,
		Endpoint: ecsEndpoint,
		Access:   access,
		TenantId: projectID,
	}

	elbClient := &ServiceClient{
		Client:   httpClient,
		Endpoint: elbEndpoint,
		Access:   access,
		TenantId: projectID,
	}

	return &ELBClient{
		ecsClient: ecsClient,
		elbClient: elbClient,
	}
}

// Regular expressions
// name: 1-64
const ELBNameFmt string = "[a-zA-Z\\_][a-zA-Z0-9\\_\\-]{0,63}"

// description: 0-128
const ELBDescFmt string = "[a-zA-Z0-9\\_\\-]{0,128}"

var elbNameFmtRegexp = regexp.MustCompile("^" + ELBNameFmt + "$")
var elnDescFmtRegexp = regexp.MustCompile("^" + ELBDescFmt + "$")

func IsValidName(name string) bool {
	return elbNameFmtRegexp.MatchString(name)
}

func IsValidDesc(desc string) bool {
	return elnDescFmtRegexp.MatchString(desc)
}

// ELB bandwidth: 1-300
func IsValidBandwidth(bandwidth int) bool {
	return 1 <= bandwidth && bandwidth <= 300
}

func (e *ELBClient) waitJobEnd(jobID string) (*AsyncJobResp, error) {
	for i := 0; i < tryJobStatusTimes; i++ {
		job, err := e.GetJobStatus(jobID)
		if err != nil {
			return nil, err
		}

		switch job.Status {
		case ELBJobStatusSuccess:
			return job, nil
		case ELBJobStatusFail:
			return nil, fmt.Errorf("job status is failed. id: %s, reason: %s", jobID, job.FailReason)
		default:
			time.Sleep(time.Second * 3)
			continue
		}
	}

	return nil, fmt.Errorf("start job time out id: %s", jobID)
}

func (e *ELBClient) WaitJobComplete(jobID string) error {
	err := common.WaitForCompleted(func() (bool, error) {
		job, err := e.GetJobStatus(jobID)
		if err != nil {
			klog.Errorf("Get job(%s) status error: %v", jobID, err)
			return false, nil
		}

		switch job.Status {
		case ELBJobStatusSuccess:
			return true, nil
		case ELBJobStatusFail:
			return false, fmt.Errorf("job status is failed. id: %s, reason: %s", jobID, job.FailReason)
		default:
			klog.Infof("Job is handling...")
			return false, nil
		}
	})

	return err
}

func (e *ELBClient) WaitMemberComplete(listenerID string, newMembers []*Member) error {
	err := common.WaitForCompleted(func() (bool, error) {
		members, err := e.ListMembers(listenerID)
		if err != nil {
			klog.Errorf("List members(%s) error: %v", listenerID, err)
			return false, nil
		}

		for _, m := range newMembers {
			for _, mDetail := range members {
				if m.ServerID == mDetail.ServerID {
					if mDetail.HealthStatus == MemberNormal {
						return true, nil
					} else if mDetail.HealthStatus == MemberUnavailable {
						return false, fmt.Errorf("Member(%s) status is abnormal", mDetail.ServerAddress)
					}
					break
				}
			}
		}

		klog.Infof("There has no normal members, waiting...")
		return false, nil
	})

	return err
}

func (e *ELBClient) GetJobStatus(jobID string) (*AsyncJobResp, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/jobs/" + jobID
	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	var job AsyncJobResp
	err = DecodeBody(resp, &job)
	if err != nil {
		return nil, fmt.Errorf("failed to getAsyncJobStatus : %v", err)
	}

	return &job, nil
}

func (e *ELBClient) Quota() (*Quota, error) {
	req := NewRequest(http.MethodGet, "/v1.0/"+e.elbClient.TenantId+"/elbaas/quotas", nil, nil)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	var quota Quota
	err = DecodeBody(resp, &quota)
	if err != nil {
		return nil, fmt.Errorf("Failed to getElbQuota : %v", err)
	}

	return &quota, nil
}

// Create an ELB instance
// 因为这个是通过portal创建的，所以无需API创建
func (e *ELBClient) CreateLoadBalancer(elbConf *ELB) (string, error) {
	if !IsValidBandwidth(elbConf.Bandwidth) ||
		!IsValidName(elbConf.Name) ||
		!IsValidDesc(elbConf.Description) ||
		(elbConf.Type != ELBTypeInternal && elbConf.Type != ELBTypeExternal) {
		return "", fmt.Errorf("invalid param")
	}

	quota, err := e.Quota()
	if err != nil {
		return "", err
	}

	bMatched := false
	for _, resource := range quota.Quotas.Resources {
		if resource.Type == QuotaTypeElb {
			bMatched = true
			if resource.Used >= resource.Quota {
				return "", fmt.Errorf("the elb quota is not enough")
			}

			break
		}
	}

	if !bMatched {
		return "", fmt.Errorf("the quota of elb type can not be found")
	}

	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/loadbalancers"
	req := NewRequest(http.MethodPost, url, nil, elbConf)
	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return "", err
	}

	var job JobResp
	err = DecodeBody(resp, &job)
	if err != nil {
		return "", fmt.Errorf("Failed to CreateElb : %v", err)
	}

	asyJob, err := e.waitJobEnd(job.JobID)
	if err != nil {
		return "", err
	}

	return asyJob.Entities.Elb.LoadbalancerId, nil
}

// DeleteLoadBalancer deletes loadbalancer by ID.
func (e *ELBClient) DeleteLoadBalancer(loadbalancerID string) error {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/loadbalancers/" + loadbalancerID

	req := NewRequest(http.MethodDelete, url, nil, nil)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return err
	}

	var job JobResp
	err = DecodeBody(resp, &job)
	if err != nil {
		return fmt.Errorf("Failed to DeleteElb : %v", err)
	}

	_, err = e.waitJobEnd(job.JobID)
	if err != nil {
		return err
	}

	return nil
}

// GetLoadBalancer gets an ELB instance by ID.
func (e *ELBClient) GetLoadBalancer(loadbalancerID string) (*ElbDetail, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/loadbalancers/" + loadbalancerID
	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	var elbDetail ElbDetail
	err = DecodeBody(resp, &elbDetail)
	if err != nil {
		return nil, fmt.Errorf("Failed to GetElbDetail : %v", err)
	}

	return &elbDetail, nil
}

// ListLoadBalancers list ELBs.
func (e *ELBClient) ListLoadBalancers(params map[string]string) (*ElbList, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/loadbalancers"
	var query string
	if len(params) != 0 {
		query += "?"

		for key, value := range params {
			if key != "" && value != "" {
				query += fmt.Sprintf("%s=%s&", key, value)
			}
		}

		query = query[0 : len(query)-1]
	}

	url += query
	req := NewRequest(http.MethodGet, url, nil, nil)
	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	// TODO: expect: {"loadbalancers":[],"instance_num":"0"}
	// but return: {"loadbalancers":{},"instance_num":"0"}. If there no ELB
	defer resp.Body.Close()
	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var elbNum *ElbNum
	err = json.Unmarshal([]byte(resBody), &elbNum)
	if err != nil {
		return nil, fmt.Errorf("Failed to get elbNum : %v", err)
	}
	if elbNum.InstanceNum == "0" {
		return &ElbList{InstanceNum: "0", Loadbalancers: []ElbDetail{}}, nil
	}

	var elbList ElbList
	err = json.Unmarshal([]byte(resBody), &elbList)
	if err != nil {
		return nil, fmt.Errorf("Failed to get elbList : %v", err)
	}

	return &elbList, nil
}

func (e *ELBClient) ModifyElb(elbConf *ELB) (*ELB, error) {
	return nil, nil
}

func (e *ELBClient) CreateListener(listenerConf *Listener) (*ListenerRsp, *ErrorRsp, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/listeners"
	req := NewRequest(http.MethodPost, url, nil, listenerConf)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	resBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var errRsp *ErrorRsp
		err = json.Unmarshal(resBody, &errRsp)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to unmarshal error resp: %v, %v", err, string(resBody))
		}

		return nil, errRsp, fmt.Errorf("Failed to create listener : %s, status code: %d", string(resBody), resp.StatusCode)
	}

	var listener ListenerRsp
	err = json.Unmarshal(resBody, &listener)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to unmarshal listener resp: %v, body %v", err, string(resBody))
	}

	return &listener, nil, nil
}

func (e *ELBClient) DeleteListener(listenerID string) error {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/listeners/" + listenerID

	req := NewRequest(http.MethodDelete, url, nil, nil)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	resBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Failed to delete listener : %s, status code: %d", string(resBody), resp.StatusCode)
	}

	return nil
}

func (e *ELBClient) GetListener(listenerID string) (*ListenerDetail, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/listeners/" + listenerID
	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	var listener ListenerDetail
	err = DecodeBody(resp, &listener)
	if err != nil {
		return nil, fmt.Errorf("Failed to GetListenerDetail : %v", err)
	}

	return &listener, nil
}

func (e *ELBClient) ListListeners(loadbalancerID string) ([]*ListenerDetail, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/listeners"
	if len(loadbalancerID) != 0 {
		url = url + "?loadbalancer_id=" + loadbalancerID
	}

	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	var listenerList []*ListenerDetail
	err = DecodeBody(resp, &listenerList)
	if err != nil {
		return nil, fmt.Errorf("Failed to GetListenersList : %v", err)
	}

	return listenerList, nil
}

func (e *ELBClient) UpdateListener(listener *Listener, listenerID string) (*ListenerDetail, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/listeners/" + listenerID
	req := NewRequest(http.MethodPut, url, nil, listener)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	var listenerDetail ListenerDetail
	err = DecodeBody(resp, &listenerDetail)
	if err != nil {
		return nil, fmt.Errorf("Failed to ModifyListener : %v", err)
	}

	return &listenerDetail, nil
}

func (e *ELBClient) CreateHealthCheck(healthConf *HealthCheck) (*HealthCheckRsp, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/healthcheck"

	req := NewRequest(http.MethodPost, url, nil, healthConf)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	var healthCheck HealthCheckRsp
	err = DecodeBody(resp, &healthCheck)
	if err != nil {
		return nil, fmt.Errorf("Failed to CreateHealthCheck : %v", err)
	}

	return &healthCheck, nil
}

// DeleteHealthCheck deletes a health check.
func (e *ELBClient) DeleteHealthCheck(healthcheckID string) error {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/healthcheck/" + healthcheckID

	req := NewRequest(http.MethodDelete, url, nil, nil)
	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	resBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Failed to delete HealthCheck : %s, status code: %d", string(resBody), resp.StatusCode)
	}

	return nil
}

// GetHealthCheck gets health check details info.
func (e *ELBClient) GetHealthCheck(healthcheckID string) (*HealthCheckDetail, *ErrorRsp, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/healthcheck/" + healthcheckID

	req := NewRequest(http.MethodGet, url, nil, nil)
	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()
	resBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var errRsp ErrorRsp
		err = json.Unmarshal(resBody, &errRsp)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to unmarshal error resp: %v, %v", err, string(resBody))
		}

		return nil, &errRsp, fmt.Errorf("Failed to get healthcheck : %s, status code: %d", string(resBody), resp.StatusCode)
	}

	var healthCheck HealthCheckDetail
	err = json.Unmarshal(resBody, &healthCheck)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to unmarshal healthcheck: %v", err)
	}

	return &healthCheck, nil, nil
}

func (e *ELBClient) UpdateHealthCheck(healthConf *HealthCheck, healthcheckID string) (*HealthCheckRsp, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/healthcheck/" + healthcheckID

	req := NewRequest(http.MethodPut, url, nil, healthConf)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	var healthCheck HealthCheckRsp
	err = DecodeBody(resp, &healthCheck)
	if err != nil {
		return nil, fmt.Errorf("Failed to ModifyHealthCheck : %v", err)
	}

	return &healthCheck, nil
}

func (e *ELBClient) RegisterInstancesWithListener(listenerID string, memberConf []*Member) (*AsyncJobResp, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/listeners/" + listenerID + "/members"

	req := NewRequest(http.MethodPost, url, nil, memberConf)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	var job JobResp
	err = DecodeBody(resp, &job)
	if err != nil {
		return nil, fmt.Errorf("Failed to AddMember : %v", err)
	}

	asyJobRsp, err := e.waitJobEnd(job.JobID)
	if err != nil {
		return nil, err
	}

	return asyJobRsp, nil
}

func (e *ELBClient) ListMembers(listenerID string) ([]*MemDetail, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/listeners/" + listenerID + "/members"

	req := NewRequest(http.MethodGet, url, nil, nil)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	// TODO: expect return body: [], but return: {}.
	defer resp.Body.Close()
	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(string(resBody)) == "{}" {
		return []*MemDetail{}, nil
	}

	var members []*MemDetail
	err = json.Unmarshal([]byte(resBody), &members)
	if err != nil {
		return nil, fmt.Errorf("Failed to GetMembers : %v", err)
	}

	return members, nil
}

// members as type *MembersDel
func (e *ELBClient) DeleteMembers(listenerID string) error {
	members, err := e.ListMembers(listenerID)
	if err != nil {
		return err
	}

	memDel := &MembersDel{}

	for _, member := range members {
		memDel.RemoveMember = append(memDel.RemoveMember, MemberRm{ID: member.ID, Address: member.Address})
	}

	if memDel.RemoveMember == nil || len(memDel.RemoveMember) == 0 {
		return nil
	}

	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/listeners/" + listenerID + "/members/action"

	req := NewRequest(http.MethodPost, url, nil, memDel)
	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return err
	}

	var job JobResp
	err = DecodeBody(resp, &job)
	if err != nil {
		return fmt.Errorf("Failed to DeleteMembers : %v", err)
	}

	err = e.WaitJobComplete(job.JobID)
	if err != nil {
		return err
	}

	return nil
}

// members as type *MembersDel
func (e *ELBClient) DeregisterInstancesFromListener(listenerID string, memDel *MembersDel) error {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/listeners/" + listenerID + "/members/action"
	req := NewRequest(http.MethodPost, url, nil, memDel)
	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return err
	}

	var job JobResp
	err = DecodeBody(resp, &job)
	if err != nil {
		return fmt.Errorf("Failed to DeleteSpecMembers : %v", err)
	}

	_, err = e.waitJobEnd(job.JobID)
	if err != nil {
		return err
	}

	return nil
}

// GetEcsByIp get hws ecs server by IP address
func (e *ELBClient) ListMachines() (*EcsServers, error) {
	url := "/v2/" + e.ecsClient.TenantId + "/servers/detail"
	req := NewRequest(http.MethodGet, url, nil, nil)
	resp, err := DoRequest(e.ecsClient, nil, req)
	if err != nil {
		return nil, err
	}

	var servers EcsServers
	err = DecodeBody(resp, &servers)
	if err != nil {
		return nil, fmt.Errorf("Failed to GetEcsList : %v", err)
	}

	return &servers, nil
}

func (e *ELBClient) AsyncCreateMembers(listenerID string, memberConf []*Member) (*JobResp, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/listeners/" + listenerID + "/members"

	req := NewRequest(http.MethodPost, url, nil, memberConf)

	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	var job JobResp
	err = DecodeBody(resp, &job)
	if err != nil {
		return nil, fmt.Errorf("Failed to AddMembers : %v", err)
	}

	return &job, nil
}

// AsyncDeleteMembers deletes members as type *MembersDel.
func (e *ELBClient) AsyncDeleteMembers(listenerID string, memDel *MembersDel) (*JobResp, error) {
	url := "/v1.0/" + e.elbClient.TenantId + "/elbaas/listeners/" + listenerID + "/members/action"
	req := NewRequest(http.MethodPost, url, nil, memDel)
	resp, err := DoRequest(e.elbClient, nil, req)
	if err != nil {
		return nil, err
	}

	var job JobResp
	err = DecodeBody(resp, &job)
	if err != nil {
		return nil, fmt.Errorf("Failed to DeleteMembers : %v", err)
	}

	return &job, nil
}
