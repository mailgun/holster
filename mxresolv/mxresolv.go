package mxresolv

import (
	"context"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
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

	// randomizer allows the seed function to be patched in tests using SetDeterministic()
	randomizerMu sync.Mutex
	randomizer   = rand.New(rand.NewSource(time.Now().UnixNano()))

	// Resolver is exposed to be patched in tests
	Resolver = net.DefaultResolver
)

func init() {
	lookupResultCache = collections.NewLRUCache(cacheSize)
}

// Lookup performs a DNS lookup of MX records for the specified domain. It
// returns a prioritised list of MX hostnames, where hostnames with the same
// priority are shuffled. If the second returned value is true, then the host
// does not have explicit MX records, and its A record is returned instead.
//
// It uses an LRU cache with a timeout to reduce the number of network requests.
func Lookup(ctx context.Context, domain string) (mxHosts []string, implicit bool, err error) {
	mxRecords, implicit, err := LookupWithPref(ctx, domain)
	if err != nil {
		return nil, false, err
	}
	if len(mxRecords) == 1 {
		return []string{mxRecords[0].Host}, implicit, err
	}
	return shuffleMXRecords(mxRecords), false, nil
}

// LookupWithPref performs a DNS lookup of MX records for the specified domain.
// It returns a slice of net.MX records that are ordered by preference. Records
// with the same preference are sorted by hostname to ensure deterministic
// behaviour. If the second returned value is true, then the host does not have
// explicit MX records, and its A record is used instead.
//
// It uses an LRU cache with a timeout to reduce the number of network requests.
func LookupWithPref(ctx context.Context, domainName string) (mxRecords []*net.MX, implicit bool, err error) {
	if cachedVal, ok := lookupResultCache.Get(domainName); ok {
		cachedLookupResult := cachedVal.(lookupResult)
		return cachedLookupResult.mxRecords, cachedLookupResult.implicit, cachedLookupResult.err
	}

	asciiDomainName, err := ensureASCII(domainName)
	if err != nil {
		return nil, false, errors.Wrap(err, "invalid domain name")
	}
	mxRecords, err = lookupMX(Resolver, ctx, asciiDomainName)
	if err != nil {
		var netDNSError *net.DNSError
		if errors.As(err, &netDNSError) && netDNSError.IsNotFound {
			if _, err := Resolver.LookupIPAddr(ctx, asciiDomainName); err != nil {
				return cacheAndReturn(domainName, nil, false, errors.WithStack(err))
			}
			return cacheAndReturn(domainName, []*net.MX{{Host: asciiDomainName, Pref: 1}}, true, nil)
		}
		if mxRecords == nil {
			return cacheAndReturn(domainName, nil, false, errors.WithStack(err))
		}
	}
	// Check for "Null MX" record (https://tools.ietf.org/html/rfc7505).
	if len(mxRecords) == 1 {
		if mxRecords[0].Host == "." {
			return cacheAndReturn(domainName, nil, false, errNullMXRecord)
		}
		// 0.0.0.0 is not really a "Null MX" record, but some people apparently
		// have never heard of RFC7505 and configure it this way.
		if strings.HasPrefix(mxRecords[0].Host, "0.0.0.0") {
			return cacheAndReturn(domainName, nil, false, errNullMXRecord)
		}
	}
	// Purge records with non-ASCII characters. we have seen such records in
	// production, they are obviously products of human errors.
	for i := 0; i < len(mxRecords); {
		if isASCII(mxRecords[i].Host) {
			i++
			continue
		}
		copy(mxRecords[i:], mxRecords[i+1:])
		mxRecords = mxRecords[:len(mxRecords)-1]
	}
	// If there are no valid records left, then return an error.
	if len(mxRecords) == 0 {
		return cacheAndReturn(domainName, nil, false, errNoValidMXHosts)
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
	return cacheAndReturn(domainName, mxRecords, false, nil)
}

// SetDeterministicInTests sets rand to deterministic seed for testing, and is
// not Thread-Safe.
func SetDeterministicInTests() func() {
	randomizerMu.Lock()
	old := randomizer
	randomizer = rand.New(rand.NewSource(1))
	randomizerMu.Unlock()
	return func() {
		randomizerMu.Lock()
		randomizer = old
		randomizerMu.Unlock()
	}
}

// ResetCache clears the cache for use in tests, and is not Thread-Safe
func ResetCache() {
	lookupResultCache = collections.NewLRUCache(1000)
}

func shuffleMXRecords(mxRecords []*net.MX) []string {
	// Shuffle the hosts within the preference groups.
	var (
		mxHosts    []string
		groupBegin = 0
		groupEnd   = 0
		groupPref  uint16
	)
	for _, mxRecord := range mxRecords {
		// If a hostname has non-ASCII characters then ignore it, for it is
		// a kind of human error that we saw in production.
		if !isASCII(mxRecord.Host) {
			continue
		}
		// Just being overly cautious, so checking for empty values.
		if mxRecord.Host == "" {
			continue
		}
		// If it is the first valid record in the set, then allocate a slice
		// for MX hosts and put it there.
		if mxHosts == nil {
			mxHosts = make([]string, 0, len(mxRecords))
			mxHosts = append(mxHosts, mxRecord.Host)
			groupPref = mxRecord.Pref
			groupEnd = 1
			continue
		}
		// Put the next valid record to the slice.
		mxHosts = append(mxHosts, mxRecord.Host)
		// If the added host has the same preference as the first one in the
		// current group, then continue the MX record set traversal.
		if groupPref == mxRecord.Pref {
			groupEnd++
			continue
		}
		// After finding the end of the current preference group, shuffle it.
		if groupEnd-groupBegin > 1 {
			shuffleHosts(mxHosts[groupBegin:groupEnd])
		}
		// Set up the next preference group.
		groupBegin = groupEnd
		groupEnd++
		groupPref = mxRecord.Pref
	}
	// Shuffle the last preference group, if there is one.
	if groupEnd-groupBegin > 1 {
		shuffleHosts(mxHosts[groupBegin:groupEnd])
	}
	return mxHosts
}

func shuffleHosts(hosts []string) {
	randomizerMu.Lock()
	randomizer.Shuffle(len(hosts), func(i, j int) { hosts[i], hosts[j] = hosts[j], hosts[i] })
	randomizerMu.Unlock()
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
	mxRecords []*net.MX
	implicit  bool
	err       error
}

func cacheAndReturn(hostname string, mxRecords []*net.MX, implicit bool, err error) ([]*net.MX, bool, error) {
	lookupResultCache.AddWithTTL(hostname, lookupResult{mxRecords: mxRecords, implicit: implicit, err: err}, cacheTTL)
	return mxRecords, implicit, err
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
