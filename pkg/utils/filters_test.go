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

func TestFilterSlice(t *testing.T) {
	type childrenStruct struct {
		ID      int
		PIntVal *int
		FValue  float64
		PFValue *float64
		StrVal  string
		PStrVal *string
	}
	type filterTestStruct struct {
		ID      int
		PIntVal *int
		FValue  float64
		PFValue *float64
		StrVal  string
		PStrVal *string

		StructA  childrenStruct
		PStructA *childrenStruct
	}

	testData := make([]filterTestStruct, 0, 9)
	for i := 1; i <= 9; i++ {
		val := i % 3
		fVal := float64(i % 3)
		strVal := "strVal_" + strconv.Itoa(i)
		child := childrenStruct{
			ID:      i,
			PIntVal: &val,
			FValue:  fVal,
			PFValue: &fVal,
			StrVal:  strVal,
			PStrVal: &strVal,
		}

		obj := filterTestStruct{
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

	testCases := []struct {
		name       string
		filter     map[string]interface{}
		expected   int
		ignoreZero bool
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
		}, {
			name: "test 8",
			filter: map[string]interface{}{
				"PIntVal":          1,
				"FValue":           float64(1),
				"PFValue":          float64(1),
				"PStructA.PIntVal": 1,
				"StrVal":           "",
			},
			ignoreZero: true,
			expected:   3,
		}, {
			name: "test 9",
			filter: map[string]interface{}{
				"PIntVal":          1,
				"FValue":           float64(1),
				"PFValue":          float64(1),
				"PStructA.PIntVal": 1,
				"StrVal":           "",
			},
			ignoreZero: false,
			expected:   0,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var result []filterTestStruct

			if err := FilterSlice(testData, testCase.filter, &result, testCase.ignoreZero); err != nil {
				t.Fatalf("filter error: %s", err)
			} else if len(result) != testCase.expected {
				t.Fatalf("expected: %d, got : %d", testCase.expected, len(result))
			}
		})
	}
}

func TestFilterSliceBasic(t *testing.T) {
	a := "a"
	b := "b"
	c := "c"
	d := "d"

	target := []string{a, b, c, d}
	filter := []string{a, b}

	targetPtr := []*string{&a, &b, &c, &d}
	filterPtr := []*string{&a, &b}

	testCases := []struct {
		name     string
		target   any
		filter   any
		dedupe   bool
		expected int
	}{
		{
			name:     "test 1",
			target:   target,
			filter:   filter,
			expected: 2,
		}, {
			name:     "test 2",
			target:   targetPtr,
			filter:   filter,
			expected: 2,
		}, {
			name:     "test 3",
			target:   target,
			filter:   filterPtr,
			expected: 2,
		}, {
			name:     "test 4",
			target:   targetPtr,
			filter:   filterPtr,
			expected: 2,
		}, {
			name:     "test 5",
			target:   &targetPtr,
			filter:   &filterPtr,
			expected: 2,
		}, {
			name:     "test 6",
			target:   &target,
			filter:   &filter,
			expected: 2,
		}, {
			name:     "test 7",
			target:   target,
			filter:   &filter,
			expected: 2,
		}, {
			name:     "test 8",
			target:   &target,
			filter:   filter,
			expected: 2,
		}, {
			name:     "test 9",
			target:   []int{0, 0, 1, 2, 3},
			filter:   []int{0, 1},
			expected: 3,
		}, {
			name:     "test 10",
			target:   []int{0, 0, 1, 2, 3},
			filter:   []int{0, 1},
			dedupe:   true,
			expected: 2,
		}, {
			name:     "test 11",
			target:   []bool{true, true, true, false, false},
			filter:   []bool{false},
			expected: 2,
		}, {
			name:     "test 12",
			target:   []bool{true, true, true, false, false},
			filter:   []bool{false},
			dedupe:   true,
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result []any
			if err := FilterSlice(tc.target, tc.filter, &result, tc.dedupe); err != nil {
				t.Fatalf("filter error: %s", err)
			} else if len(result) != tc.expected {
				t.Fatalf("expected: %d, got : %d", tc.expected, len(result))
			}
		})
	}
}
