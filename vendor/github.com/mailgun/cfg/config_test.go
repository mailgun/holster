package cfg

import (
	"os"
	"testing"

	. "gopkg.in/check.v1"
)

func TestConfig(t *testing.T) { TestingT(t) }

type ConfigSuite struct{}

var _ = Suite(&ConfigSuite{})

type PtrStruct struct {
	PtrBlah *string
	PtrOptional *string `config:"optional"`
}

type PtrOptional struct {
	PtrBlah *string
}

type MissingBoolean struct {
	Optional  *bool
	Mandatory *bool
}

type Config struct {
	Key     string
	List    []string `config:"optional"`
	Env     string
	Bool    bool
	Builtin struct {
		AnotherKey     string `config:"optional"`
		Mandatory      string
		OneMoreBuiltin struct {
			Blah string
		}
	}
	PtrStruct *PtrStruct
	PtrOptional *PtrOptional `config:"optional"`
	Number int32
}

func (s *ConfigSuite) TestConfigOk(c *C) {
	config := Config{}

	// set environment variable to test templating
	os.Setenv("SOME_ENV_KEY", "env_value")

	err := LoadConfig("configs/correct.yaml", &config)
	c.Assert(err, IsNil)
	c.Assert(config.Key, Equals, "val")
	c.Assert(config.List, DeepEquals, []string{"val1", "val2"})
	c.Assert(config.Env, Equals, "env_value")
	c.Assert(config.Builtin.AnotherKey, Equals, "anothervalue")
	c.Assert(config.Builtin.Mandatory, Equals, "onemorevalue")
	c.Assert(config.Builtin.OneMoreBuiltin.Blah, Equals, "blah")
	c.Assert(*config.PtrStruct.PtrBlah, Equals, "blah")
	c.Assert(*config.PtrStruct.PtrOptional, Equals, "optional")
	c.Assert(*config.PtrOptional.PtrBlah, Equals, "blah")
}

func (s *ConfigSuite) TestConfigMissingRequired(c *C) {
	config := Config{}

	err := LoadConfig("configs/missing1.yaml", &config)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "Missing required config field: Key of type string")
}

func (s *ConfigSuite) TestConfigMissingOptional(c *C) {
	config := Config{}

	err := LoadConfig("configs/missing3.yaml", &config)
	c.Assert(err, IsNil)
}

func (s *ConfigSuite) TestConfigMissingRequiredInBuiltin(c *C) {
	config := Config{}

	err := LoadConfig("configs/missing2.yaml", &config)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "Missing required config field: Builtin.Mandatory of type string")
}

func (s *ConfigSuite) TestConfigOptionalPtr(c *C) {
	config := Config{}

	err := LoadConfig("configs/missing4.yaml", &config)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "Missing required config field: PtrOptional.PtrBlah of type *string")
}

func (s *ConfigSuite) TestMissingBoolean(c *C) {
	config := MissingBoolean{}

	err := LoadConfig("configs/missing5.yaml", &config)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "Missing required config field: Mandatory of type *bool")
}
