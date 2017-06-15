package httpsign

import (
	"fmt"
	"testing"
	"time"

	"github.com/mailgun/holster"
)

var _ = fmt.Printf // for testing

func TestInCache(t *testing.T) {
	// setup
	nc := NewNonceCache(
		100,
		1,
		&holster.FrozenClock{CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC)},
	)

	// nothing in cache, it should be valid
	inCache := nc.InCache("0")
	if inCache {
		t.Error("Check should be valid, but failed.")
	}

	// second time around it shouldn't be
	inCache = nc.InCache("0")
	if !inCache {
		t.Error("Check should be invalid, but passed.")
	}

	// check some other value
	inCache = nc.InCache("1")
	if inCache {
		t.Error("Check should be valid, but failed.")
	}

	// age off first value, then it should be valid
	ftime := nc.clock.(*holster.FrozenClock)
	time4 := time.Date(2012, 3, 4, 5, 6, 10, 0, time.UTC)
	ftime.CurrentTime = time4

	inCache = nc.InCache("0")
	if inCache {
		t.Error("Check should be valid, but failed.")
	}
}
