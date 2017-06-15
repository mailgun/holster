package httpsign_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mailgun/holster"
	"github.com/mailgun/holster/httpsign"
	"github.com/mailgun/holster/random"
	"github.com/mailgun/holster/secret"
)

var testKey = []byte("042DAD12E0BE4625AC0B2C3F7172DBA8")
var _ = fmt.Printf // for testing

func TestSignRequest(t *testing.T) {
	var signtests = []struct {
		inHeadersToSign     map[string]string
		inSignVerbAndUri    bool
		inHttpVerb          string
		inRequestUri        string
		inRequestBody       string
		outNonce            string
		outTimestamp        string
		outSignature        string
		outSignatureVersion string
	}{
		{nil, false, "POST", "", `{"hello": "world"}`,
			"000102030405060708090a0b0c0d0e0f", "1330837567", "5a42c21371e8b3a2b50ca1ad72869dc7882aa83a6a2fb13db1bf108d92c6f05f", "2"},
		{map[string]string{"X-Mailgun-Foo": "bar"}, false, "POST", "", `{"hello": "world"}`,
			"000102030405060708090a0b0c0d0e0f", "1330837567", "d3bee620f172eb16a3bb30fb6b44b7193fdf04391d44c392d080efe71250753d", "2"},
		{nil, true, "POST", "/path?key=value&key=value#fragment", `{"hello": "world"}`,
			"000102030405060708090a0b0c0d0e0f", "1330837567", "6341720191526856d8940d01611394bfc72a04bc6b8fe90f976ff4eb976ec016", "2"},
	}

	for i, tt := range signtests {
		headerNames := make([]string, 0, len(tt.inHeadersToSign))
		for k := range tt.inHeadersToSign {
			headerNames = append(headerNames, k)
		}
		// setup
		s, err := httpsign.NewWithProviders(
			&httpsign.Config{
				KeyBytes:           testKey,
				HeadersToSign:      headerNames,
				SignVerbAndURI:     tt.inSignVerbAndUri,
				NonceCacheCapacity: httpsign.CacheCapacity,
				NonceCacheTimeout:  httpsign.CacheTimeout,
			},
			&holster.FrozenClock{CurrentTime: time.Unix(1330837567, 0)},
			&random.FakeRNG{},
		)
		if err != nil {
			t.Errorf("[%v] Got unexpected error from NewWithHeadersAndProviders: %v", i, err)
		}

		body := strings.NewReader(tt.inRequestBody)
		request, err := http.NewRequest(tt.inHttpVerb, tt.inRequestUri, body)
		if err != nil {
			t.Errorf("[%v] Got unexpected error from http.NewRequest: %v", i, err)
		}
		if len(tt.inHeadersToSign) > 0 {
			for k, v := range tt.inHeadersToSign {
				request.Header.Set(k, v)
			}
		}

		// test signing a request
		err = s.SignRequest(request)
		if err != nil {
			t.Errorf("[%v] Got unexpected error from SignRequest: %v", i, err)
		}

		// check nonce
		if g, w := request.Header.Get(httpsign.XMailgunNonce), tt.outNonce; g != w {
			t.Errorf("[%v] Nonce from SignRequest: Got %s, Want %s", i, g, w)
		}

		// check timestamp
		if g, w := request.Header.Get(httpsign.XMailgunTimestamp), tt.outTimestamp; g != w {
			t.Errorf("[%v] Timestamp from SignRequest: Got %s, Want %s", i, g, w)
		}

		// check signature
		if g, w := request.Header.Get(httpsign.XMailgunSignature), tt.outSignature; g != w {
			t.Errorf("[%v] Signature from SignRequest: Got %s, Want %s", i, g, w)
		}

		// check signature version
		if g, w := request.Header.Get(httpsign.XMailgunSignatureVersion), tt.outSignatureVersion; g != w {
			t.Errorf("[%v] SignatureVersion from SignRequest: Got %s, Want %s", i, g, w)
		}
	}
}

