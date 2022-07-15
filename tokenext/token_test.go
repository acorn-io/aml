package tokenext

import (
	"testing"

	"cuelang.org/go/cue/token"
)

func TestKeyword(t *testing.T) {
	if !IsKeyword(ELSE) {
		t.Fatalf("invalid")
	}
	if !IsKeyword(ELSEIF) {
		t.Fatalf("invalid")
	}
}

func TestLookup(t *testing.T) {
	if Lookup("else") != ELSE {
		t.Fatalf("invalid")
	}
	if Lookup("elif") != ELSEIF {
		t.Fatalf("invalid")
	}
}

func TestIota(t *testing.T) {
	if ELSE-20 != token.NULL {
		t.Fatalf("invalid token number %d != %d", ELSE, token.NULL)
	}
	if ELSEIF-1 != ELSE {
		t.Fatalf("invalid token number %d", ELSEIF)
	}
}
