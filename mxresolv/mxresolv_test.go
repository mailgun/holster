package mxresolv_test

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"reflect"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/foxcpp/go-mockdns"
	"github.com/mailgun/holster/v4/clock"
	"github.com/mailgun/holster/v4/errors"
	"github.com/mailgun/holster/v4/mxresolv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	zones := map[string]mockdns.Zone{
		"test-a.definbox.com.": {
			A: []string{"192.168.19.2"},
		},
		"test-cname.definbox.com.": {
			CNAME: "definbox.com.",
		},
		"definbox.com.": {
			MX: []net.MX{
				{Host: "mxa.ninomail.com.", Pref: 10},
				{Host: "mxb.ninomail.com.", Pref: 10},
			},
		},
		"prefer.example.com.": {
			MX: []net.MX{
				{Host: "mxa.example.com.", Pref: 20},
				{Host: "mxb.example.com.", Pref: 1},
			},
		},
		"prefer3.example.com.": {
			MX: []net.MX{
				{Host: "mxa.example.com.", Pref: 1},
				{Host: "mxb.example.com.", Pref: 1},
				{Host: "mxc.example.com.", Pref: 2},
			},
		},
		"test-unicode.definbox.com.": {
			MX: []net.MX{
				{Host: "mxa.definbox.com.", Pref: 1},
				{Host: "ex\\228mple.com.", Pref: 2},
				{Host: "mxb.definbox.com.", Pref: 3},
			},
		},
		"test-underscore.definbox.com.": {
			MX: []net.MX{
				{Host: "foo_bar.definbox.com.", Pref: 1},
			},
		},
		"xn--test--xweh4bya7b6j.definbox.com.": {
			MX: []net.MX{
				{Host: "xn--test---mofb0ab4b8camvcmn8gxd.definbox.com.", Pref: 10},
			},
		},
		"test-mx-ipv4.definbox.com.": {
			MX: []net.MX{
				{Host: "34.150.176.225.", Pref: 10},
			},
		},
		"test-mx-ipv6.definbox.com.": {
			MX: []net.MX{
				{Host: "::ffff:2296:b0e1.", Pref: 10},
			},
		},
		"example.com.": {
			MX: []net.MX{
				{Host: ".", Pref: 0},
			},
		},
		"test-mx-zero.definbox.com.": {
			MX: []net.MX{
				{Host: "0.0.0.0.", Pref: 0},
			},
		},
		"test-mx.definbox.com.": {
			MX: []net.MX{
				{Host: "mxg.definbox.com.", Pref: 3},
				{Host: "mxa.definbox.com.", Pref: 1},
				{Host: "mxe.definbox.com.", Pref: 1},
				{Host: "mxi.definbox.com.", Pref: 1},
				{Host: "mxd.definbox.com.", Pref: 3},
				{Host: "mxc.definbox.com.", Pref: 2},
				{Host: "mxb.definbox.com.", Pref: 3},
				{Host: "mxf.definbox.com.", Pref: 3},
				{Host: "mxh.definbox.com.", Pref: 3},
			},
		},
	}
	server, err := SpawnMockDNS(zones)
	if err != nil {
		panic(err)
	}

	server.Patch(mxresolv.Resolver)
	exitVal := m.Run()
	server.UnPatch(mxresolv.Resolver)
	server.Stop()
	os.Exit(exitVal)
}

