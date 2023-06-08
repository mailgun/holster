package httpsign

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"math"
	math_rand "math/rand"
)

// Interface for our random number generator. We need this
// to fake random  values in tests.
type randomProvider interface {
	bytes(bytes int) ([]byte, error)
	hexDigest(bytes int) (string, error)
}

// Real random values, used in production
type realRandom struct{}

// Return n-bytes of random values from the realRandom.
func (c *realRandom) bytes(bytes int) ([]byte, error) {
	n := make([]byte, bytes)

	// get bytes-bit random number from /dev/urandom
	_, err := io.ReadFull(rand.Reader, n)
	if err != nil {
		return nil, err
	}

	return n, nil
}

// Return n-bytes of random values from the realRandom but as a
// hex-encoded (base16) string.
func (c *realRandom) hexDigest(bytes int) (string, error) {
	return hexDigest(c, bytes)
}

// Fake random, used in tests. never use this in production!
type fakeRandom struct{}

// Fake random number generator, never use in production. Always
// returns a predictable sequence of bytes that looks like: 0x00,
// 0x01, 0x02, 0x03, ...
func (f *fakeRandom) bytes(bytes int) ([]byte, error) {
	// create bytes long array
	b := make([]byte, bytes)

	for i := 0; i < len(b); i++ {
		b[i] = byte(i)
	}

	return b, nil
}

// Fake random number generator, never use in production. Always returns
// a predictable hex-encoded (base16) string that looks like "00010203..."
func (f *fakeRandom) hexDigest(bytes int) (string, error) {
	return hexDigest(f, bytes)
}

// SeededRNG returns bytes generated in a predictable sequence by package math/rand.
// Not cryptographically secure, not thread safe.
// Changes to Seed after the first call to Bytes or HexDigest
// will have no effect. The zero value of SeededRNG is ready to use,
// and will use a seed of 0.
type SeededRNG struct {
	Seed int64
	rand *math_rand.Rand
}

// Bytes conforms to the randomProvider interface. Returns bytes
// generated by a math/rand.Rand.
func (r *SeededRNG) bytes(bytes int) ([]byte, error) {
	if r.rand == nil {
		//nolint: gosec // Use of math/rand is indended.
		r.rand = math_rand.New(math_rand.NewSource(r.Seed))
	}
	b := make([]byte, bytes)
	for i := range b {
		b[i] = byte(r.rand.Intn(math.MaxUint8 + 1))
	}
	return b, nil
}

// HexDigest conforms to the randomProvider interface. Returns
// a hex encoding of bytes generated by a math/rand.Rand.
func (r *SeededRNG) hexDigest(bytes int) (string, error) {
	return hexDigest(r, bytes)
}

func hexDigest(r randomProvider, bytes int) (string, error) {
	b, err := r.bytes(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
