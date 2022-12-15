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
	"strconv"
	"testing"
)

func TestFilter(t *testing.T) {
	type filterTestStructA struct {
		ID      int
		PIntVal *int
		FValue  float64
		PFValue *float64
		StrVal  string
		PStrVal *string
	}
	type filterTestStructB struct {
		ID      int
		PIntVal *int
		FValue  float64
		PFValue *float64
		StrVal  string
		PStrVal *string

		StructA  filterTestStructA
		PStructA *filterTestStructA
	}

	testData := make([]filterTestStructB, 0, 9)
	for i := 1; i <= 9; i++ {
		val := i % 3
		fVal := float64(i % 3)
		strVal := "strVal_" + strconv.Itoa(i)
		child := filterTestStructA{
			ID:      i,
			PIntVal: &val,
			FValue:  fVal,
			PFValue: &fVal,
			StrVal:  strVal,
			PStrVal: &strVal,
		}

		obj := filterTestStructB{
			ID:       i,
			PIntVal:  &val,
			FValue:   fVal,
			PFValue:  &fVal,
			StrVal:   strVal,
			PStrVal:  &strVal,
			StructA:  child,
			PStructA: &child,
		}
		testData = append(testData, obj)
	}

	tests := []struct {
		name     string
		filter   map[string]interface{}
		expected int
	}{
		{
			name: "test 1",
			filter: map[string]interface{}{
				"ID": 1,
			},
			expected: 1,
		}, {
			name: "test 2",
			filter: map[string]interface{}{
				"PIntVal": 1,
			},
			expected: 3,
		}, {
			name: "test 3",
			filter: map[string]interface{}{
				"PIntVal": 1,
				"FValue":  float64(1),
				"PFValue": float64(1),
			},
			expected: 3,
		}, {
			name: "test 4",
			filter: map[string]interface{}{
				"StructA.PIntVal": 1,
				"StructA.FValue":  float64(1),
				"StructA.PFValue": float64(1),
			},
			expected: 3,
		}, {
			name: "test 5",
			filter: map[string]interface{}{
				"ID":      1,
				"StrVal":  "strVal_1",
				"PStrVal": "strVal_1",
			},
			expected: 1,
		}, {
			name: "test 6",
			filter: map[string]interface{}{
				"PIntVal":         1,
				"FValue":          float64(1),
				"PFValue":         float64(1),
				"StructA.PIntVal": 1,
			},
			expected: 3,
		}, {
			name: "test 7",
			filter: map[string]interface{}{
				"PIntVal":          1,
				"FValue":           float64(1),
				"PFValue":          float64(1),
				"PStructA.PIntVal": 1,
			},
			expected: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []filterTestStructB

			if err := FilterSlice(testData, tt.filter, &result); err != nil {
				t.Fatalf("filter error: %s", err)
			} else if len(result) != tt.expected {
				t.Fatalf("expected: %d, got : %d", tt.expected, len(result))
			}
		})
	}
}
