package clock

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
)

type DurationJSON struct {
	Duration time.Duration
	str      string
}

func (d DurationJSON) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

func (d *DurationJSON) UnmarshalJSON(b []byte) error {
	var v interface{}
	var err error

	if err = json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		d.str = fmt.Sprintf("%f", value)
	case string:
		d.Duration, err = time.ParseDuration(value)
		d.str = value
	default:
		return errors.New("invalid duration")
	}
	return err
}

func (d DurationJSON) String() string {
	return d.str
}
