package errors

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/acorn-io/aml/pkg/token"
	"github.com/acorn-io/aml/pkg/value"
)

var (
	Join = errors.Join
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

type ErrEval struct {
	Position value.Position
	Err      error
}

func NewErrEval(pos value.Position, err error) error {
	if err == nil {
		return nil
	}
	return &ErrEval{
		Position: pos,
		Err:      err,
	}
}

func (e *ErrEval) Unwrap() error {
	return e.Err
}

func printPath(pos []value.Position) string {
	buf := strings.Builder{}
	end := pos[len(pos)-1]
	last := pos[len(pos)-1]
	for i := len(pos) - 2; i >= 0; i-- {
		next := pos[i]
		if next == end {
			break
		} else if last == next {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteString("->")
		}
		if last.Filename != next.Filename {
			buf.WriteString(next.Filename)
		}
		buf.WriteString(fmt.Sprintf("%d:%d", next.Line, next.Column))
	}

	return buf.String()
}

func (e *ErrEval) Error() string {
	var (
		pos  = []value.Position{e.Position}
		last = e
	)

	var cur error = e
	for cur != nil {
		cur = errors.Unwrap(cur)
		if e, ok := cur.(*ErrEval); ok {
			pos = append(pos, e.Position)
			last = e
		}
	}

	backtrace := printPath(pos)
	if len(backtrace) > 0 {
		return fmt.Sprintf("%s: %s (backtrace %s)", last.Err.Error(), last.Position, printPath(pos))
	}
	return fmt.Sprintf("%s: %s", last.Err.Error(), last.Position)
}
