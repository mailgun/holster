/*
Copyright 2017 Mailgun Technologies Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
 Provides tools for signing and authenticating HTTP requests between web services

 An keyed-hash message authentication code (HMAC) is used to provide integrity and
 authenticity of a message between web services. The following elements are input
 into the HMAC. Only the items in bold are required to be passed in by the user, the
 other elements are either optional or build by httpsign for you.

 Each request element is delimited with the character `|` and each request element is
 preceded by its length. A simple example with only the required parameters:

  // Randomly generated number from a CSPRNG.
  shared_secret = '042DAD12E0BE4625AC0B2C3F7172DBA8'
  // Epoch time (number of seconds since January 1, 1970 UTC).
  timestamp     = '1330837567'
  // Randomly generated number from a CSPRNG.
  nonce         = '000102030405060708090a0b0c0d0e0f'
  // Request body
  request_body  = '{"hello": "world"}'
  // Optionally the HTTP Verb and HTTP Request URI.
  // Optionally an additional headers to sign.

  signature     = HMAC('042DAD12E0BE4625AC0B2C3F7172DBA8',
    '10|1330837567|32|000102030405060708090a0b0c0d0e0f|18|{"hello": "world"}')

 The timestamp, nonce, signature, and signature version are set as headers for the
 HTTP request to be signed. They are then verified on the receiving side by running the
 same algorithm and verifying that the signatures match.

 Note: By default the service can securely handle authenticating 5,000 requests per
 second. If you need to authenticate more, increase the capacity of the nonce
 cache when initializing the package.
*/
package httpsign

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/mailgun/holster"
	"github.com/mailgun/holster/random"
	"github.com/mailgun/metrics"
)

// Modify NonceCacheCapacity and NonceCacheTimeout if your service needs to
// authenticate more than 5,000 requests per second. For example, if you need
// to handle 10,000 requests per second and timeout after one minute,  you may
// want to set NonceCacheTimeout to 60 and NonceCacheCapacity to
// 10000 * cacheTimeout = 600000.
type Config struct {
	// KeyPath is a path to a file that contains the key to sign requests. If
	// it is an empty string then the key should be provided in `KeyBytes`.
	KeyPath string
	// KeyBytes is a key that is used by lemma to sign requests. Ignored if
	// `KeyPath` is not an empty string.
	KeyBytes []byte
	// List of headers to sign
	HeadersToSign []string
	// Include the http verb and uri in request
	SignVerbAndURI bool
	// Capacity of the nonce cache
	NonceCacheCapacity int
	// Nonce cache timeout
	NonceCacheTimeout int
	// Clock to use when signing
	Clock holster.Clock
	// Random Number Generator to use when signing
	Random random.RandomProvider
	// Toggle emitting metrics or not
	EmitStats bool
	// Hostname of statsd server
	StatsdHost string
	// Port of statsd server
	StatsdPort int
	// Prefix to prepend to metrics
	StatsdPrefix string
	// Default: X-Mailgun-Nonce
	NonceHeaderName string
	// Default: X-Mailgun-Timestamp
	TimestampHeaderName string
	// Default: X-Mailgun-Signature
	SignatureHeaderName string
	// Default: X-Mailgun-Signature-Version
	SignatureVersionHeaderName string
}

// Represents a service that can be used to sign and authenticate requests.
type Service struct {
	config         *Config
	nonceCache     *NonceCache
	randomProvider random.RandomProvider
	clock          holster.Clock
	secretKey      []byte
	metricsClient  metrics.Client
}

// Return a new Service. Config can not be nil. If you need control over
// setting time and random providers, use NewWithProviders.
func New(config *Config) (*Service, error) {
	holster.SetDefault(&config.Clock, &holster.SystemClock{})
	holster.SetDefault(&config.Random, &random.CSPRNG{})

	return NewWithProviders(
		config,
		config.Clock,
		config.Random,
	)
}

