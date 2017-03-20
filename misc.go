package holster

import (
	"reflect"

	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/fatih/structs"
)

// Given a struct or map[string]interface{} return as a logrus.Fields{} map
func ToFields(value interface{}) logrus.Fields {
	v := reflect.ValueOf(value)
	var hash map[string]interface{}
	var ok bool

	switch v.Kind() {
	case reflect.Struct:
		hash = structs.Map(value)
	case reflect.Map:
		hash, ok = value.(map[string]interface{})
		if !ok {
			panic("ToFields(): map kind must be of type map[string]interface{}")
		}
	default:
		panic("ToFields(): value must be of kind struct or map")
	}

	result := make(logrus.Fields, len(hash))
	for key, value := range hash {
		// Convert values the JSON marshaller doesn't know how to marshal
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.Func:
			value = fmt.Sprintf("%+v", value)
		case reflect.Struct, reflect.Map:
			value = ToFields(value)
		}

		// Ensure the key is a string. convert it if not
		v = reflect.ValueOf(key)
		if v.Kind() != reflect.String {
			key = fmt.Sprintf("%+v", key)
		}
		result[key] = value
	}
	return result
}
