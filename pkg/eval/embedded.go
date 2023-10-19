package eval

import (
	"context"

	"github.com/acorn-io/aml/pkg/value"
)

// type assertions
var (
	_ Field = (*Embedded)(nil)
)

type Embedded struct {
	Pos        value.Position
	Comments   Comments
	Expression Expression
}

func (e *Embedded) IsForLookup(_ context.Context) bool {
	return true
}

func (e *Embedded) IsForValue(_ context.Context) bool {
	return true
}

func (e *Embedded) Position() value.Position {
	return e.Pos
}

func (e *Embedded) IsPositionalArgument() bool {
	return true
}

func (e *Embedded) ToValueForIndex(ctx context.Context, fieldIndex int) (value.Value, bool, error) {
	return e.Expression.ToValue(ctx)
}
