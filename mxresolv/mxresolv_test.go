package mxresolv

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"testing"

	"github.com/mailgun/holster/v4/clock"
	"github.com/mailgun/holster/v4/collections"
	"github.com/stretchr/testify/assert"
)

func TestLookup(t *testing.T) {
	defer disableShuffle()()
	for i, tc := range []struct {
		desc          string
		inDomainName  string
		outMXHosts    []string
		outImplicitMX bool
	}{{
		desc:         "MX record preference is respected",
		inDomainName: "test-mx.definbox.com",
		outMXHosts: []string{
			/* 1 */ "mxa.definbox.com", "mxe.definbox.com", "mxi.definbox.com",
			/* 2 */ "mxc.definbox.com",
			/* 3 */ "mxb.definbox.com", "mxd.definbox.com", "mxf.definbox.com", "mxg.definbox.com", "mxh.definbox.com"},
		outImplicitMX: false,
	}, {
		inDomainName:  "test-a.definbox.com",
		outMXHosts:    []string{"test-a.definbox.com"},
		outImplicitMX: true,
	}, {
		inDomainName:  "test-cname.definbox.com",
		outMXHosts:    []string{"mxa.ninomail.com", "mxb.ninomail.com"},
		outImplicitMX: false,
	}, {
		desc: "If an MX host returned by the resolver contains non ASCII " +
			"characters then it is silently dropped from the returned list",
		inDomainName:  "test-unicode.definbox.com",
		outMXHosts:    []string{"mxa.definbox.com", "mxb.definbox.com"},
		outImplicitMX: false,
	}, {
		desc:          "Underscore is allowed in domain names",
		inDomainName:  "test-underscore.definbox.com",
		outMXHosts:    []string{"foo_bar.definbox.com"},
		outImplicitMX: false,
	}, {
		inDomainName:  "test-яндекс.definbox.com",
		outMXHosts:    []string{"xn--test---mofb0ab4b8camvcmn8gxd.definbox.com"},
		outImplicitMX: false,
	}, {
		inDomainName:  "xn--test--xweh4bya7b6j.definbox.com",
		outMXHosts:    []string{"xn--test---mofb0ab4b8camvcmn8gxd.definbox.com"},
		outImplicitMX: false,
	}} {
		fmt.Printf("Test case #%d: %s, %s\n", i, tc.inDomainName, tc.desc)
		// When
		ctx, cancel := context.WithTimeout(context.Background(), 3*clock.Second)
		mxHosts, explictMX, err := Lookup(ctx, tc.inDomainName, nil)
		cancel()
		// Then
		assert.NoError(t, err)
		assert.Equal(t, tc.outMXHosts, mxHosts)
		assert.Equal(t, tc.outImplicitMX, explictMX)

		// The second lookup returns the cached result, that only shows on the
		// coverage report.
		mxHosts, explictMX, err = Lookup(ctx, tc.inDomainName, nil)
		assert.NoError(t, err)
		assert.Equal(t, tc.outMXHosts, mxHosts)
		assert.Equal(t, tc.outImplicitMX, explictMX)
	}
}

func TestLookupError(t *testing.T) {
	defer disableShuffle()()
	for i, tc := range []struct {
		desc       string
		inHostname string
		outError   string
	}{{
		inHostname: "test-broken.definbox.com",
		outError:   "lookup test-broken.definbox.com.*: no such host",
	}, {
		inHostname: "",
		outError:   "lookup : no such host",
	}, {
		inHostname: "kaboom",
		outError:   "lookup kaboom.*: no such host",
	}, {
		inHostname: "example.com",
		outError:   "domain accepts no mail",
	}} {
		fmt.Printf("Test case #%d: %s, %s\n", i, tc.inHostname, tc.desc)
		// When
		ctx, cancel := context.WithTimeout(context.Background(), 3*clock.Second)
		_, _, err := Lookup(ctx, tc.inHostname, nil)
		cancel()
		// Then
		assert.Regexp(t, regexp.MustCompile(tc.outError), err.Error())

		// The second lookup returns the cached result, that only shows on the
		// coverage report.
		_, _, err = Lookup(ctx, tc.inHostname, nil)
		assert.Regexp(t, regexp.MustCompile(tc.outError), err.Error())
	}
}

// Shuffling only does not cross preference group boundaries.
//
// Preference groups are:
//  1: mxa.definbox.com, mxe.definbox.com, mxi.definbox.com
//  2: mxc.definbox.com
//  3: mxb.definbox.com, mxd.definbox.com, mxf.definbox.com, mxg.definbox.com, mxh.definbox.com
//
// Warning: since the data set is pretty small subsequent shuffles can produce
// the same result causing the test to fail.
func TestLookupShuffle(t *testing.T) {
	// When
	ctx, cancel := context.WithTimeout(context.Background(), 3*clock.Second)
	defer cancel()
	shuffle1, _, err := Lookup(ctx, "test-mx.definbox.com", nil)
	assert.NoError(t, err)
	resetCache()
	shuffle2, _, err := Lookup(ctx, "test-mx.definbox.com", nil)
	assert.NoError(t, err)

	// Then
	assert.NotEqual(t, shuffle1[:3], shuffle2[:3])
	assert.NotEqual(t, shuffle1[4:], shuffle2[4:])

	sort.Strings(shuffle1[:3])
	sort.Strings(shuffle2[:3])
	assert.Equal(t, []string{"mxa.definbox.com", "mxe.definbox.com", "mxi.definbox.com"}, shuffle1[:3])
	assert.Equal(t, shuffle1[:3], shuffle2[:3])

	assert.Equal(t, "mxc.definbox.com", shuffle1[3])
	assert.Equal(t, shuffle1[3], shuffle2[3])

	sort.Strings(shuffle1[4:])
	sort.Strings(shuffle2[4:])
	assert.Equal(t, []string{"mxb.definbox.com", "mxd.definbox.com", "mxf.definbox.com", "mxg.definbox.com", "mxh.definbox.com"}, shuffle1[4:])
	assert.Equal(t, shuffle1[4:], shuffle2[4:])
}

func disableShuffle() func() {
	shuffle = false
	return func() {
		shuffle = true
	}
}

func resetCache() {
	lookupResultCache = collections.NewLRUCache(1000)
}