func TestLookupWithPref(t *testing.T) {
	for _, tc := range []struct {
		desc          string
		inDomainName  string
		outMXHosts    []*net.MX
		outImplicitMX bool
	}{{
		desc:         "MX record preference is respected",
		inDomainName: "test-mx.definbox.com",
		outMXHosts: []*net.MX{
			{Host: "mxa.definbox.com", Pref: 1}, {Host: "mxe.definbox.com", Pref: 1}, {Host: "mxi.definbox.com", Pref: 1},
			{Host: "mxc.definbox.com", Pref: 2},
			{Host: "mxb.definbox.com", Pref: 3}, {Host: "mxd.definbox.com", Pref: 3}, {Host: "mxf.definbox.com", Pref: 3}, {Host: "mxg.definbox.com", Pref: 3}, {Host: "mxh.definbox.com", Pref: 3},
		},
		outImplicitMX: false,
	}, {
		inDomainName:  "test-a.definbox.com",
		outMXHosts:    []*net.MX{{Host: "test-a.definbox.com", Pref: 1}},
		outImplicitMX: true,
	}, {
		inDomainName:  "test-cname.definbox.com",
		outMXHosts:    []*net.MX{{Host: "mxa.ninomail.com", Pref: 10}, {Host: "mxb.ninomail.com", Pref: 10}},
		outImplicitMX: false,
	}, {
		inDomainName:  "definbox.com",
		outMXHosts:    []*net.MX{{Host: "mxa.ninomail.com", Pref: 10}, {Host: "mxb.ninomail.com", Pref: 10}},
		outImplicitMX: false,
	}, {
		desc: "If an MX host returned by the resolver contains non ASCII " +
			"characters then it is silently dropped from the returned list",
		inDomainName:  "test-unicode.definbox.com",
		outMXHosts:    []*net.MX{{Host: "mxa.definbox.com", Pref: 1}, {Host: "mxb.definbox.com", Pref: 3}},
		outImplicitMX: false,
	}, {
		desc:          "Underscore is allowed in domain names",
		inDomainName:  "test-underscore.definbox.com",
		outMXHosts:    []*net.MX{{Host: "foo_bar.definbox.com", Pref: 1}},
		outImplicitMX: false,
	}, {
		inDomainName:  "test-яндекс.definbox.com",
		outMXHosts:    []*net.MX{{Host: "xn--test---mofb0ab4b8camvcmn8gxd.definbox.com", Pref: 10}},
		outImplicitMX: false,
	}, {
		inDomainName:  "xn--test--xweh4bya7b6j.definbox.com",
		outMXHosts:    []*net.MX{{Host: "xn--test---mofb0ab4b8camvcmn8gxd.definbox.com", Pref: 10}},
		outImplicitMX: false,
	}, {
		inDomainName:  "test-mx-ipv4.definbox.com",
		outMXHosts:    []*net.MX{{Host: "34.150.176.225", Pref: 10}},
		outImplicitMX: false,
	}, {
		inDomainName:  "test-mx-ipv6.definbox.com",
		outMXHosts:    []*net.MX{{Host: "::ffff:2296:b0e1", Pref: 10}},
		outImplicitMX: false,
	}} {
		t.Run(tc.inDomainName, func(t *testing.T) {
			defer mxresolv.SetDeterministicInTests()()

			// When
			ctx, cancel := context.WithTimeout(context.Background(), 3*clock.Second)
			defer cancel()
			mxRecords, implicitMX, err := mxresolv.LookupWithPref(ctx, tc.inDomainName)
			// Then
			assert.NoError(t, err)
			assert.Equal(t, tc.outMXHosts, mxRecords)
			assert.Equal(t, tc.outImplicitMX, implicitMX)
		})
	}
}

func TestLookup(t *testing.T) {
	for _, tc := range []struct {
		desc          string
		inDomainName  string
		outMXHosts    []string
		outImplicitMX bool
	}{{
		desc:         "MX record preference is respected",
		inDomainName: "test-mx.definbox.com",
		outMXHosts: []string{
			/* 1 */ "mxa.definbox.com", "mxi.definbox.com", "mxe.definbox.com",
			/* 2 */ "mxc.definbox.com",
			/* 3 */ "mxb.definbox.com", "mxf.definbox.com", "mxh.definbox.com", "mxd.definbox.com", "mxg.definbox.com",
		},
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
		inDomainName:  "definbox.com",
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
	}, {
		inDomainName:  "test-mx-ipv4.definbox.com",
		outMXHosts:    []string{"34.150.176.225"},
		outImplicitMX: false,
	}, {
		inDomainName:  "test-mx-ipv6.definbox.com",
		outMXHosts:    []string{"::ffff:2296:b0e1"},
		outImplicitMX: false,
	}} {
		t.Run(tc.inDomainName, func(t *testing.T) {
			defer mxresolv.SetDeterministicInTests()()

			// When
			ctx, cancel := context.WithTimeout(context.Background(), 3*clock.Second)
			defer cancel()
			mxHosts, implicitMX, err := mxresolv.Lookup(ctx, tc.inDomainName)
			// Then
			assert.NoError(t, err)
			assert.Equal(t, tc.outMXHosts, mxHosts)
			assert.Equal(t, tc.outImplicitMX, implicitMX)
		})
	}
}