func TestAuthenticateRequest(t *testing.T) {
	// setup
	s, err := httpsign.NewWithProviders(
		&httpsign.Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     false,
			NonceCacheCapacity: httpsign.CacheCapacity,
			NonceCacheTimeout:  httpsign.CacheTimeout,
		},
		&holster.FrozenClock{CurrentTime: time.Unix(1330837567, 0)},
		&random.FakeRNG{},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.AuthenticateRequest(r)

		// check
		if err != nil {
			t.Errorf("AuthenticateRequest failed to authenticate a correctly signed request. It returned this error: %v", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequest("POST", ts.URL, body)
	if err != nil {
		t.Errorf("Got unexpected error from http.NewRequest: %v", err)
	}

	// sign request
	err = s.SignRequest(request)
	if err != nil {
		t.Errorf("Got unexpected error from SignRequest: %v", err)
	}

	// submit request
	client := &http.Client{}
	_, err = client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
}

func TestAuthenticateRequestWithHeaders(t *testing.T) {
	// setup
	s, err := httpsign.NewWithProviders(
		&httpsign.Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{"X-Mailgun-Custom-Header"},
			SignVerbAndURI:     false,
			NonceCacheCapacity: httpsign.CacheCapacity,
			NonceCacheTimeout:  httpsign.CacheTimeout,
		},
		&holster.FrozenClock{CurrentTime: time.Unix(1330837567, 0)},
		&random.FakeRNG{},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.AuthenticateRequest(r)

		// check
		if err != nil {
			t.Errorf("AuthenticateRequest failed to authenticate a correctly signed request. It returned this error: %v", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequest("POST", ts.URL, body)
	if err != nil {
		t.Errorf("Got unexpected error from http.NewRequest: %v", err)
	}
	request.Header.Set("X-Mailgun-Custom-Header", "bar")

	// sign request
	err = s.SignRequest(request)
	if err != nil {
		t.Errorf("Got unexpected error from SignRequest: %v", err)
	}

	// submit request
	client := &http.Client{}
	_, err = client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
}

func TestAuthenticateRequestWithKey(t *testing.T) {
	// setup
	s, err := httpsign.NewWithProviders(
		&httpsign.Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     false,
			NonceCacheCapacity: httpsign.CacheCapacity,
			NonceCacheTimeout:  httpsign.CacheTimeout,
		},
		&holster.FrozenClock{CurrentTime: time.Unix(1330837567, 0)},
		&random.FakeRNG{},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.AuthenticateRequestWithKey(r, []byte("abc"))

		// check
		if err != nil {
			t.Errorf("AuthenticateRequest failed to authenticate a correctly signed request. It returned this error: %v", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequest("POST", ts.URL, body)
	if err != nil {
		t.Errorf("Got unexpected error from http.NewRequest: %v", err)
	}

	// sign request
	err = s.SignRequestWithKey(request, []byte("abc"))
	if err != nil {
		t.Errorf("Got unexpected error from SignRequest: %v", err)
	}

	// submit request
	client := &http.Client{}
	_, err = client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
}

func TestAuthenticateRequestWithVerbAndUri(t *testing.T) {
	// setup
	s, err := httpsign.NewWithProviders(
		&httpsign.Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     true,
			NonceCacheCapacity: httpsign.CacheCapacity,
			NonceCacheTimeout:  httpsign.CacheTimeout,
		},
		&holster.FrozenClock{CurrentTime: time.Unix(1330837567, 0)},
		&random.FakeRNG{},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.AuthenticateRequestWithKey(r, []byte("abc"))

		// check
		if err != nil {
			t.Errorf("AuthenticateRequest failed to authenticate a correctly signed request. It returned this error: %v", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequest("POST", ts.URL, body)
	if err != nil {
		t.Errorf("Got unexpected error from http.NewRequest: %v", err)
	}

	// sign request
	err = s.SignRequestWithKey(request, []byte("abc"))
	if err != nil {
		t.Errorf("Got unexpected error from SignRequest: %v", err)
	}

	// submit request
	client := &http.Client{}
	_, err = client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
}

func TestAuthenticateRequestForged(t *testing.T) {
	// setup
	s, err := httpsign.NewWithProviders(
		&httpsign.Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     false,
			NonceCacheCapacity: httpsign.CacheCapacity,
			NonceCacheTimeout:  httpsign.CacheTimeout,
		},
		&holster.FrozenClock{CurrentTime: time.Unix(1330837567, 0)},
		&random.FakeRNG{},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.AuthenticateRequest(r)

		// check
		if err == nil {
			t.Error("AuthenticateRequest failed to authenticate a correctly signed request. It returned this error:", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequest("POST", ts.URL, body)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	// try and forget signing the request
	request.Header.Set(httpsign.XMailgunNonce, "000102030405060708090a0b0c0d0e0f")
	request.Header.Set(httpsign.XMailgunTimestamp, "1330837567")
	request.Header.Set(httpsign.XMailgunSignature, "0000000000000000000000000000000000000000000000000000000000000000")

	// submit request
	client := &http.Client{}
	_, err = client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
}

func TestAuthenticateRequestMissingHeaders(t *testing.T) {
	// setup
	s, err := httpsign.NewWithProviders(
		&httpsign.Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     false,
			NonceCacheCapacity: httpsign.CacheCapacity,
			NonceCacheTimeout:  httpsign.CacheTimeout,
		},
		&holster.FrozenClock{CurrentTime: time.Unix(1330837567, 0)},
		&random.FakeRNG{},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.AuthenticateRequest(r)

		// check
		if err == nil {
			t.Error("AuthenticateRequest failed to authenticate a correctly signed request. It returned this error:", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequest("POST", ts.URL, body)
	if err != nil {
		t.Errorf("Got unexpected error from http.NewRequest: %v", err)
	}

	// try and forget signing the request
	request.Header.Set(httpsign.XMailgunNonce, "000102030405060708090a0b0c0d0e0f")
	request.Header.Set(httpsign.XMailgunTimestamp, "1330837567")

	// submit request
	client := &http.Client{}
	_, err = client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
}

func TestCheckTimestamp(t *testing.T) {
	// setup
	s, err := httpsign.NewWithProviders(
		&httpsign.Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     false,
			NonceCacheCapacity: 100,
			NonceCacheTimeout:  30,
		},
		&holster.FrozenClock{CurrentTime: time.Unix(1330837567, 0)},
		&random.FakeRNG{},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// test goldilocks (perfect) timestamp
	time0 := time.Unix(1330837567, 0)
	timestamp0 := strconv.FormatInt(time0.Unix(), 10)
	isValid0, err := s.CheckTimestamp(timestamp0)
	if !isValid0 {
		t.Errorf("Got unexpected error from checkTimestamp: %v", err)
	}

	// test old timestamp
	time1 := time.Unix(1330837517, 0)
	timestamp1 := strconv.FormatInt(time1.Unix(), 10)
	isValid1, err := s.CheckTimestamp(timestamp1)
	if isValid1 {
		t.Errorf("Got unexpected error from checkTimestamp: %v", err)
	}

	// test timestamp from the future
	time2 := time.Unix(1330837587, 0)
	timestamp2 := strconv.FormatInt(time2.Unix(), 10)
	isValid2, err := s.CheckTimestamp(timestamp2)
	if isValid2 {
		t.Errorf("Got unexpected error from checkTimestamp: %v", err)
	}
}

func ExampleService_SignRequest() {
	// For consistency during tests, OMIT THIS LINE IN PRODUCTION
	secret.RandomProvider = &random.FakeRNG{}

	// Create a new randomly generated key
	key, err := secret.NewKey()
	// Store the key on disk for retrieval later
	fd, err := os.Create("/tmp/test-secret.key")
	if err != nil {
		panic(err)
	}
	fd.Write([]byte(secret.KeyToEncodedString(key)))
	fd.Close()

	auths, err := httpsign.New(&httpsign.Config{
		// Our pre-generated shared key
		KeyPath: "/tmp/test-secret.key",
		// Optionally include headers in the signed request
		HeadersToSign: []string{"X-Mailgun-Header"},
		// Optionally include the HTTP Verb and URI in the signed request
		SignVerbAndURI: true,
		// For consistency during tests, OMIT THESE 2 LINES IN PRODUCTION
		Clock:  &holster.FrozenClock{CurrentTime: time.Unix(1330837567, 0)},
		Random: &random.FakeRNG{},
	})
	if err != nil {
		panic(err)
	}

	// Build new request
	r, _ := http.NewRequest("POST", "", strings.NewReader(`{"hello":"world"}`))
	// Add our custom header that is included in the signature
	r.Header.Set("X-Mailgun-Header", "nyan-cat")

	// Sign the request
	err = auths.SignRequest(r)
	if err != nil {
		panic(err)
	}

	// Preform the request
	// client := &http.Client{}
	// response, _ := client.Do(r)

	fmt.Printf("%s: %s\n", httpsign.XMailgunNonce, r.Header.Get(httpsign.XMailgunNonce))
	fmt.Printf("%s: %s\n", httpsign.XMailgunTimestamp, r.Header.Get(httpsign.XMailgunTimestamp))
	fmt.Printf("%s: %s\n", httpsign.XMailgunSignature, r.Header.Get(httpsign.XMailgunSignature))
	fmt.Printf("%s: %s\n", httpsign.XMailgunSignatureVersion, r.Header.Get(httpsign.XMailgunSignatureVersion))

	// Output: X-Mailgun-Nonce: 000102030405060708090a0b0c0d0e0f
	// X-Mailgun-Timestamp: 1330837567
	// X-Mailgun-Signature: 33f589de065a81b671c9728e7c6b6fecfb94324cb10472f33dc1f78b2a9e4fee
	// X-Mailgun-Signature-Version: 2
}

func ExampleService_AuthenticateRequest() {
	// For consistency during tests, OMIT THIS LINE IN PRODUCTION
	secret.RandomProvider = &random.FakeRNG{}

	// Create a new randomly generated key
	key, err := secret.NewKey()
	// Store the key on disk for retrieval later
	fd, err := os.Create("/tmp/test-secret.key")
	if err != nil {
		panic(err)
	}
	fd.Write([]byte(secret.KeyToEncodedString(key)))
	fd.Close()

	// When authenticating a request, the config must match that of the signing code
	auths, err := httpsign.New(&httpsign.Config{
		// Our pre-generated shared key
		KeyPath: "/tmp/test-secret.key",
		// Include headers in the signed request
		HeadersToSign: []string{"X-Mailgun-Header"},
		// Include the HTTP Verb and URI in the signed request
		SignVerbAndURI: true,
		// For consistency during tests, OMIT THESE 2 LINES IN PRODUCTION
		Clock:  &holster.FrozenClock{CurrentTime: time.Unix(1330837567, 0)},
		Random: &random.FakeRNG{},
	})
	if err != nil {
		panic(err)
	}

	// Pretend we received a new signed request
	r, _ := http.NewRequest("POST", "", strings.NewReader(`{"hello":"world"}`))
	// Add our custom header that is included in the signature
	r.Header.Set("X-Mailgun-Header", "nyan-cat")

	// These are the fields set by the client signing the request
	r.Header.Set("X-Mailgun-Nonce", "000102030405060708090a0b0c0d0e0f")
	r.Header.Set("X-Mailgun-Timestamp", "1330837567")
	r.Header.Set("X-Mailgun-Signature", "33f589de065a81b671c9728e7c6b6fecfb94324cb10472f33dc1f78b2a9e4fee")
	r.Header.Set("X-Mailgun-Signature-Version", "2")

	// Verify the request
	err = auths.AuthenticateRequest(r)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Request Verified\n")

	// Output: Request Verified
}
