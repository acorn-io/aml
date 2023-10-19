package eval

import (
	"context"

	"github.com/acorn-io/aml/pkg/value"
)

type Schema struct {
	Comments       Comments
	Expression     Expression
	AllowNewFields bool
}

func (s *Schema) ToValue(ctx context.Context) (value.Value, bool, error) {
	return s.Expression.ToValue(
		WithAllowNewKeys(WithSchema(ctx, true), s.AllowNewFields))
}
