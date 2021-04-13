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

type LbErrorCode string

const (
	ElbError6091 LbErrorCode = "ELB.6091" //create listener failed: ELB already has 5 listeners
	ElbError2012 LbErrorCode = "ELB.2012" //backend server does not exist
	ElbError1101 LbErrorCode = "ELB.1101" //invalid parameter: vip_address already exists
	ElbError6101 LbErrorCode = "ELB.6101" //create listener failed: duplicated port
	ElbError7020 LbErrorCode = "ELB.7020" //healthcheck is not exist
)