// Returns a new Service. Provides control over time and random providers.
func NewWithProviders(config *Config, clock holster.Clock,
	randomProvider random.RandomProvider) (*Service, error) {

	// config is required!
	if config == nil {
		return nil, fmt.Errorf("config is required.")
	}

	// set defaults if not set
	if config.NonceCacheCapacity < 1 {
		config.NonceCacheCapacity = CacheCapacity
	}
	if config.NonceCacheTimeout < 1 {
		config.NonceCacheTimeout = CacheTimeout
	}
	if config.NonceHeaderName == "" {
		config.NonceHeaderName = XMailgunNonce
	}
	if config.TimestampHeaderName == "" {
		config.TimestampHeaderName = XMailgunTimestamp
	}
	if config.SignatureHeaderName == "" {
		config.SignatureHeaderName = XMailgunSignature
	}
	if config.SignatureVersionHeaderName == "" {
		config.SignatureVersionHeaderName = XMailgunSignatureVersion
	}

	// setup metrics service
	metricsClient := metrics.NewNop()
	if config.EmitStats {
		// get hostname of box
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to obtain hostname: %v", err)
		}

		// build lemma prefix
		prefix := "lemma." + strings.Replace(hostname, ".", "_", -1)
		if config.StatsdPrefix != "" {
			prefix += "." + config.StatsdPrefix
		}

		// build metrics client
		hostport := fmt.Sprintf("%v:%v", config.StatsdHost, config.StatsdPort)
		metricsClient, err = metrics.NewWithOptions(hostport, prefix, metrics.Options{UseBuffering: true})
		if err != nil {
			return nil, err
		}
	}

	// Read in key from KeyPath or if not given, try getting them from KeyBytes.
	var keyBytes []byte
	var err error
	if config.KeyPath != "" {
		if keyBytes, err = readKeyFromDisk(config.KeyPath); err != nil {
			return nil, err
		}
	} else {
		if config.KeyBytes == nil {
			return nil, errors.New("no key bytes provided")
		}
		keyBytes = config.KeyBytes
	}

	// setup nonce cache
	ncache := NewNonceCache(config.NonceCacheCapacity, config.NonceCacheTimeout, clock)

	// return service
	return &Service{
		config:         config,
		nonceCache:     ncache,
		secretKey:      keyBytes,
		clock:          clock,
		randomProvider: randomProvider,
		metricsClient:  metricsClient,
	}, nil
}

// Signs a given HTTP request with signature, nonce, and timestamp.
func (s *Service) SignRequest(r *http.Request) error {
	if s.secretKey == nil {
		return fmt.Errorf("service not loaded with key.")
	}
	return s.SignRequestWithKey(r, s.secretKey)
}

// Signs a given HTTP request with signature, nonce, and timestamp. Signs the
// message with the passed in key not the one initialized with.
func (s *Service) SignRequestWithKey(r *http.Request, secretKey []byte) error {
	// extract request body bytes
	bodyBytes, err := readBody(r)
	if err != nil {
		return err
	}

	// extract any headers if requested
	headerValues, err := extractHeaderValues(r, s.config.HeadersToSign)
	if err != nil {
		return err
	}

	// get 128-bit random number from /dev/urandom and base16 encode it
	nonce, err := s.randomProvider.HexDigest(16)
	if err != nil {
		return fmt.Errorf("unable to get random : %v", err)
	}

	// get current timestamp
	timestamp := strconv.FormatInt(s.clock.Now().UTC().Unix(), 10)

	// compute the hmac and base16 encode it
	computedMAC := computeMAC(secretKey, s.config.SignVerbAndURI, r.Method, r.URL.RequestURI(),
		timestamp, nonce, bodyBytes, headerValues)
	signature := hex.EncodeToString(computedMAC)

	// set headers
	r.Header.Set(s.config.NonceHeaderName, nonce)
	r.Header.Set(s.config.TimestampHeaderName, timestamp)
	r.Header.Set(s.config.SignatureHeaderName, signature)
	r.Header.Set(s.config.SignatureVersionHeaderName, "2")

	// set the body bytes we read in to nil to hint to the gc to pick it up
	bodyBytes = nil

	return nil
}

