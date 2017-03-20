package holster_test

import (
	"github.com/Sirupsen/logrus"
	"github.com/mailgun/holster"
	. "gopkg.in/check.v1"
)

type MiscTestSuite struct{}

var _ = Suite(&MiscTestSuite{})

func (s *MiscTestSuite) SetUpSuite(c *C) {
}

func (s *MiscTestSuite) TestToFieldsStruct(c *C) {
	var conf struct {
		Foo string
		Bar int
	}
	conf.Bar = 23
	conf.Foo = "bar"

	fields := holster.ToFields(conf)
	c.Assert(fields, DeepEquals, logrus.Fields{
		"Foo": "bar",
		"Bar": 23,
	})
}

func (s *MiscTestSuite) TestToFieldsMap(c *C) {
	conf := map[string]interface{}{
		"Bar": 23,
		"Foo": "bar",
	}

	fields := holster.ToFields(conf)
	c.Assert(fields, DeepEquals, logrus.Fields{
		"Foo": "bar",
		"Bar": 23,
	})
}

func (s *MiscTestSuite) TestToFieldsPanic(c *C) {
	defer func() {
		if r := recover(); r != nil {
			c.Assert(r, Equals, "ToFields(): value must be of kind struct or map")
		}
	}()

	// Should panic
	holster.ToFields(1)
	c.Fatalf("Should have caught panic")
}
