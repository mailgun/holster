package cfg

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"text/template"

	"github.com/go-yaml/yaml"
)

func LoadConfig(configPath string, configStruct interface{}) error {
	bytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	bytes, err = substitute(bytes)
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(bytes, configStruct); err != nil {
		return err
	}

	if err = validate(configStruct); err != nil {
		return err
	}

	return nil
}

type templateData struct {
	Env map[string]string
}

func substitute(in []byte) ([]byte, error) {
	t, err := template.New("config").Parse(string(in))
	if err != nil {
		return nil, err
	}

	data := &templateData{
		Env: make(map[string]string),
	}

	values := os.Environ()
	for _, val := range values {
		keyval := strings.SplitN(val, "=", 2)
		if len(keyval) != 2 {
			continue
		}
		data.Env[keyval[0]] = keyval[1]
	}

	buffer := &bytes.Buffer{}
	if err = t.Execute(buffer, data); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

type InValid struct {
	Name  string
	Type  string
}

func validate(object interface{}) error {
	if valid := validateValue(object); valid != nil {
		return errors.New(fmt.Sprintf("Missing required config field: %v of type %s", valid.Name, valid.Type))
	}
	return nil
}

func validateValue(object interface{}) *InValid {
	objType := reflect.TypeOf(object)
	objValue := reflect.ValueOf(object)
	// If object is a nil interface value, TypeOf returns nil.
	if objType == nil {
		// Don't validate nil interfaces
		return nil
	}

	switch objType.Kind() {
	case reflect.Ptr:
		// If the ptr is nil
		if objValue.IsNil() {
			return &InValid{Type: objType.String()}
		}
		// De-reference the ptr and pass the object to validate
		return validateValue(objValue.Elem().Interface())
	case reflect.Struct:
		for idx := 0; idx < objValue.NumField(); idx++ {
			if valid := validateValue(objValue.Field(idx).Interface()); valid != nil {
				field := objType.Field(idx)
				// Capture sub struct names
				if valid.Name != "" {
					field.Name = field.Name + "." + valid.Name
				}

				// If our field is a pointer and it's pointing to an object
				if field.Type.Kind() == reflect.Ptr && !objValue.Field(idx).IsNil() {
					// The optional doesn't apply because our field does exist
					// instead the de-referenced object failed validation
					if field.Tag.Get("config") == "optional" {
						return &InValid{Name: field.Name, Type: valid.Type}
					}
				}
				// If the field is optional, don't invalidate
				if field.Tag.Get("config") != "optional" {
					return &InValid{Name: field.Name, Type: valid.Type}
				}
			}
		}
	// no way to tell if boolean or integer fields are provided or not
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.Bool, reflect.Interface,
		reflect.Func:
		return nil
	default:
		if objValue.Len() == 0 {
			return &InValid{Type: objType.Name()}
		}
	}
	return nil
}
