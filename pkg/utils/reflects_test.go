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
	"reflect"
	"testing"

	"k8s.io/utils/pointer"
)

func TestReflectBasicType(t *testing.T) {
	tests := []struct {
		name     string
		target   interface{}
		keys     string
		expected interface{}
	}{
		{
			name: "reflect_test_int",
			target: struct {
				Value int
			}{
				Value: 1,
			},
			keys:     "Value",
			expected: 1,
		}, {
			name: "reflect_test_bool",
			target: struct {
				Value bool
			}{
				Value: true,
			},
			keys:     "Value",
			expected: true,
		}, {
			name: "reflect_test_uint",
			target: struct {
				Value uint
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: uint(123),
		}, {
			name: "reflect_test_uint8",
			target: struct {
				Value uint8
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: uint8(123),
		}, {
			name: "reflect_test_uint16",
			target: struct {
				Value uint16
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: uint16(123),
		}, {
			name: "reflect_test_uint32",
			target: struct {
				Value uint32
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: uint32(123),
		}, {
			name: "reflect_test_uint64",
			target: struct {
				Value uint64
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: uint64(123),
		}, {
			name: "reflect_test_int8",
			target: struct {
				Value int8
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: int8(123),
		}, {
			name: "reflect_test_int16",
			target: struct {
				Value int16
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: int16(123),
		}, {
			name: "reflect_test_int32",
			target: struct {
				Value int32
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: int32(123),
		}, {
			name: "reflect_test_int64",
			target: struct {
				Value int64
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: int64(123),
		}, {
			name: "reflect_test_float32",
			target: struct {
				Value float32
			}{
				Value: float32(123.321),
			},
			keys:     "Value",
			expected: float32(123.321),
		}, {
			name: "reflect_test_float64",
			target: struct {
				Value float64
			}{
				Value: float64(123.321),
			},
			keys:     "Value",
			expected: float64(123.321),
		}, {
			name: "reflect_test_complex64",
			target: struct {
				Value complex64
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: complex64(123),
		}, {
			name: "reflect_test_icomplex128",
			target: struct {
				Value complex128
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: complex128(123),
		}, {
			name: "reflect_test_string",
			target: struct {
				Value string
			}{
				Value: "ABCabc123",
			},
			keys:     "Value",
			expected: "ABCabc123",
		}, {
			name: "reflect_test_uintptr",
			target: struct {
				Value uintptr
			}{
				Value: 123,
			},
			keys:     "Value",
			expected: uintptr(123),
		}, {
			name: "reflect_test_inner",
			target: struct {
				Value struct {
					TestVal string
				}
			}{
				Value: struct {
					TestVal string
				}{
					TestVal: "Abcdefg",
				},
			},
			keys:     "Value.TestVal",
			expected: "Abcdefg",
		}, {
			name: "reflect_test_inner_ptr1",
			target: struct {
				Value struct {
					TestVal *string
				}
			}{
				Value: struct {
					TestVal *string
				}{
					TestVal: pointer.String("Abcdefg"),
				},
			},
			keys:     "Value.TestVal",
			expected: "Abcdefg",
		},
		{
			name: "reflect_test_inner_ptr2",
			target: struct {
				Value *struct {
					TestVal *string
				}
			}{
				Value: &struct {
					TestVal *string
				}{
					TestVal: pointer.String("Abcdefg"),
				},
			},
			keys:     "Value.TestVal",
			expected: "Abcdefg",
		}, {
			name: "reflect_test_slice",
			target: struct {
				Value []string
			}{
				Value: []string{"a", "b", "c"},
			},
			keys:     "Value",
			expected: []string{"a", "b", "c"},
		}, {
			name: "reflect_test_slice2_ptr",
			target: struct {
				Value *[]string
			}{
				Value: &[]string{"a", "b", "c"},
			},
			keys:     "Value",
			expected: []string{"a", "b", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetStructField(tt.target, tt.keys)
			if err != nil {
				t.Fatalf("Failed, name: %s, error: %s", tt.name, err)
			}

			if result.Kind() == reflect.Pointer {
				result = result.Elem()
			}

			if result.Kind() == reflect.Slice {
				if result.Len() != reflect.ValueOf(tt.expected).Len() {
					t.Errorf("GetStructField() test slice error, got = %v, expected %v",
						result.Interface(), tt.expected)
				}
			} else {
				if result.Interface() != tt.expected {
					t.Errorf("GetStructField() test basic type error, got = %v, expected %v",
						result.Interface(), tt.expected)
				}
			}
		})
	}
}
