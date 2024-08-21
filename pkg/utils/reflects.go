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
	"strings"
)

func GetStructField(object any, keys string) (reflect.Value, error) {
	value := reflect.ValueOf(object)
	if value.Kind() == reflect.Slice || (value.Kind() == reflect.Pointer && value.Elem().Kind() == reflect.Slice) {
		return reflect.Value{}, fmt.Errorf("GetStructField: arg: object must be a struct, not a slice")
	}
	for _, key := range strings.Split(keys, ".") {
		var err error
		value, err = ReflectStructField(value, key)
		if err != nil {
			return reflect.Value{}, err
		}
	}
	return value, nil
}

func ReflectStructField(value reflect.Value, key string) (reflect.Value, error) {
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}

	field := value.FieldByName(key)
	if field.IsValid() {
		return field, nil
	}
	return reflect.Value{}, fmt.Errorf("key %s not found in %#v", key, value)
}
