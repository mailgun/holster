package anonymize

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jdkato/prose/v2"
	"github.com/mailgun/holster/v4/errors"
)

const anonym = "xxx"

var tokenSep = regexp.MustCompile(`\s|[,;]`)
var userSep = regexp.MustCompile("[._-]")
var adjacentSecrets = regexp.MustCompile(fmt.Sprintf(`%s(\s%s)+`, anonym, anonym))

// Anonymize replace secret information with xxx.
func Anonymize(src string, secrets ...string) (string, error) {
	s := src
	//s, err := replaceNames(src)
	//if err != nil {
	//	return src, errors.Wrapf(err, "fail to replace names in src %s", src)
	//}
	tokens := tokenize(secrets...)
	if len(tokens) == 0 {
		return s, nil
	}
	secret, err := or(tokens)
	if err != nil {
		return s, err
	}
	s = secret.ReplaceAllString(s, anonym)
	s = adjacentSecrets.ReplaceAllString(s, anonym)
	return s, nil
}

func replaceNames(s string) (string, error) {
	doc, err := prose.NewDocument(s)
	if err != nil {
		return s, errors.Wrapf(err, "fail to parse string %s", s)
	}
	for _, ent := range doc.Entities() {
		if ent.Label == "PERSON" {
			s = strings.ReplaceAll(s, ent.Text, anonym)
		}
	}
	return s, nil
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
