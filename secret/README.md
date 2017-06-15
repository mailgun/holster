# Secret
Secret is a library for encrypting and decrypting authenticated messages.
Metrics are built in and can be emitted to check for anomalous behavior.

[NaCl](http://nacl.cr.yp.to/) is the underlying secret-key authenticated encryption
library used. NaCl uses Salsa20 and Poly1305 as its cipher and MAC respectively.

### Usage
Demonstrates encryption and decryption of a message using a common key and nonce
```go
import (
	"github.com/mailgun/holster/random"
	"github.com/mailgun/holster/secret"
)
// Create a new randomly generated key
key, err := secret.NewKey()

// Store the key on disk for retrieval later
fd, err := os.Create("/tmp/test-secret.key")
if err != nil {
    panic(err)
}
fd.Write([]byte(secret.KeyToEncodedString(key)))
fd.Close()

// Read base64 encoded key in from disk
s, err := secret.New(&secret.Config{KeyPath: "/tmp/test-secret.key"})
if err != nil {
    panic(err)
}

// Encrypt the message using the key provided and a randomly generated nonce
sealed, err := s.Seal([]byte("hello, world"))
if err != nil {
    panic(err)
}

// Optionally base64 encode them and store them somewhere (like in a database)
cipherText := base64.StdEncoding.EncodeToString(sealed.CiphertextBytes())
nonce := base64.StdEncoding.EncodeToString(sealed.NonceBytes())
fmt.Printf("Ciphertext: %s, Nonce: %s\n", cipherText, nonce)

// Decrypt the message
msg, err := s.Open(&secret.SealedBytes{
    Ciphertext: sealed.CiphertextBytes(),
    Nonce:      sealed.NonceBytes(),
})
fmt.Printf("Decrypted Plaintext: %s\n", string(msg))

// Output: Ciphertext: Pg7RWodWBNwVViVfySz1RTaaVCOo5oJn1E7jWg==, Nonce: AAECAwQFBgcICQoLDA0ODxAREhMUFRYX
// Decrypted Plaintext: hello, world

```

### Convenience Functions
Demonstrates encryption and decryption of a message using a common key and nonce using the package level functions
```go
// Create a or load a new randomly generated key
key, _ := secret.NewKey()

// Encrypt the message using the key provided and a randomly generated nonce
sealed, err := secret.Seal([]byte("hello, world"), key)
if err != nil {
    panic(err)
}

// Optionally base64 encode them and store them somewhere (like in a database)
cipherText := base64.StdEncoding.EncodeToString(sealed.CiphertextBytes())
nonce := base64.StdEncoding.EncodeToString(sealed.NonceBytes())
fmt.Printf("Ciphertext: %s, Nonce: %s\n", cipherText, nonce)

// Decrypt the message
msg, err := secret.Open(&secret.SealedBytes{
    Ciphertext: sealed.CiphertextBytes(),
    Nonce:      sealed.NonceBytes(),
}, key)
fmt.Printf("Decrypted Plaintext: %s\n", string(msg))

// Output: Ciphertext: Pg7RWodWBNwVViVfySz1RTaaVCOo5oJn1E7jWg==, Nonce: AAECAwQFBgcICQoLDA0ODxAREhMUFRYX
// Decrypted Plaintext: hello, world

```

### Key Generation

```go
import (
	"github.com/mailgun/holster/random"
	"github.com/mailgun/holster/secret"
)
// For consistency during tests, DO NOT USE IN PRODUCTION
secret.RandomProvider = &random.FakeRNG{}

// Create a new randomly generated key
keyBytes, _ := secret.NewKey()
fmt.Printf("New Key: %s\n", secret.KeyToEncodedString(keyBytes))

// given key bytes, return an base64 encoded key
encodedKey := secret.KeyToEncodedString(keyBytes)
// given a base64 encoded key, return key bytes
decodedKey, _ := secret.EncodedStringToKey(encodedKey)

fmt.Printf("Key and Encoded/Decoded key are equal: %t", bytes.Equal((*keyBytes)[:], decodedKey[:]))

// Output: New Key: AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=
// Key and Encoded/Decoded key are equal: true

```

### Testing
Inject a consistent random number generator when writting tests
```go
// For consistency during tests
secret.RandomProvider = &random.FakeRNG{}

// Create a key that will always equal "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="
key, _ := secret.NewKey()
````
