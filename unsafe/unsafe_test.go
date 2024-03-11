package unsafe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBytesToString(t *testing.T) {
	assert.Equal(t, "hello", BytesToString([]byte("hello")))
}

func TestStringToBytes(t *testing.T) {
	assert.Equal(t, []byte("hello"), StringToBytes("hello"))
}
