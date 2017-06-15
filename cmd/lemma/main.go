package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/pbkdf2"

	"github.com/mailgun/holster/random"
	"github.com/mailgun/holster/secret"
)

type EncodedCiphertext struct {
	KeySalt      []byte `json:"key_salt,omitempty"`
	KeyIter      int    `json:"key_iter_count,omitempty"`
	KeyAlgorithm string `json:"key_algorithm,omitempty"`

	CiphertextNonce []byte `json:"ciphertext_nonce"`
	Ciphertext      []byte `json:"ciphertext"`
	CipherAlgorithm string `json:"cipher_algorithm"`
}

func main() {
	mode, keypath, itercount, inputpath, outputpath := parseArguments(os.Args)

	switch mode {
	case "encrypt":
		encrypt(keypath, itercount, inputpath, outputpath)
	case "decrypt":
		decrypt(keypath, inputpath, outputpath)
	}
}

func usage() {
	fmt.Printf(`
lemma is a tool that uses authenticated encryption (Salsa20 with Poly1305) to encrypt/decrypt small files on disk.

Usage:
    lemma command [flags]

The commands are:
    encrypt     encrypt a file on disk
    decrypt     decrypt a file on disk

The flags are:
    in          path to file to be read in
    out         path to file to be written out
    keypath     path to base64-encoded 32-byte key on disk, if no path is provided, a passphrase will be used
    itercount   if a passphrase is used, iteration count for PBKDF#2, the default is 524288
`)
}

func parseArguments(args []string) (mode string, keypath string, itercount int, input string, output string) {
	if len(args) < 2 {
		usage()
		os.Exit(255)
	}

	mode = args[1]

	fs := flag.NewFlagSet("fs", flag.ExitOnError)
	in := fs.String("in", "", "path to file to be read in")
	out := fs.String("out", "", "path to file to be written out")
	key := fs.String("keypath", "", "path to base64-encoded 32-byte key on disk, if no path is provided, a passphrase will be used")
	iter := fs.Int("itercount", 524288, "if a passphrase is used, iteration count for pbkdf#2")

	err := fs.Parse(args[2:])
	if err != nil {
		fmt.Printf("lemmacmd: unable to parse flags: %v\n", err)
	}

	if mode != "encrypt" && mode != "decrypt" {
		fmt.Printf("lemmacmd: mode must be encrypt or decrypt, not: %q\n", mode)
	}
	if *in == "" {
		fmt.Printf("lemmacmd: input path required\n")
	}
	if *out == "" {
		fmt.Printf("lemmacmd: output path required\n")
	}
	if *in == "" || *out == "" {
		usage()
		os.Exit(255)
	}

	return mode, *key, *iter, *in, *out
}

func encrypt(keypath string, itercount int, inputpath string, outputpath string) {
	salt, err := (&random.CSPRNG{}).Bytes(16)
	if err != nil {
		fmt.Printf("lemmacmd: unable to generate salt: %v\n", err)
		os.Exit(255)
	}

	key, isPass, err := generateKey(keypath, salt, itercount)
	if err != nil {
		fmt.Printf("lemmacmd: unable to generate or read in key: %v\n", err)
		os.Exit(255)
	}

	plaintextBytes, err := ioutil.ReadFile(inputpath)
	if err != nil {
		fmt.Printf("lemmacmd: unable to read plaintext file %q: %v\n", inputpath, err)
		os.Exit(255)
	}

	sealedData, err := secret.Seal(plaintextBytes, key)
	if err != nil {
		fmt.Printf("lemmacmd: unable to seal plaintext: %v\n", err)
		os.Exit(255)
	}

	err = writeCiphertext(salt, itercount, isPass, sealedData, outputpath)
	if err != nil {
		fmt.Printf("lemmacmd: unable to write sealed data to disk: %v\n", err)
		os.Exit(255)
	}
}

func decrypt(keypath string, inputpath string, outputpath string) {
	keySalt, keyIter, sealedData, err := readCiphertext(inputpath)
	if err != nil {
		fmt.Printf("lemmacmd: unable to read ciphertext file: %v\n", err)
		os.Exit(255)
	}

	key, _, err := generateKey(keypath, keySalt, keyIter)
	if err != nil {
		fmt.Printf("lemmacmd: unable to build key: %v\n", err)
		os.Exit(255)
	}

	plaintextBytes, err := secret.Open(sealedData, key)
	if err != nil {
		fmt.Printf("lemmacmd: unable to open ciphertext: %v\n", err)
		os.Exit(255)
	}

	err = ioutil.WriteFile(outputpath, plaintextBytes, 0600)
	if err != nil {
		fmt.Printf("lemmacmd: unable to write plaintext bytes to disk: %v\n", err)
		os.Exit(255)
	}
}

// Returns key from keypath or uses salt + passphrase to generate key using a KDF.
func generateKey(keypath string, salt []byte, keyiter int) (key *[secret.SecretKeyLength]byte, isPass bool, err error) {
	// if a keypath is given try and use it
	if keypath != "" {
		key, err := secret.ReadKeyFromDisk(keypath)
		if err != nil {
			return nil, false, fmt.Errorf("unable to build secret service: %v", err)
		}
		return key, false, nil
	}

	// otherwise read in a passphrase from disk and use that, remember to reset your terminal afterwards
	var passphrase string
	fmt.Printf("Passphrase: ")
	fmt.Scanln(&passphrase)

	// derive key and return it
	keySlice := pbkdf2.Key([]byte(passphrase), salt, keyiter, 32, sha256.New)
	keyBytes, err := secret.KeySliceToArray(keySlice)
	if err != nil {
		return nil, true, err
	}

	return keyBytes, true, nil
}

// Encodes all data needed to decrypt message into a JSON string and writes it to disk.
func writeCiphertext(salt []byte, keyiter int, isPass bool, sealed secret.SealedData, filename string) error {
	// fill in the ciphertext fields
	ec := EncodedCiphertext{
		CiphertextNonce: sealed.NonceBytes(),
		Ciphertext:      sealed.CiphertextBytes(),
		CipherAlgorithm: "salsa20_poly1305",
	}

	// if we used a passphrase, also set the passphrase fields
	if isPass == true {
		ec.KeySalt = salt
		ec.KeyIter = keyiter
		ec.KeyAlgorithm = "pbkdf#2"
	}

	// marshal encoded ciphertext into a json string
	b, err := json.MarshalIndent(ec, "", "   ")
	if err != nil {
		return err
	}

	// write to disk with read only permissions for the current user
	return ioutil.WriteFile(filename, b, 0600)
}

// Reads in encoded ciphertext from disk and breaks into component parts.
func readCiphertext(filename string) ([]byte, int, *secret.SealedBytes, error) {
	plaintextBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, 0, nil, err
	}

	var ec EncodedCiphertext
	err = json.Unmarshal(plaintextBytes, &ec)
	if err != nil {
		return nil, 0, nil, err
	}

	sealedBytes := &secret.SealedBytes{
		Ciphertext: ec.Ciphertext,
		Nonce:      ec.CiphertextNonce,
	}

	return ec.KeySalt, ec.KeyIter, sealedBytes, nil
}