// Authenticates HTTP request to ensure it was sent by an authorized sender.
func (s *Service) AuthenticateRequest(r *http.Request) error {
	if s.secretKey == nil {
		return fmt.Errorf("service not loaded with key.")
	}
	return s.AuthenticateRequestWithKey(r, s.secretKey)
}

// Authenticates HTTP request to ensure it was sent by an authorized sender.
// Checks message signature with the passed in key, not the one initialized with.
func (s *Service) AuthenticateRequestWithKey(r *http.Request, secretKey []byte) (err error) {
	// Emit a success or failure metric on return.
	defer func() {
		if err == nil {
			s.metricsClient.Inc("success", 1, 1)
		} else {
			s.metricsClient.Inc("failure", 1, 1)
		}
	}()

	// extract parameters
	signature := r.Header.Get(s.config.SignatureHeaderName)
	if signature == "" {
		return fmt.Errorf("header not found: %v", s.config.SignatureHeaderName)
	}
	nonce := r.Header.Get(s.config.NonceHeaderName)
	if nonce == "" {
		return fmt.Errorf("header not found: %v", s.config.NonceHeaderName)
	}
	timestamp := r.Header.Get(s.config.TimestampHeaderName)
	if timestamp == "" {
		return fmt.Errorf("header not found: %v", s.config.TimestampHeaderName)
	}

	// extract request body bytes
	bodyBytes, err := readBody(r)
	if err != nil {
		return err
	}

	// extract any headers if requested
	headerValues, err := extractHeaderValues(r, s.config.HeadersToSign)
	if err != nil {
		return err
	}

	// check the hmac
	isValid, err := checkMAC(secretKey, s.config.SignVerbAndURI, r.Method, r.URL.RequestURI(),
		timestamp, nonce, bodyBytes, headerValues, signature)
	if !isValid {
		return err
	}

	// check timestamp
	isValid, err = s.CheckTimestamp(timestamp)
	if !isValid {
		return err
	}

	// check to see if we have seen nonce before
	inCache := s.nonceCache.InCache(nonce)
	if inCache {
		return fmt.Errorf("nonce already in cache: %v", nonce)
	}

	// set the body bytes we read in to nil to hint to the gc to pick it up
	bodyBytes = nil

	return nil
}

// Parses a timestamp header and returns true if the timestamp is neither older than the TTL or is from the future.
func (s *Service) CheckTimestamp(timestampHeader string) (bool, error) {
	// convert unix timestamp string into time struct
	timestamp, err := strconv.ParseInt(timestampHeader, 10, 0)
	if err != nil {
		return false, fmt.Errorf("unable to parse %v: %v", s.config.TimestampHeaderName, timestampHeader)
	}

	now := s.clock.Now().UTC().Unix()

	// if timestamp is from the future, it's invalid
	if timestamp >= now+MaxSkewSec {
		return false, fmt.Errorf("timestamp header from the future; now: %v; %v: %v; difference: %v",
			now, s.config.TimestampHeaderName, timestamp, timestamp-now)
	}

	// if the timestamp is older than ttl - skew, it's invalid
	if timestamp <= now-int64(s.nonceCache.cacheTTL-MaxSkewSec) {
		return false, fmt.Errorf("timestamp header too old; now: %v; %v: %v; difference: %v",
			now, s.config.TimestampHeaderName, timestamp, now-timestamp)
	}

	return true, nil
}

