/*
Copyright 2017 Mailgun Technologies Inc

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
package setter_test

import (
	"testing"

	"github.com/mailgun/holster/v4/setter"
	"github.com/stretchr/testify/assert"
)

func TestIfEmpty(t *testing.T) {
	var conf struct {
		Foo string
		Bar int
	}
	assert.Equal(t, "", conf.Foo)
	assert.Equal(t, 0, conf.Bar)

	// Should apply the default values
	setter.SetDefault(&conf.Foo, "default")
	setter.SetDefault(&conf.Bar, 200)

	assert.Equal(t, "default", conf.Foo)
	assert.Equal(t, 200, conf.Bar)

	conf.Foo = "thrawn"
	conf.Bar = 500

	// Should NOT apply the default values
	setter.SetDefault(&conf.Foo, "default")
	setter.SetDefault(&conf.Bar, 200)

	assert.Equal(t, "thrawn", conf.Foo)
	assert.Equal(t, 500, conf.Bar)
}

func TestIfDefaultPrecedence(t *testing.T) {
	var conf struct {
		Foo string
		Bar string
	}
	assert.Equal(t, "", conf.Foo)
	assert.Equal(t, "", conf.Bar)

	// Should use the final default value
	envValue := ""
	setter.SetDefault(&conf.Foo, envValue, "default")
	assert.Equal(t, "default", conf.Foo)

	// Should use envValue
	envValue = "bar"
	setter.SetDefault(&conf.Bar, envValue, "default")
	assert.Equal(t, "bar", conf.Bar)
}

func TestIsEmpty(t *testing.T) {
	var count64 int64
	var thing string

	// Should return true
	assert.Equal(t, true, setter.IsZero(count64))
	assert.Equal(t, true, setter.IsZero(thing))

	thing = "thrawn"
	count64 = int64(1)
	assert.Equal(t, false, setter.IsZero(count64))
	assert.Equal(t, false, setter.IsZero(thing))
}

func TestIfEmptyTypePanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "reflect.Set: value of type int is not assignable to type string", r)
		}
	}()

	var thing string
	// Should panic
	setter.SetDefault(&thing, 1)
	assert.Fail(t, "Should have caught panic")
}

func TestIfEmptyNonPtrPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, "setter.SetDefault: Expected first argument to be of type reflect.Ptr", r)
		}
	}()

	var thing string
	// Should panic
	setter.SetDefault(thing, "thrawn")
	assert.Fail(t, "Should have caught panic")
}

type MyInterface interface {
	Thing() string
}

type MyImplementation struct{}

func (s *MyImplementation) Thing() string {
	return "thing"
}

func NewImplementation() MyInterface {
	// Type and Value are not nil
	var p *MyImplementation = nil
	return p
}

type MyStruct struct {
	T MyInterface
}

func NewMyStruct(t MyInterface) *MyStruct {
	return &MyStruct{T: t}
}

func TestIsNil(t *testing.T) {
	m := MyStruct{T: &MyImplementation{}}
	assert.True(t, m.T != nil)
	m.T = nil
	assert.True(t, m.T == nil)

	o := NewMyStruct(nil)
	assert.True(t, o.T == nil)

	thing := NewImplementation()
	assert.False(t, thing == nil)
	assert.True(t, setter.IsNil(thing))
	assert.False(t, setter.IsNil(&MyImplementation{}))
}
