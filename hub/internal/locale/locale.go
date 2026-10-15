// Package locale picks the hub-side language (emails) from a request.
package locale

import "golang.org/x/text/language"

const Default = "en"

// Supported lists the languages the hub writes; the web client has the same set.
var Supported = []string{"en", "pt-BR"}

var matcher = language.NewMatcher([]language.Tag{language.English, language.BrazilianPortuguese})

// Match returns the supported locale closest to an Accept-Language header.
func Match(acceptLanguage string) string {
	tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil || len(tags) == 0 {
		return Default
	}
	_, index, confidence := matcher.Match(tags...)
	if confidence == language.No {
		return Default
	}
	return Supported[index]
}
