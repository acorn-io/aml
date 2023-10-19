package value

import (
	"errors"
	"fmt"
	"strings"
)

type ErrPosition struct {
	Position Position
	Err      error
}

func NewErrPosition(pos Position, err error) error {
	if err == nil {
		return nil
	}
	return &ErrPosition{
		Position: pos,
		Err:      err,
	}
}

func (e *ErrPosition) Pos() Position {
	return e.Position
}

func (e *ErrPosition) Unwrap() error {
	return e.Err
}

func (e *ErrPosition) Error() string {
	var (
		pos  []Position
		last = e
	)

	if e.Position != NoPosition {
		pos = append(pos, e.Position)
	}

	var cur error = e
	for cur != nil {
		next := errors.Unwrap(cur)
		if next == nil {
			if l, ok := cur.(interface {
				Unwrap() []error
			}); ok {
				errs := l.Unwrap()
				if len(errs) > 0 {
					next = errs[0]
				}
			}
		}
		cur = next
		if p, ok := cur.(interface {
			Pos() Position
		}); ok && p.Pos() != NoPosition {
			pos = append(pos, p.Pos())
		}
		if e, ok := cur.(*ErrPosition); ok {
			last = e
		}
	}

	backtrace := printPath(pos)
	if len(backtrace) > 0 {
		return fmt.Sprintf("%s: %s (%s)", last.Err.Error(), last.Position, printPath(pos))
	}
	if last.Position == NoPosition {
		return last.Err.Error()
	}
	return fmt.Sprintf("%s: %s", last.Err.Error(), last.Position)
}

func printPath(pos []Position) string {
	if len(pos) <= 1 {
		return ""
	}

	buf := strings.Builder{}
	last := pos[len(pos)-1]
	buf.WriteString(fmt.Sprintf("%d:%d", last.Line, last.Column))

	for i := len(pos) - 1; i >= 0; i-- {
		next := pos[i]
		if next == last {
			continue
		}
		buf.WriteString("<-")
		if last.Filename != next.Filename {
			buf.WriteString(next.Filename)
			buf.WriteString(":")
		}
		buf.WriteString(fmt.Sprintf("%d:%d", next.Line, next.Column))
		last = next
	}

	return buf.String()
}