func computeMAC(secretKey []byte, signVerbAndUri bool, httpVerb string, httpResourceUri string,
	timestamp string, nonce string, body []byte, headerValues []string) []byte {

	// use hmac-sha256
	mac := hmac.New(sha256.New, secretKey)

	// required parameters (timestamp, nonce, body)
	mac.Write([]byte(fmt.Sprintf("%v|", len(timestamp))))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(fmt.Sprintf("|%v|", len(nonce))))
	mac.Write([]byte(nonce))
	mac.Write([]byte(fmt.Sprintf("|%v|", len(body))))
	mac.Write(body)

	// optional parameters (httpVerb, httpResourceUri)
	if signVerbAndUri {
		mac.Write([]byte(fmt.Sprintf("|%v|", len(httpVerb))))
		mac.Write([]byte(httpVerb))
		mac.Write([]byte(fmt.Sprintf("|%v|", len(httpResourceUri))))
		mac.Write([]byte(httpResourceUri))
	}

	// optional parameters (headers)
	for _, headerValue := range headerValues {
		mac.Write([]byte(fmt.Sprintf("|%v|", len(headerValue))))
		mac.Write([]byte(headerValue))
	}

	return mac.Sum(nil)
}

func checkMAC(secretKey []byte, signVerbAndUri bool, httpVerb string, httpResourceUri string,
	timestamp string, nonce string, body []byte, headerValues []string, signature string) (bool, error) {

	// the hmac we get is a hexdigest (string representation of hex values)
	// which needs to be decoded before before we can use it
	expectedMAC, err := hex.DecodeString(signature)
	if err != nil {
		return false, err
	}

	// compute the hmac
	computedMAC := computeMAC(secretKey, signVerbAndUri, httpVerb, httpResourceUri, timestamp, nonce, body, headerValues)

	// constant time compare
	isEqual := hmac.Equal(expectedMAC, computedMAC)
	if !isEqual {
		return false, fmt.Errorf("signature header value %v does not match computed value", expectedMAC)
	}

	return true, nil
}

// readBody will read in the request body, return a byte slice, and also restore it
// within the *http.Request so it can be read later. Tries to be smart and initialize
// a buffer based off content-length.
//
// See for more details:
// https://github.com/golang/go/blob/release-branch.go1.5/src/io/ioutil/ioutil.go#L16-L43
func readBody(r *http.Request) (b []byte, err error) {
	// if we have no body, like a GET request, set it to ""
	if r.Body == nil {
		return []byte(""), nil
	}

	// try and be smart and pre-allocate buffer
	var n int64 = bytes.MinRead
	if r.ContentLength > int64(n) {
		n = r.ContentLength
	}
	buf := bytes.NewBuffer(make([]byte, 0, n))

	// If the buffer overflows, we will get bytes.ErrTooLarge.
	// Return that as an error. Any other panic remains.
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if panicErr, ok := e.(error); ok && panicErr == bytes.ErrTooLarge {
			err = panicErr
		} else {
			panic(e)
		}
	}()
	_, err = buf.ReadFrom(r.Body)

	// restore the body back to the request
	b = buf.Bytes()
	r.Body = ioutil.NopCloser(bytes.NewReader(b))

	return b, err
}

func extractHeaderValues(r *http.Request, headerNames []string) ([]string, error) {
	if len(headerNames) < 1 {
		return nil, nil
	}

	headerValues := make([]string, len(headerNames))
	for i, headerName := range headerNames {
		_, ok := r.Header[headerName]
		if !ok {
			return nil, fmt.Errorf("header %v not found in request.", headerName)
		}
		headerValues[i] = r.Header.Get(headerName)
	}

	return headerValues, nil
}

func readKeyFromDisk(keypath string) ([]byte, error) {
	// load key from disk
	keyBytes, err := ioutil.ReadFile(keypath)
	if err != nil {
		return nil, err
	}

	// strip newline (\n or 0x0a) if it's at the end
	keyBytes = bytes.TrimSuffix(keyBytes, []byte("\n"))

	return keyBytes, nil
}
