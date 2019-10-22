package slice

import "strings"

// ContainsString checks if a given slice of strings contains the provided string.
// If a modifier func is provided, it is called with the slice item before the comparation.
//      haystack := []string{"one", "Two", "Three"}
//	if slice.ContainsString(haystack, "two", strings.ToLower) {
//		// Do thing
// 	}
func ContainsString(s string, slice []string, modifier func(s string) string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
		if modifier != nil && modifier(item) == s {
			return true
		}
	}
	return false
}

// ContainsStringEqualFold checks if a given slice of strings contains the provided string
// as ignore the cases.
//
//  haystack := []string{"aa", "bb", "Cc"}
//	if slice.ContainsStringEqualFold(haystack, "cC") {
//		// Do thing
// 	}
func ContainsStringEqualFold(s string, slice []string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}