func TestLookupRegression(t *testing.T) {
	defer mxresolv.SetDeterministicInTests()()
	mxresolv.ResetCache()

	// When
	ctx, cancel := context.WithTimeout(context.Background(), 3*clock.Second)
	defer cancel()

	mxHosts, explictMX, err := mxresolv.Lookup(ctx, "test-mx.definbox.com")
	// Then
	require.NoError(t, err)
	assert.Equal(t, []string{
		/* 1 */ "mxa.definbox.com", "mxi.definbox.com", "mxe.definbox.com",
		/* 2 */ "mxc.definbox.com",
		/* 3 */ "mxb.definbox.com", "mxf.definbox.com", "mxh.definbox.com", "mxd.definbox.com", "mxg.definbox.com",
	}, mxHosts)
	assert.Equal(t, false, explictMX)

	// The second lookup returns the cached result, the cached result is shuffled.
	mxHosts, explictMX, err = mxresolv.Lookup(ctx, "test-mx.definbox.com")
	require.NoError(t, err)
	assert.Equal(t, []string{
		/* 1 */ "mxe.definbox.com", "mxi.definbox.com", "mxa.definbox.com",
		/* 2 */ "mxc.definbox.com",
		/* 3 */ "mxh.definbox.com", "mxf.definbox.com", "mxg.definbox.com", "mxd.definbox.com", "mxb.definbox.com",
	}, mxHosts)
	assert.Equal(t, false, explictMX)

	mxHosts, _, err = mxresolv.Lookup(ctx, "definbox.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"mxb.ninomail.com", "mxa.ninomail.com"}, mxHosts)

	// Should always prefer mxb over mxa since mxb has a lower pref than mxa
	for i := 0; i < 100; i++ {
		mxHosts, _, err = mxresolv.Lookup(ctx, "prefer.example.com")
		require.NoError(t, err)
		assert.Equal(t, []string{"mxb.example.com", "mxa.example.com"}, mxHosts)
	}

	// Should randomly order mxa and mxb. We make lookup 10 times and make sure
	// that the returned result is not always the same.
	mxHosts, _, err = mxresolv.Lookup(ctx, "prefer3.example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"mxb.example.com", "mxa.example.com", "mxc.example.com"}, mxHosts)
	sameCount := 0
	for i := 0; i < 10; i++ {
		mxHosts2, _, err := mxresolv.Lookup(ctx, "prefer3.example.com")
		assert.NoError(t, err)
		if reflect.DeepEqual(mxHosts, mxHosts2) {
			sameCount++
		}
	}
	assert.Less(t, sameCount, 10)

	// mxc.example.com should always be last as it has a different priority,
	// than the other two.
	for i := 0; i < 100; i++ {
		mxHosts, _, err = mxresolv.Lookup(ctx, "prefer3.example.com")
		require.NoError(t, err)
		assert.Equal(t, "mxc.example.com", mxHosts[2])
	}
}

func TestLookupError(t *testing.T) {
	for _, tc := range []struct {
		desc         string
		inDomainName string
		outError     string
	}{
		{
			inDomainName: "test-bogus.definbox.com",
			outError:     "lookup test-bogus.definbox.com.*: no such host",
		},
		{
			inDomainName: "",
			outError:     "lookup : no such host",
		},
		{
			inDomainName: "kaboom",
			outError:     "lookup kaboom.*: no such host",
		},
		{
			// MX  0  .
			inDomainName: "example.com",
			outError:     "domain accepts no mail",
		},
		{
			// MX  10  0.0.0.0.
			inDomainName: "test-mx-zero.definbox.com",
			outError:     "domain accepts no mail",
		},
	} {
		t.Run(tc.inDomainName, func(t *testing.T) {
			// When
			ctx, cancel := context.WithTimeout(context.Background(), 3*clock.Second)
			defer cancel()
			_, _, err := mxresolv.Lookup(ctx, tc.inDomainName)

			// Then
			require.Error(t, err)
			assert.Regexp(t, regexp.MustCompile(tc.outError), err.Error())

			gotTemporary := false
			var temporary interface{ Temporary() bool }
			if errors.As(err, &temporary) {
				gotTemporary = temporary.Temporary()
			}
			assert.False(t, gotTemporary)

			// The second lookup returns the cached result, that only shows on the
			// coverage report.
			_, _, err = mxresolv.Lookup(ctx, tc.inDomainName)
			assert.Regexp(t, regexp.MustCompile(tc.outError), err.Error())
		})
	}
}

// Shuffling does not cross preference group boundaries.
//
// Preference groups are:
//
//	1: mxa.definbox.com, mxe.definbox.com, mxi.definbox.com
//	2: mxc.definbox.com
//	3: mxb.definbox.com, mxd.definbox.com, mxf.definbox.com, mxg.definbox.com, mxh.definbox.com
func TestLookupShuffle(t *testing.T) {
	defer mxresolv.SetDeterministicInTests()()

	// When
	ctx, cancel := context.WithTimeout(context.Background(), 3*clock.Second)
	defer cancel()
	shuffle1, _, err := mxresolv.Lookup(ctx, "test-mx.definbox.com")
	assert.NoError(t, err)
	shuffle2, _, err := mxresolv.Lookup(ctx, "test-mx.definbox.com")
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
	assert.Equal(t, []string{"mxb.definbox.com", "mxd.definbox.com", "mxf.definbox.com",
		"mxg.definbox.com", "mxh.definbox.com"}, shuffle1[4:])
	assert.Equal(t, shuffle1[4:], shuffle2[4:])
}

func TestDistribution(t *testing.T) {
	mxresolv.ResetCache()

	// 2 host distribution should be uniform
	dist := make(map[string]int, 2)
	for i := 0; i < 1000; i++ {
		s, _, _ := mxresolv.Lookup(context.Background(), "definbox.com")
		_, ok := dist[s[0]]
		if ok {
			dist[s[0]] += 1
		} else {
			dist[s[0]] = 0
		}
	}

	assertDistribution(t, dist, 35.0)

	dist = make(map[string]int, 3)
	for i := 0; i < 1000; i++ {
		s, _, _ := mxresolv.Lookup(context.Background(), "test-mx.definbox.com")
		_, ok := dist[s[0]]
		if ok {
			dist[s[0]] += 1
		} else {
			dist[s[0]] = 0
		}
	}
	assertDistribution(t, dist, 35.0)

	// This is what a standard distribution looks like when 3 hosts have the same MX priority
	// spew.Dump(dist)
	// (map[string]int) (len=3) {
	// 	(string) (len=16) "mxa.definbox.com": (int) 324,
	//	(string) (len=16) "mxe.definbox.com": (int) 359,
	//	(string) (len=16) "mxi.definbox.com": (int) 314
	// }
}

// Golang optimizes the allocation so there is no hit to performance or memory usage when calling
// `rand.New()` for each call to `shuffleNew()` over `rand.Shuffle()` which has a mutex.
//
// pkg: github.com/mailgun/holster/v4/mxresolv
// BenchmarkShuffleWithNew
// BenchmarkShuffleWithNew-10    	   61962	     18434 ns/op	    5376 B/op	       1 allocs/op
// BenchmarkShuffleGlobal
// BenchmarkShuffleGlobal-10     	   65205	     18480 ns/op	       0 B/op	       0 allocs/op
func BenchmarkShuffleWithNew(b *testing.B) {
	for n := b.N; n > 0; n-- {
		shuffleNew()
	}
	b.ReportAllocs()
}

func BenchmarkShuffleGlobal(b *testing.B) {
	for n := b.N; n > 0; n-- {
		shuffleGlobal()
	}
	b.ReportAllocs()
}

func shuffleNew() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(52, func(i, j int) {})
}

func shuffleGlobal() {
	rand.Shuffle(52, func(i, j int) {})
}

func assertDistribution(t *testing.T, dist map[string]int, expected float64) {
	t.Helper()

	// Calculate the mean of the distribution
	var sum int
	for _, value := range dist {
		sum += value
	}
	mean := float64(sum) / float64(len(dist))

	// Calculate the sum of squared differences
	var squaredDifferences float64
	for _, value := range dist {
		diff := float64(value) - mean
		squaredDifferences += diff * diff
	}

	// Calculate the variance and standard deviation
	variance := squaredDifferences / float64(len(dist))
	stdDev := math.Sqrt(variance)

	// The distribution of random hosts chosen should not exceed 35
	assert.False(t, stdDev > expected,
		fmt.Sprintf("Standard deviation is greater than %f:", expected)+"%.2f", stdDev)

}
