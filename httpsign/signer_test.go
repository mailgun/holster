package httpsign

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mailgun/holster/v4/clock"
	"github.com/stretchr/testify/require"
)

var (
	testKey = []byte("042DAD12E0BE4625AC0B2C3F7172DBA8")
)

func TestSignRequest(t *testing.T) {
	ctx := context.Background()
	clock.Freeze(clock.Unix(1330837567, 0))
	defer clock.Unfreeze()
	randomPrv = &fakeRandom{}

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
		s, err := New(
			&Config{
				KeyBytes:           testKey,
				HeadersToSign:      headerNames,
				SignVerbAndURI:     tt.inSignVerbAndUri,
				NonceCacheCapacity: defaultCacheCapacity,
				NonceCacheTimeout:  defaultCacheTimeout,
			},
		)
		if err != nil {
			t.Errorf("[%v] Got unexpected error from NewWithHeadersAndProviders: %v", i, err)
		}

		body := strings.NewReader(tt.inRequestBody)
		request, err := http.NewRequestWithContext(ctx, tt.inHttpVerb, tt.inRequestUri, body)
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
		if g, w := request.Header.Get(XMailgunNonce), tt.outNonce; g != w {
			t.Errorf("[%v] Nonce from SignRequest: Got %s, Want %s", i, g, w)
		}

		// check timestamp
		if g, w := request.Header.Get(XMailgunTimestamp), tt.outTimestamp; g != w {
			t.Errorf("[%v] Timestamp from SignRequest: Got %s, Want %s", i, g, w)
		}

		// check signature
		if g, w := request.Header.Get(XMailgunSignature), tt.outSignature; g != w {
			t.Errorf("[%v] Signature from SignRequest: Got %s, Want %s", i, g, w)
		}

		// check signature version
		if g, w := request.Header.Get(XMailgunSignatureVersion), tt.outSignatureVersion; g != w {
			t.Errorf("[%v] SignatureVersion from SignRequest: Got %s, Want %s", i, g, w)
		}
	}
}

