package holster

import (
	"crypto/rand"
	"fmt"
	"strings"
)

const NumericRunes = "0123456789"
const AlphaRunes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Return a random string made up of characters passed
func RandomRunes(prefix string, length int, runes ...string) string {
	chars := strings.Join(runes, "")
	var bytes = make([]byte, length)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = chars[b%byte(len(chars))]
	}
	return prefix + string(bytes)
}

// Return a random string of alpha characters
func RandomAlpha(prefix string, length int) string {
	return RandomRunes(prefix, length, AlphaRunes)
}

// Return a random string of alpha and numeric characters
func RandomString(prefix string, length int) string {
	return RandomRunes(prefix, length, AlphaRunes, NumericRunes)
}

// Given a list of strings, return one of the strings randomly
func RandomItem(items ...string) string {
	var bytes = make([]byte, 1)
	rand.Read(bytes)
	return items[bytes[0]%byte(len(items))]
}

// Return a random domain name in the form "randomAlpha.net"
func RandomDomainName() string {
	return fmt.Sprintf("%s.%s",
		RandomAlpha("", 14),
		RandomItem("net", "com", "org", "io", "gov"))
}
