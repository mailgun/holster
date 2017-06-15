# lemma

lemma is a command-line utility that uses lemma to provide authenticated
 symmetric cryptography for small files on disk.

Download: [Latest](https://github.com/mailgun/lemma/releases)

**Usage**

```
Usage:
    lemmacmd command [flags]

The commands are:
    encrypt     encrypt a file on disk
    decrypt     decrypt a file on disk

The flags are: 
    in          path to file to be read in
    out         path to file to be written out
    keypath     path to base64-encoded 32-byte key on disk, if no path is given, a passphrase is used
    itercount   if a passphrase is used, iteration count for PBKDF#2, the default is 524288
```

**Example**

```
lemmacmd encrypt -in foo.txt -out foo.txt.enc
lemmacmd decrypt -in foo.txt.enc -out foo.txt
```

**Performance**

The following benchmarks were run to calculate wall-clock time to encrypt files of various sizes. The benchmarks were run on a machine with the following specs: 2x Intel Xeon E5-2680 2.8Ghz and 32GB RAM.

|      | 1 MB  | 10 MB | 100 MB |
|------|-------|-------|--------|
| Time | 0.98s | 1.33s | 4.85s  |


**Technical Details**

* Can be used with either a randomly generated key on disk or a passpharse.
* When used with a passphrase, the key derivation function (KDF) is HMAC-SHA-256 based PBKDF#2 with a randomly generated 128-bit salt and 524,288 iterations (tunable).
* The symmetric cipher used is Salsa20 with Poly1305 as the message authentication code (MAC) from the Networking and Cryptography (NaCl) library.
