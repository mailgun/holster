# httpsign

library for signing and authenticating HTTP requests between web services.

### Overview

An keyed-hash message authentication code (HMAC) is used to provide integrity and
authenticity of a message between web services. The following elements are input
into the HMAC. Only the items in bold are required to be passed in by the user, the
other elements are either optional or build by httpsign for you.

* **Shared secret**, a randomly generated number from a CSPRNG.
* Timestamp in epoch time (number of seconds since January 1, 1970 UTC).
* Nonce, a randomly generated number from a CSPRNG.
* **Request body.**
* Optionally the HTTP Verb and HTTP Request URI.
* Optionally an additional headers to sign.

Each request element is delimited with the character `|` and each request element is
preceded by its length. A simple example with only the required parameters:

```
shared_secret = '042DAD12E0BE4625AC0B2C3F7172DBA8'
timestamp     = '1330837567'
nonce         = '000102030405060708090a0b0c0d0e0f'
request_body  = '{"hello": "world"}'

signature     = HMAC('042DAD12E0BE4625AC0B2C3F7172DBA8',
   '10|1330837567|32|000102030405060708090a0b0c0d0e0f|18|{"hello": "world"}')
```

The timestamp, nonce, signature, and signature version are set as headers for the
HTTP request to be signed. They are then verified on the receiving side by running the
same algorithm and verifying that the signatures match.

Note: By default the service can securely handle authenticating 5,000 requests per
second. If you need to authenticate more, increase the capacity of the nonce 
cache when initializing the package.

### Examples

_Signing a Request_

```go
import (
    "github.com/mailgun/holster/httpsign"
    "github.com/mailgun/holster"
    "github.com/mailgun/holster/httpsign"
    "github.com/mailgun/holster/random"
    "github.com/mailgun/holster/secret"
)
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
```

_Authenticating a Request_

```go
import (
    "github.com/mailgun/holster"
    "github.com/mailgun/holster/httpsign"
    "github.com/mailgun/holster/random"
    "github.com/mailgun/holster/secret"
)

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
```
