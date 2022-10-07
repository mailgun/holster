package anonymize

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/mailgun/holster/errors"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"
	"regexp"
	"sort"
	"strings"
)

var tokenSep = regexp.MustCompile(`\s|[,;]`)
var userSep = regexp.MustCompile("[._-]")
var adjacentSecrets = regexp.MustCompile(`xxx(\sxxx)+`)
var names []string

type Config struct {
	namesFilename string
}

type Option func(c *Config)

func NamesFilename(namesFilename string) Option {
	return func(c *Config) {
		c.namesFilename = namesFilename
	}
}

func LoadNames(options ...Option) error {
	config := Config{namesFilename: "names.txt"}
	for _, option := range options {
		option(&config)
	}
	f, err := os.Open(config.namesFilename)
	if err != nil {
		return errors.Wrapf(err, "fail to open file with names %s", config.namesFilename)
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		names = append(names, strings.ToLower(scanner.Text()))
	}
	return errors.Wrapf(scanner.Err(), "fail to scan file with names")
}

// Anonymize replace secret information with xxx.
func Anonymize(src string, secrets ...string) (string, error) {
	tokens := tokenize(secrets...)
	if len(tokens) == 0 {
		return src, nil
	}
	secret, err := or(tokens)
	if err != nil {
		return src, err
	}
	s := secret.ReplaceAllString(src, "xxx")
	s = replaceNames(s)
	s = adjacentSecrets.ReplaceAllString(s, "xxx")
	return s, nil
}

func replaceNames(s string) string {
	if len(names) == 0 {
		return s
	}
	var lowTrimmedWords []string
	var trimmedWords []string
	words := strings.Split(s, " ")
	for i, word := range words {
		trimmedWords = append(trimmedWords, strings.Trim(word, ","))
		lowTrimmedWords = append(lowTrimmedWords, strings.ToLower(trimmedWords[i]))
	}
	ss, _ := json.Marshal(words)
	fmt.Printf("words %s\n", ss)
	ss, _ = json.Marshal(trimmedWords)
	fmt.Printf("trimmed words %s\n", ss)
	ss, _ = json.Marshal(lowTrimmedWords)
	fmt.Printf("low trimmed words %s\n", ss)

	for i, lowTrimmedWord := range lowTrimmedWords {
		for _, name := range names {
			if name == strings.Trim(lowTrimmedWord, ",") {
				capitalized := isCapitalized(trimmedWords[i])
				upperCased := isUpperCased(trimmedWords[i])
				if capitalized || upperCased {
					words[i] = strings.ReplaceAll(words[i], trimmedWords[i], "xxx")
					if i > 0 && len(trimmedWords[i-1]) > 1 {
						prevCapitalized := isCapitalized(trimmedWords[i-1])
						prevUpperCased := isUpperCased(trimmedWords[i-1])
						bothCapitalized := capitalized && prevCapitalized
						bothUpperCased := upperCased && prevUpperCased
						if bothCapitalized || bothUpperCased {
							words[i-1] = strings.ReplaceAll(words[i-1], trimmedWords[i-1], "xxx")
						}
					}
					if i < len(words)-1 && len(trimmedWords[i+1]) > 1 {
						nextCapitalized := isCapitalized(trimmedWords[i+1])
						nextUpperCased := isUpperCased(trimmedWords[i+1])
						bothCapitalized := capitalized && nextCapitalized
						bothUpperCased := upperCased && nextUpperCased
						fmt.Printf("for found name %s i %v is cap %v is upper %v\n", name, i, capitalized, upperCased)
						if bothCapitalized || bothUpperCased {
							words[i+1] = strings.ReplaceAll(words[i+1], trimmedWords[i+1], "xxx")
						}
					}
				}
			}
		}
	}
	return strings.Join(words, " ")
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

func isCapitalized(s string) bool {
	fmt.Printf("is capitalized %v '%s' '%s'\n",
		len(s) > 1 && cases.Title(language.Und, cases.NoLower).String(s) == s,
		s,
		cases.Title(language.Und, cases.NoLower).String(s),
	)
	capitalized := cases.Title(language.Und, cases.NoLower).String(s)
	return len(s) > 1 && capitalized == s && !isUpperCased(s)
}

func isUpperCased(s string) bool {
	return len(s) > 1 && strings.ToUpper(s) == s
}
