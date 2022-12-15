package anonymize

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const anonym = "xxx"

var tokenSep = regexp.MustCompile(`\s|[,;]`)
var userSep = regexp.MustCompile("[._-]")
var adjacentSecrets = regexp.MustCompile(fmt.Sprintf(`%s(\s%s)+`, anonym, anonym))
const namesSep = `(\b|[!,.?:-]+)`
var namesRe = regexp.MustCompile(
	fmt.Sprintf(
		`(?i)%s%s%s`,
		namesSep,
		strings.Join(
			names,
			fmt.Sprintf(`%s|%s`, namesSep, namesSep),
		),
		namesSep,
	),
)

// Anonymize replace secret information with xxx.
func Anonymize(src string, secrets ...string) (string, error) {
	src = namesRe.ReplaceAllString(src, anonym)
	tokens := tokenize(secrets...)
	if len(tokens) == 0 {
		return src, nil
	}
	secret, err := or(tokens)
	if err != nil {
		return src, err
	}
	src = secret.ReplaceAllString(src, anonym)
	src = adjacentSecrets.ReplaceAllString(src, anonym)
	return src, nil
}

func tokenize(text ...string) (tokens []string) {
	tokenSet := map[string]interface{}{}
	for _, s := range text {
		for _, token := range tokenSep.Split(strings.ToLower(s), -1) {
			token = strings.Trim(token, "<>\" \n\t'")
			if strings.Contains(token, "@") {
				parts := strings.SplitN(token, "@", 2)
				tokenSet[parts[1]] = true
				for _, userPart := range userSep.Split(parts[0], 5) {
					if len(userPart) > 2 {
						tokenSet[userPart] = true
					}
				}
			} else if len(token) > 1 {
				tokenSet[token] = true
			}
		}
	}
	for token := range tokenSet {
		tokens = append(tokens, regexp.QuoteMeta(token))
	}
	sort.SliceStable(tokens, func(i, j int) bool {
		return len(tokens[i]) > len(tokens[j])
	})
	return tokens
}

func or(tokens []string) (*regexp.Regexp, error) {
	return regexp.Compile(fmt.Sprintf("(?i)%s", strings.Join(tokens, "|")))
}
