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

package metadata

import (
	"strings"
	"testing"
)

func TestParseMetadata(t *testing.T) {
	_, err := parseMetadata(strings.NewReader("bogus"))
	if err == nil {
		t.Errorf("Should fail when bad data is provided: %s", err)
	}

	data := strings.NewReader(`
{
  "uuid": "b77c45c1-b6cf-4f5e-b072-0ee86daeb6c2",
  "availability_zone": "ap-southeast-1b",
  "region_id": "ap-southeast-1",
  "name": "k8s-a01"
}
`)
	md, err := parseMetadata(data)
	if err != nil {
		t.Fatalf("Should succeed when provided with valid data: %s", err)
	}

	if md.Name != "k8s-a01" {
		t.Errorf("incorrect name: %s", md.Name)
	}

	if md.UUID != "b77c45c1-b6cf-4f5e-b072-0ee86daeb6c2" {
		t.Errorf("incorrect uuid: %s", md.UUID)
	}

	if md.AvailabilityZone != "ap-southeast-1b" {
		t.Errorf("incorrect az: %s", md.AvailabilityZone)
	}

	if md.RegionID != "ap-southeast-1" {
		t.Errorf("incorrect region: %s", md.AvailabilityZone)
	}
}
