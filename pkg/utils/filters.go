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
	"fmt"
	"reflect"
)

// FilterBasicSlice can filter the slice all through a slice filter.
//
//	The filtered result will be stored to the memory pointed to by result.
//
// Example: Filter basic slice all through a slice filter..
//
//	var rst2 []User
//	data := []string{"a", "a", "b", "c"}
//	sliceFilter := []string{"a", "b"}
//	FilterBasicSlice(data, sliceFilter, rst2)       // rst2 = ["a", "a", "b"]
//	FilterBasicSlice(data, sliceFilter, rst2, true) // rst2 = ["a", "b"]
//
// Example: Filter pointer of basic type slice all through a slice filter.
//
//	var rst2 []string
//	a := "a"
//	b := "b"
//	data := []string{&a, &a, &b}
//	sliceFilter := []string{"a"}
//	FilterBasicSlice(data, sliceFilter, rst2)       // rst2 = [&a, &a]
//	FilterBasicSlice(data, sliceFilter, rst2, true) // rst2 = [&a]
func FilterBasicSlice(target any, filter any, result any, args ...any) error {
	rstRefVal := reflect.ValueOf(result)
	if rstRefVal.Kind() != reflect.Pointer {
		return fmt.Errorf("FilterSlice arg: result must be a pointer")
	}
	rstArr := rstRefVal.Elem()
	destArr := make([]reflect.Value, 0)

	targetVal := getReflectVal(target)
	if targetVal.Kind() != reflect.Slice {
		return fmt.Errorf("FilterSlice arg: target must be a slice")
	}

	filterMapper := make(map[interface{}]bool)

	filterVal := getReflectVal(filter)
	for i := 0; i < filterVal.Len(); i++ {
		itemRefVal := filterVal.Index(i)
		if itemRefVal.Kind() == reflect.Ptr {
			itemRefVal = itemRefVal.Elem()
		}
		if itemRefVal.Kind() == reflect.Struct {
			return fmt.Errorf("object in slice is not a basic type")
		}
		filterMapper[itemRefVal.Interface()] = true
	}

	dedupe := false
	if len(args) == 1 {
		if v, ok := args[0].(bool); ok {
			dedupe = v
		}
	}

	for i := 0; i < targetVal.Len(); i++ {
		itemRefVal := targetVal.Index(i)
		if itemRefVal.Kind() == reflect.Ptr {
			itemRefVal = itemRefVal.Elem()
		}
		if itemRefVal.Kind() == reflect.Struct {
			return fmt.Errorf("object in slice is not a basic type")
		}
		if v, ok := filterMapper[itemRefVal.Interface()]; ok && v {
			destArr = append(destArr, itemRefVal)
			filterMapper[itemRefVal.Interface()] = !dedupe
		}
	}

	rstArr.Set(reflect.Append(rstArr, destArr...))
	return nil
}

// FilterSlice can filter the slice all through a map or slice filter.
//
//	If the field is a nested value, using dot(.) to split them, e.g. "StructField.SubField".
//	The filtered result will be stored to the memory pointed to by result.
//
// Example: Filter struct slice all through a map filter.
//
//	var rst1 []User
//	users := []User{{ID: 1, Name: "Jerry"}, {ID: 2, Name: "Tom"}}
//	mapFilter := map[string]any{"ID": 0, "Name": "Tom"}
//	FilterSlice(users, mapFilter, rst1)       // rst1 = []
//	FilterSlice(users, mapFilter, rst1, true) // rst1 = [{"ID": 2, "Name": "Tom"}]
//
// Example: Filter basic type slice all through a slice filter.
//
//	var rst2 []string
//	data := []string{"a", "a", "b", "c"}
//	sliceFilter := []string{"a", "b"}
//	FilterSlice(data, sliceFilter, rst2)       // rst2 = ["a", "a", "b"]
//	FilterSlice(data, sliceFilter, rst2, true) // rst2 = ["a", "b"]
func FilterSlice(target any, filter any, result any, args ...any) error {
	rstRefVal := reflect.ValueOf(result)
	if rstRefVal.Kind() != reflect.Pointer {
		return fmt.Errorf("FilterSlice arg: result must be a pointer")
	}
	if getReflectVal(filter).Kind() == reflect.Slice {
		return FilterBasicSlice(target, filter, result, args...)
	}

	rstArr := rstRefVal.Elem()
	destArr := make([]reflect.Value, 0)

	targetVal := getReflectVal(target)
	if targetVal.Kind() != reflect.Slice {
		return fmt.Errorf("FilterSlice arg: target must be a slice")
	}

	newFilter := filter.(map[string]interface{})
	if len(args) == 1 {
		if ignoreZero, ok := args[0].(bool); ok && ignoreZero {
			newFilter = removeZero(newFilter)
		}
	}

	for i := 0; i < targetVal.Len(); i++ {
		itemRefVal := targetVal.Index(i)
		if itemRefVal.Kind() == reflect.Ptr {
			itemRefVal = itemRefVal.Elem()
		}
		if itemRefVal.Kind() != reflect.Struct {
			return fmt.Errorf("object in slice is not a struct")
		}

		isMatch := true
		for key, val := range newFilter {
			itemVal, err := getItemFieldVal(itemRefVal, key)
			if err != nil {
				return err
			}
			if itemVal.Interface() != val {
				isMatch = false
				break
			}
		}
		if isMatch {
			destArr = append(destArr, itemRefVal)
		}
	}

	rstArr.Set(reflect.Append(rstArr, destArr...))
	return nil
}

func getReflectVal(a any) reflect.Value {
	rv := reflect.ValueOf(a)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	return rv
}

func removeZero(filter map[string]interface{}) map[string]interface{} {
	newFilter := filter
	for key, val := range filter {
		keyValue := reflect.ValueOf(val)
		if keyValue.IsZero() {
			delete(newFilter, key)
		}
	}
	return newFilter
}

func getItemFieldVal(itemRefVal reflect.Value, key string) (*reflect.Value, error) {
	itemVal, err := GetStructField(itemRefVal.Interface(), key)
	if err != nil {
		return nil, err
	}
	if itemVal.Kind() == reflect.Ptr {
		itemVal = itemVal.Elem()
	}
	return &itemVal, nil
}
