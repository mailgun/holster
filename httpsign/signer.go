/*
Package httpsign provides tools for signing and authenticating HTTP requests between
web services. See README.md for more details.
*/
package httpsign

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/mailgun/holster/v4/clock"
)

const (
	XMailgunSignature        = "X-Mailgun-Signature"
	XMailgunSignatureVersion = "X-Mailgun-Signature-Version"
	XMailgunNonce            = "X-Mailgun-Nonce"
	XMailgunTimestamp        = "X-Mailgun-Timestamp"

	defaultCacheTimeout  = 100                        // 100 sec
	defaultCacheCapacity = 5000 * defaultCacheTimeout // 5,000 msg/sec * 100 sec = 500,000 elements

	maxSkewSec = 5 // 5 sec
)

var (
	randomPrv randomProvider = &realRandom{}
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

	HeadersToSign  []string // list of headers to sign
	SignVerbAndURI bool     // include the http verb and uri in request

	NonceCacheCapacity int // capacity of the nonce cache
	NonceCacheTimeout  int // nonce cache timeout

	EmitStats    bool   // toggle emitting metrics or not
	StatsdHost   string // hostname of statsd server
	StatsdPort   int    // port of statsd server
	StatsdPrefix string // prefix to prepend to metrics

	NonceHeaderName            string // default: X-Mailgun-Nonce
	TimestampHeaderName        string // default: X-Mailgun-Timestamp
	SignatureHeaderName        string // default: X-Mailgun-Signature
	SignatureVersionHeaderName string // default: X-Mailgun-Signature-Version
}

// Represents an entity that can be used to sign and authenticate requests.
type Signer struct {
	config     *Config
	nonceCache *nonceCache
	secretKey  []byte
}

