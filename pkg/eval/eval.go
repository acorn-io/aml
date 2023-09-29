package eval

import (
	"context"

	"github.com/acorn-io/aml/pkg/ast"
	"github.com/acorn-io/aml/pkg/value"
)

func EvalFile(ctx context.Context, ast *ast.File, opts ...BuildOption) (value.Value, bool, error) {
	expr, err := Build(ast, opts...)
	if err != nil {
		return nil, false, err
	}
	scope := Builtin.Push(nil, ScopeOption{
		Context: ctx,
	})
	return expr.ToValue(scope)
}

func EvalExpr(ctx context.Context, expr Expression) (value.Value, bool, error) {
	scope := Builtin.Push(nil, ScopeOption{
		Context: ctx,
	})
	return expr.ToValue(scope)
}

func EvalSchema(ctx context.Context, expr Expression) (value.Value, bool, error) {
	scope := Builtin.Push(nil, ScopeOption{
		Schema:  true,
		Context: ctx,
	})
	return expr.ToValue(scope)
}