func TestAuthenticateRequest(t *testing.T) {
	ctx := context.Background()
	clock.Freeze(clock.Unix(1330837567, 0))
	defer clock.Unfreeze()
	randomPrv = &fakeRandom{}

	s, err := New(
		&Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     false,
			NonceCacheCapacity: defaultCacheCapacity,
			NonceCacheTimeout:  defaultCacheTimeout,
		},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.VerifyRequest(r)

		// check
		if err != nil {
			t.Errorf("VerifyRequest failed to authenticate a correctly signed request. It returned this error: %v", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequestWithContext(ctx, "POST", ts.URL, body)
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
	res, err := client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
	err = res.Body.Close()
	require.NoError(t, err)
}

func TestAuthenticateRequestWithHeaders(t *testing.T) {
	ctx := context.Background()
	clock.Freeze(clock.Unix(1330837567, 0))
	defer clock.Unfreeze()
	randomPrv = &fakeRandom{}

	s, err := New(
		&Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{"X-Mailgun-Custom-Header"},
			SignVerbAndURI:     false,
			NonceCacheCapacity: defaultCacheCapacity,
			NonceCacheTimeout:  defaultCacheTimeout,
		},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.VerifyRequest(r)

		// check
		if err != nil {
			t.Errorf("VerifyRequest failed to authenticate a correctly signed request. It returned this error: %v", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequestWithContext(ctx, "POST", ts.URL, body)
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
	res, err := client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
	err = res.Body.Close()
	require.NoError(t, err)
}

func TestAuthenticateRequestWithKey(t *testing.T) {
	ctx := context.Background()
	clock.Freeze(clock.Unix(1330837567, 0))
	defer clock.Unfreeze()
	randomPrv = &fakeRandom{}

	s, err := New(
		&Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     false,
			NonceCacheCapacity: defaultCacheCapacity,
			NonceCacheTimeout:  defaultCacheTimeout,
		},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.VerifyRequestWithKey(r, []byte("abc"))

		// check
		if err != nil {
			t.Errorf("VerifyRequest failed to authenticate a correctly signed request. It returned this error: %v", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequestWithContext(ctx, "POST", ts.URL, body)
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
	res, err := client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
	err = res.Body.Close()
	require.NoError(t, err)
}

func TestAuthenticateRequestWithVerbAndUri(t *testing.T) {
	ctx := context.Background()
	clock.Freeze(clock.Unix(1330837567, 0))
	defer clock.Unfreeze()
	randomPrv = &fakeRandom{}

	s, err := New(
		&Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     true,
			NonceCacheCapacity: defaultCacheCapacity,
			NonceCacheTimeout:  defaultCacheTimeout,
		},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.VerifyRequestWithKey(r, []byte("abc"))

		// check
		if err != nil {
			t.Errorf("VerifyRequest failed to authenticate a correctly signed request. It returned this error: %v", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequestWithContext(ctx, "POST", ts.URL, body)
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
	res, err := client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
	err = res.Body.Close()
	require.NoError(t, err)
}

func TestAuthenticateRequestForged(t *testing.T) {
	ctx := context.Background()
	clock.Freeze(clock.Unix(1330837567, 0))
	defer clock.Unfreeze()
	randomPrv = &fakeRandom{}

	s, err := New(
		&Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     false,
			NonceCacheCapacity: defaultCacheCapacity,
			NonceCacheTimeout:  defaultCacheTimeout,
		},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.VerifyRequest(r)

		// check
		if err == nil {
			t.Error("VerifyRequest failed to authenticate a correctly signed request. It returned this error:", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequestWithContext(ctx, "POST", ts.URL, body)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	// try and forget signing the request
	request.Header.Set(XMailgunNonce, "000102030405060708090a0b0c0d0e0f")
	request.Header.Set(XMailgunTimestamp, "1330837567")
	request.Header.Set(XMailgunSignature, "0000000000000000000000000000000000000000000000000000000000000000")

	// submit request
	client := &http.Client{}
	res, err := client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
	err = res.Body.Close()
	require.NoError(t, err)
}

func TestAuthenticateRequestMissingHeaders(t *testing.T) {
	ctx := context.Background()
	clock.Freeze(clock.Unix(1330837567, 0))
	defer clock.Unfreeze()
	randomPrv = &fakeRandom{}

	s, err := New(
		&Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     false,
			NonceCacheCapacity: defaultCacheCapacity,
			NonceCacheTimeout:  defaultCacheTimeout,
		},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// http server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// test
		err := s.VerifyRequest(r)

		// check
		if err == nil {
			t.Error("VerifyRequest failed to authenticate a correctly signed request. It returned this error:", err)
		}

		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	// setup request to test with
	body := strings.NewReader(`{"hello": "world"}`)
	request, err := http.NewRequestWithContext(ctx, "POST", ts.URL, body)
	if err != nil {
		t.Errorf("Got unexpected error from http.NewRequest: %v", err)
	}

	// try and forget signing the request
	request.Header.Set(XMailgunNonce, "000102030405060708090a0b0c0d0e0f")
	request.Header.Set(XMailgunTimestamp, "1330837567")

	// submit request
	client := &http.Client{}
	res, err := client.Do(request)
	if err != nil {
		t.Errorf("Got unexpected error from client.Do: %v", err)
	}
	err = res.Body.Close()
	require.NoError(t, err)
}

func TestCheckTimestamp(t *testing.T) {
	clock.Freeze(clock.Unix(1330837567, 0))
	defer clock.Unfreeze()
	randomPrv = &fakeRandom{}

	s, err := New(
		&Config{
			KeyBytes:           testKey,
			HeadersToSign:      []string{},
			SignVerbAndURI:     false,
			NonceCacheCapacity: 100,
			NonceCacheTimeout:  30,
		},
	)
	if err != nil {
		t.Errorf("Got unexpected error from NewWithHeadersAndProviders: %v", err)
	}

	// test goldilocks (perfect) timestamp
	time0 := time.Unix(1330837567, 0)
	timestamp0 := strconv.FormatInt(time0.Unix(), 10)
	isValid0, err := s.checkTimestamp(timestamp0)
	if !isValid0 {
		t.Errorf("Got unexpected error from checkTimestamp: %v", err)
	}

	// test old timestamp
	time1 := time.Unix(1330837517, 0)
	timestamp1 := strconv.FormatInt(time1.Unix(), 10)
	isValid1, err := s.checkTimestamp(timestamp1)
	if isValid1 {
		t.Errorf("Got unexpected error from checkTimestamp: %v", err)
	}

	// test timestamp from the future
	time2 := time.Unix(1330837587, 0)
	timestamp2 := strconv.FormatInt(time2.Unix(), 10)
	isValid2, err := s.checkTimestamp(timestamp2)
	if isValid2 {
		t.Errorf("Got unexpected error from checkTimestamp: %v", err)
	}
}
