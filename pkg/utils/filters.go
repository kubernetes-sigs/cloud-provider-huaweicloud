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

func FilterSlice(target interface{}, filter map[string]interface{}, result interface{}) error {
	rstRefVal := reflect.ValueOf(result)
	if rstRefVal.Kind() != reflect.Pointer {
		return fmt.Errorf("FilterSlice arg: result must be a pointer")
	}
	rstArr := rstRefVal.Elem()
	destArr := make([]reflect.Value, 0)

	targetVal := reflect.ValueOf(target)
	if targetVal.Kind() != reflect.Slice {
		return fmt.Errorf("FilterSlice arg: target must be a slice")
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
		for key, val := range filter {
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
