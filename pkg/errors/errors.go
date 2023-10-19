package errors

import (
	"errors"
	"fmt"
	"sort"

	"github.com/acorn-io/aml/pkg/token"
	"github.com/acorn-io/aml/pkg/value"
)

var (
	Join = errors.Join
	Is   = errors.Is
	As   = errors.As
)

// NewParserError creates an Error with the associated position and message.
func NewParserError(p token.Pos, format string, args ...interface{}) error {
	return &ParserError{
		Position: p,
		Format:   format,
		Args:     args,
	}
}

type ParserError struct {
	Position token.Pos
	Format   string
	Args     []interface{}
}

func (p *ParserError) Pos() value.Position {
	return value.Position{
		Filename: p.Position.Filename(),
		Offset:   p.Position.Offset(),
		Line:     p.Position.Line(),
		Column:   p.Position.Column(),
	}
}

func (p *ParserError) Error() string {
	return fmt.Sprintf("%s: %s", fmt.Sprintf(p.Format, p.Args...), p.Position)
}

func lessOrMore(isLess bool) int {
	if isLess {
		return -1
	}
	return 1
}

func comparePos(a, b token.Pos) int {
	if a.Filename() != b.Filename() {
		return lessOrMore(a.Filename() < b.Filename())
	}
	if a.Line() != b.Line() {
		return lessOrMore(a.Line() < b.Line())
	}
	if a.Column() != b.Column() {
		return lessOrMore(a.Column() < b.Column())
	}
	return 0
}

// SanitizeParserErrors sorts multiple errors and removes duplicates on a best effort basis.
// If err represents a single or no error, it returns the error as is.
func SanitizeParserErrors(err error) error {
	if errs, ok := err.(interface {
		Unwrap() []error
	}); ok {
		return sanitize(errs.Unwrap())
	}
	return err
}

func sanitize(errs []error) error {
	var perrs []*ParserError
	for _, err := range errs {
		if pe := (*ParserError)(nil); errors.As(err, &pe) {
			perrs = append(perrs, pe)
		} else {
			return errors.Join(errs...)
		}
	}
	return removeMultiples(perrs)
}

func removeMultiples(errs []*ParserError) error {
	var ret []error
	sort.Slice(errs, func(i, j int) bool {
		if c := comparePos(errs[i].Position, errs[j].Position); c != 0 {
			return c == -1
		}
		return errs[i].Error() < errs[j].Error()
	})

	var last *ParserError
	for _, e := range errs {
		if !approximateEqual(last, e) {
			ret = append(ret, e)
		}
		last = e
	}
	return errors.Join(ret...)
}

func approximateEqual(a, b *ParserError) bool {
	if a == nil || b == nil {
		return false
	}
	aPos := a.Position
	bPos := b.Position
	if aPos == token.NoPos || bPos == token.NoPos {
		return a.Error() == b.Error()
	}
	return aPos.Filename() == bPos.Filename() &&
		aPos.Line() == bPos.Line() &&
		aPos.Column() == bPos.Column()
}

func NewErrValueNotDefined(pos value.Position, key string) *ErrValueNotDefined {
	return &ErrValueNotDefined{
		Position: pos,
		Key:      key,
	}
}

type ErrValueNotDefined struct {
	Position value.Position
	Key      string
}

func (e *ErrValueNotDefined) Error() string {
	return fmt.Sprintf("value for key not defined: %s", e.Key)
}

func (e *ErrValueNotDefined) Pos() value.Position {
	return e.Position
}
