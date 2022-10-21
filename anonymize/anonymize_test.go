package anonymize

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type AnonymizeSuite struct {
	suite.Suite
}

func TestAnonymizeSuite(t *testing.T) {
	suite.Run(t, new(AnonymizeSuite))
}

func (s *AnonymizeSuite) TestAnonymizeWithNoSecretsReturnsUnchangedString() {
	anonimized, err := Anonymize("Hello Dear User", "")
	s.Nil(err)
	s.Equal("Hello Dear User", anonimized)
	anonimized, err = Anonymize("Hello Dear User")
	s.Nil(err)
	s.Equal("Hello Dear User", anonimized)
}

func (s *AnonymizeSuite) TestAnonymizeWithNoSecretsInStringReturnsUnchangedString() {
	anonimized, err := Anonymize("Hello Dear User", "John")
	s.Nil(err)
	s.Equal("Hello Dear User", anonimized)
}

func (s *AnonymizeSuite) TestAnonymizeEscapesSecrets() {
	anonimized, err := Anonymize(
		"Hello Dear User", `\s`)
	s.Nil(err)
	s.Equal("Hello Dear User", anonimized)
}

func (s *AnonymizeSuite) TestAnonymizeSquashesAdjacentSecrets() {
	anonimized, err := Anonymize(
		"Hello Иван Иванов ivan ivanov foo.bar",
		`"Иван Иванов" <ivan.ivanov@foo.bar>`)
	s.Nil(err)
	s.Equal(anonimized, "Hello xxx")
}
