package httpsign

import (
	"bytes"
	"fmt"
	"testing"
)

var _ = fmt.Printf // for testing

func TestRealRandom(t *testing.T) {
	// We really can't test the output of the csprng, so let's just check that
	// the output lengths match what we think we ask for.

	// Get the real random number generator.
	csprng := realRandom{}

	// Test Bytes().
	b, _ := csprng.bytes(16)
	if g, w := len(b), 16; g != w {
		t.Errorf("&realRandom{}.Bytes(16) produced a slice of length %d; want %d", g, w)
	}

	// Test HexDigest().
	s, _ := csprng.hexDigest(16)
	if g, w := len(s), 32; g != w {
		t.Errorf("&realRandom{}.HexDigest(16) produced a slice of length %d; want %d", g, w)
	}
}

func TestFakeRandom(t *testing.T) {
	// Get fake random number generator.
	frng := fakeRandom{}

	// Test Bytes().
	g0, err := frng.bytes(8)
	if err != nil {
		t.Error("Got unexpected error from frng.Bytes:", err)
	}
	if w := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}; !bytes.Equal(g0, w) {
		t.Errorf("&FRNG{}.Bytes(8) = %v; want %v", g0, w)
	}

	// Test HexDigest().
	g1, err := frng.hexDigest(4)
	if err != nil {
		t.Error("Got unexpected error from frng.HexDigest:", err)
	}
	if w := "00010203"; g1 != w {
		t.Errorf("&FRNG{}.HexDigest(4) = %v; want %v", g1, w)
	}
}

func TestSeededRNG(t *testing.T) {
	rng := SeededRNG{}

	// Test Bytes().
	g0, err := rng.bytes(8)
	if err != nil {
		t.Error("Got unexpected error from SeededRNG.Bytes:", err)
	}
	if w := []byte{0xfa, 0x12, 0xf9, 0x2a, 0xfb, 0xe0, 0x0f, 0x85}; !bytes.Equal(g0, w) {
		t.Errorf("&SeededRNG{Seed: 0}.Bytes(8) = %v, want %v", g0, w)
	}

	// Reseed.
	rng = SeededRNG{}

	// Test HexDigest().
	g1, err := rng.hexDigest(4)
	if w := "fa12f92a"; g1 != w {
		t.Errorf("&SeededRNG{Seed: 0}.Bytes(4) = %v, want %v", g1, w)
	}
}