// Return a new Signer. Config can not be nil. If you need control over
// setting time and random providers, use NewWithProviders.
func New(config *Config) (*Signer, error) {
	// config is required!
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	// set defaults if not set
	if config.NonceCacheCapacity < 1 {
		config.NonceCacheCapacity = defaultCacheCapacity
	}
	if config.NonceCacheTimeout < 1 {
		config.NonceCacheTimeout = defaultCacheTimeout
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

	// Commented out this code because it does effectively nothing.
	// // setup metrics service
	// if config.EmitStats {
	// 	// get hostname of box
	// 	hostname, err := os.Hostname()
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to obtain hostname: %v", err)
	// 	}
	//
	// 	// build lemma prefix
	// 	prefix := "lemma." + strings.ReplaceAll(hostname, ".", "_")
	// 	if config.StatsdPrefix != "" {
	// 		prefix += "." + config.StatsdPrefix
	// 	}
	// }

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
	ncache, err := newNonceCache(config.NonceCacheCapacity, config.NonceCacheTimeout)
	if err != nil {
		return nil, err
	}

	return &Signer{
		config:     config,
		nonceCache: ncache,
		secretKey:  keyBytes,
	}, nil
}

// Signs a given HTTP request with signature, nonce, and timestamp.
func (s *Signer) SignRequest(r *http.Request) error {
	if s.secretKey == nil {
		return fmt.Errorf("service not loaded with key")
	}
	return s.SignRequestWithKey(r, s.secretKey)
}

// Signs a given HTTP request with signature, nonce, and timestamp. Signs the
// message with the passed in key not the one initialized with.
func (s *Signer) SignRequestWithKey(r *http.Request, secretKey []byte) error {
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
	nonce, err := randomPrv.hexDigest(16)
	if err != nil {
		return fmt.Errorf("unable to get random : %v", err)
	}

	// get current timestamp
	timestamp := strconv.FormatInt(clock.Now().Unix(), 10)

	// compute the hmac and base16 encode it
	computedMAC := computeMAC(secretKey, s.config.SignVerbAndURI, r.Method, r.URL.RequestURI(),
		timestamp, nonce, bodyBytes, headerValues)
	signature := hex.EncodeToString(computedMAC)

	// set headers
	r.Header.Set(s.config.NonceHeaderName, nonce)
	r.Header.Set(s.config.TimestampHeaderName, timestamp)
	r.Header.Set(s.config.SignatureHeaderName, signature)
	r.Header.Set(s.config.SignatureVersionHeaderName, "2")
	return nil
}

// VerifyRequest checks that an HTTP request was sent by an authorized sender.
func (s *Signer) VerifyRequest(r *http.Request) error {
	if s.secretKey == nil {
		return fmt.Errorf("service not loaded with key")
	}
	return s.VerifyRequestWithKey(r, s.secretKey)
}

// VerifyRequestWithKey checks that an HTTP request was sent by an authorized
// sender. The check is performed against the passed in key.
func (s *Signer) VerifyRequestWithKey(r *http.Request, secretKey []byte) (err error) {
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
	isValid, err = s.checkTimestamp(timestamp)
	if !isValid {
		return err
	}

	// check to see if we have seen nonce before
	inCache, err := s.nonceCache.inCache(nonce)
	if err != nil {
		return fmt.Errorf("while reading nonce cache for value %v: %w", nonce, err)
	}
	if inCache {
		return fmt.Errorf("nonce already in cache: %v", nonce)
	}

	return nil
}

func (s *Signer) checkTimestamp(timestampHeader string) (bool, error) {
	// convert unix timestamp string into time struct
	timestamp, err := strconv.ParseInt(timestampHeader, 10, 0)
	if err != nil {
		return false, fmt.Errorf("unable to parse %v: %v", s.config.TimestampHeaderName, timestampHeader)
	}

	now := clock.Now().Unix()

	// if timestamp is from the future, it's invalid
	if timestamp >= now+maxSkewSec {
		return false, fmt.Errorf("timestamp header from the future; now: %v; %v: %v; difference: %v",
			now, s.config.TimestampHeaderName, timestamp, timestamp-now)
	}

	// if the timestamp is older than ttl - skew, it's invalid
	if timestamp <= now-int64(s.nonceCache.cacheTTL-maxSkewSec) {
		return false, fmt.Errorf("timestamp header too old; now: %v; %v: %v; difference: %v",
			now, s.config.TimestampHeaderName, timestamp, now-timestamp)
	}

	return true, nil
}

func computeMAC(secretKey []byte, signVerbAndURI bool, httpVerb string, httpResourceURI string,
	timestamp string, nonce string, body []byte, headerValues []string) []byte {

	// use hmac-sha256
	mac := hmac.New(sha256.New, secretKey)

	// required parameters (timestamp, nonce, body)
	fmt.Fprintf(mac, "%v|", len(timestamp))
	mac.Write([]byte(timestamp))
	fmt.Fprintf(mac, "|%v|", len(nonce))
	mac.Write([]byte(nonce))
	fmt.Fprintf(mac, "|%v|", len(body))
	mac.Write(body)

	// optional parameters (httpVerb, httpResourceUri)
	if signVerbAndURI {
		fmt.Fprintf(mac, "|%v|", len(httpVerb))
		mac.Write([]byte(httpVerb))
		fmt.Fprintf(mac, "|%v|", len(httpResourceURI))
		mac.Write([]byte(httpResourceURI))
	}

	// optional parameters (headers)
	for _, headerValue := range headerValues {
		fmt.Fprintf(mac, "|%v|", len(headerValue))
		mac.Write([]byte(headerValue))
	}

	return mac.Sum(nil)
}

func checkMAC(secretKey []byte, signVerbAndURI bool, httpVerb string, httpResourceURI string,
	timestamp string, nonce string, body []byte, headerValues []string, signature string) (bool, error) {

	// the hmac we get is a hexdigest (string representation of hex values)
	// which needs to be decoded before before we can use it
	expectedMAC, err := hex.DecodeString(signature)
	if err != nil {
		return false, err
	}

	// compute the hmac
	computedMAC := computeMAC(secretKey, signVerbAndURI, httpVerb, httpResourceURI, timestamp, nonce, body, headerValues)

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
	r.Body = io.NopCloser(bytes.NewReader(b))

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
			return nil, fmt.Errorf("header %s not found in request", headerName)
		}
		headerValues[i] = r.Header.Get(headerName)
	}

	return headerValues, nil
}

func readKeyFromDisk(keypath string) ([]byte, error) {
	// load key from disk
	keyBytes, err := os.ReadFile(keypath)
	if err != nil {
		return nil, err
	}

	// strip newline (\n or 0x0a) if it's at the end
	keyBytes = bytes.TrimSuffix(keyBytes, []byte("\n"))

	return keyBytes, nil
}
