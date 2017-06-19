package httpsign

// 5 sec
const MaxSkewSec = 5

// 100 sec
const CacheTimeout = 100

// 5,000 msg/sec * 100 sec = 500,000 elements
const CacheCapacity = 5000 * CacheTimeout

const XMailgunSignature = "X-Mailgun-Signature"
const XMailgunSignatureVersion = "X-Mailgun-Signature-Version"
const XMailgunNonce = "X-Mailgun-Nonce"
const XMailgunTimestamp = "X-Mailgun-Timestamp"
