package ast

import (
	"unicode/utf8"

	"github.com/acorn-io/aml/pkg/token"
)

func isAllowedCharacter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z'
}

func isAllowedDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

// IsValidIdent reports whether str is a valid identifier.
func IsValidIdent(ident string) bool {
	if ident == "" {
		return false
	}

	consumed := false
	if !consumed {
		if r, _ := utf8.DecodeRuneInString(ident); isAllowedDigit(r) {
			return false
		}
	}

	if ident == "$" {
		return true
	}

	for i, r := range ident {
		if isAllowedCharacter(r) || isAllowedDigit(r) || (r == '_' && i > 0) {
			continue
		}
		return false
	}

	return ident != "match" && token.Lookup(ident) == token.IDENT
}
