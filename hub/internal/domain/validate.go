package domain

import (
	"bufio"
	_ "embed"
	"net/mail"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	UsernameMinLen     = 3
	UsernameMaxLen     = 32
	PasswordMinLen     = 8
	PasswordMaxLen     = 256
	DisplayNameMaxLen  = 32
	CustomStatusMaxLen = 128
	EmailMaxLen        = 254
)

// NormalizeEmail trims surrounding space; comparison is case-insensitive in
// the database.
func NormalizeEmail(email string) string { return strings.TrimSpace(email) }

func ValidateUsername(field, username string) *FieldViolation {
	switch n := len(username); {
	case n == 0:
		return &FieldViolation{field, ReasonFieldRequired}
	case n < UsernameMinLen:
		return &FieldViolation{field, ReasonFieldTooShort}
	case n > UsernameMaxLen:
		return &FieldViolation{field, ReasonFieldTooLong}
	}
	for _, r := range username {
		if r >= utf8.RuneSelf || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.') {
			return &FieldViolation{field, ReasonFieldInvalidFormat}
		}
	}
	return nil
}

func ValidateEmail(field, email string) *FieldViolation {
	if email == "" {
		return &FieldViolation{field, ReasonFieldRequired}
	}
	if len(email) > EmailMaxLen {
		return &FieldViolation{field, ReasonFieldTooLong}
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || addr.Name != "" {
		return &FieldViolation{field, ReasonFieldInvalidFormat}
	}
	return nil
}

func ValidatePassword(field, password string) *FieldViolation {
	switch n := utf8.RuneCountInString(password); {
	case n == 0:
		return &FieldViolation{field, ReasonFieldRequired}
	case n < PasswordMinLen:
		return &FieldViolation{field, ReasonFieldTooShort}
	case len(password) > PasswordMaxLen:
		return &FieldViolation{field, ReasonFieldTooLong}
	}
	if isCommonPassword(password) {
		return &FieldViolation{field, ReasonFieldTooCommon}
	}
	return nil
}

// NormalizeDisplayName trims surrounding space.
func NormalizeDisplayName(name string) string { return strings.TrimSpace(name) }

func ValidateDisplayName(field, name string) *FieldViolation {
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return &FieldViolation{field, ReasonFieldRequired}
	case n > DisplayNameMaxLen:
		return &FieldViolation{field, ReasonFieldTooLong}
	}
	if hasControl(name) {
		return &FieldViolation{field, ReasonFieldInvalidFormat}
	}
	return nil
}

func ValidateCustomStatus(field, status string) *FieldViolation {
	if utf8.RuneCountInString(status) > CustomStatusMaxLen {
		return &FieldViolation{field, ReasonFieldTooLong}
	}
	if hasControl(status) {
		return &FieldViolation{field, ReasonFieldInvalidFormat}
	}
	return nil
}

// Violations collects the non-nil results of validators into one error.
func Violations(results ...*FieldViolation) error {
	var fields []FieldViolation
	for _, r := range results {
		if r != nil {
			fields = append(fields, *r)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return Invalid(fields...)
}

func hasControl(s string) bool {
	return strings.ContainsFunc(s, isControl)
}

func isControl(r rune) bool { return unicode.IsControl(r) }

//go:embed common_passwords.txt
var commonPasswordsFile string

var commonPasswords = sync.OnceValue(func() map[string]struct{} {
	set := map[string]struct{}{}
	sc := bufio.NewScanner(strings.NewReader(commonPasswordsFile))
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			set[line] = struct{}{}
		}
	}
	return set
})

func isCommonPassword(password string) bool {
	_, ok := commonPasswords()[strings.ToLower(password)]
	return ok
}
