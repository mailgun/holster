package mxresolv

import (
	"context"
	"math/rand"
	"net"
	"sort"
	"strings"
	"unicode"
	_ "unsafe" // For go:linkname

	"github.com/mailgun/holster/v4/clock"
	"github.com/mailgun/holster/v4/collections"
	"github.com/mailgun/holster/v4/errors"
	"golang.org/x/net/idna"
)

const (
	cacheSize = 1000
	cacheTTL  = 10 * clock.Minute
)

var (
	errNullMXRecord   = errors.New("domain accepts no mail")
	errNoValidMXHosts = errors.New("no valid MX hosts")

	lookupResultCache *collections.LRUCache

	// It is modified only in tests to make them deterministic.
	shuffle = true
)

func init() {
	lookupResultCache = collections.NewLRUCache(cacheSize)
}

// Lookup performs a DNS lookup of MX records for the specified hostname. It
// returns a prioritised list of MX hostnames, where hostnames with the same
// priority are shuffled. If the second returned value is true, then the host
// does not have explicit MX records, and its A record is returned instead.
//
// It uses an LRU cache with a timeout to reduce the number of network requests.
func Lookup(ctx context.Context, hostname string) (retMxHosts []string, retImplicit bool, reterr error) {
	if cachedVal, ok := lookupResultCache.Get(hostname); ok {
		lookupResult := cachedVal.(lookupResult)
		return lookupResult.mxHosts, lookupResult.implicit, lookupResult.err
	}
	asciiHostname, err := ensureASCII(hostname)
	if err != nil {
		return nil, false, errors.Wrap(err, "invalid hostname")
	}
	mxRecords, err := lookupMX(net.DefaultResolver, ctx, asciiHostname)
	if err != nil {
		var timeouter interface{ Timeout() bool }
		if errors.As(err, &timeouter) && timeouter.Timeout() {
			return nil, false, errors.WithStack(err)
		}
		var netDNSError *net.DNSError
		if errors.As(err, &netDNSError) && netDNSError.Err == "no such host" {
			if _, err := net.DefaultResolver.LookupIPAddr(ctx, asciiHostname); err != nil {
				return cacheAndReturn(hostname, nil, false, errors.WithStack(err))
			}
			return cacheAndReturn(hostname, []string{asciiHostname}, true, nil)
		}
		if mxRecords == nil {
			return cacheAndReturn(hostname, nil, false, errors.WithStack(err))
		}
	}
	// Check for "Null MX" record (https://tools.ietf.org/html/rfc7505).
	if len(mxRecords) == 1 {
		if mxRecords[0].Host == "." {
			return cacheAndReturn(hostname, nil, false, errNullMXRecord)
		}
		// 0.0.0.0 is not really a "Null MX" record, but some people apparently
		// have never heard of RFC7505 and configure it this way.
		if strings.HasPrefix(mxRecords[0].Host, "0.0.0.0") {
			return cacheAndReturn(hostname, nil, false, errNullMXRecord)
		}
	}
	// Normalize returned hostnames: drop trailing '.' and lowercase.
	for _, mxRecord := range mxRecords {
		lastCharIndex := len(mxRecord.Host) - 1
		if mxRecord.Host[lastCharIndex] == '.' {
			mxRecord.Host = strings.ToLower(mxRecord.Host[:lastCharIndex])
		}
	}
	// Sort records in order of preference and lexicographically within a
	// preference group. The latter is only to make tests deterministic.
	sort.Slice(mxRecords, func(i, j int) bool {
		return mxRecords[i].Pref < mxRecords[j].Pref ||
			(mxRecords[i].Pref == mxRecords[j].Pref && mxRecords[i].Host < mxRecords[j].Host)
	})
	// Shuffle records within preference groups unless disabled in tests.
	if shuffle {
		mxRecordCount := len(mxRecords)
		groupBegin := 0
		for i := 1; i < mxRecordCount; i++ {
			if mxRecords[i].Pref != mxRecords[groupBegin].Pref || i == mxRecordCount-1 {
				groupSlice := mxRecords[groupBegin:i]
				rand.Shuffle(len(groupSlice), func(i, j int) {
					groupSlice[i], groupSlice[j] = groupSlice[j], groupSlice[i]
				})
				groupBegin = i
			}
		}
	}
	// Make a hostname list, but skip non-ASCII names, that cause issues.
	mxHosts := make([]string, 0, len(mxRecords))
	for _, mxRecord := range mxRecords {
		if !isASCII(mxRecord.Host) {
			continue
		}
		if mxRecord.Host == "" {
			continue
		}
		mxHosts = append(mxHosts, mxRecord.Host)
	}
	if len(mxHosts) == 0 {
		return cacheAndReturn(hostname, nil, false, errNoValidMXHosts)
	}
	return cacheAndReturn(hostname, mxHosts, false, nil)
}

func ensureASCII(hostname string) (string, error) {
	if isASCII(hostname) {
		return hostname, nil
	}
	hostname, err := idna.ToASCII(hostname)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return hostname, nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

type lookupResult struct {
	mxHosts  []string
	implicit bool
	err      error
}

func cacheAndReturn(hostname string, mxHosts []string, implicit bool, err error) (retMxHosts []string, retImplicit bool, reterr error) {
	lookupResultCache.AddWithTTL(hostname, lookupResult{mxHosts, implicit, err}, cacheTTL)
	return mxHosts, implicit, err
}

// lookupMX exposes the respective private function of net.Resolver. The public
// alternative net.(*Resolver).LookupMX considers MX records that contain an IP
// address invalid. It is indeed invalid according to an RFC, but in reality
// some people do not read RFC and configure IP addresses in MX records.
//
// An issue against the Golang proper was created to remove the strict MX DNS
// record validation https://github.com/golang/go/issues/56025. When it is
// fixed we will be able to remove this unsafe binding and get back to calling
// the public method.
//
//go:linkname lookupMX net.(*Resolver).lookupMX
func lookupMX(r *net.Resolver, ctx context.Context, name string) ([]*net.MX, error)
