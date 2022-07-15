package tokenext

import (
	"cuelang.org/go/cue/token"
)

type Token int

// The list of tokens.
const (
	ELSE = token.NULL + 20
)

// Lookup maps an identifier to its keyword token or IDENT (if not a keyword).
//
func Lookup(ident string) token.Token {
	switch ident {
	case "else":
		return ELSE
	default:
		return token.Lookup(ident)
	}
}

// Predicates

func IsKeyword(t token.Token) bool {
	return t == ELSE || t.IsKeyword()
}
