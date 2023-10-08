package eval

import (
	"context"

	"github.com/acorn-io/aml/pkg/ast"
	"github.com/acorn-io/aml/pkg/value"
)

type EvalOptions []EvalOption

func (e EvalOptions) Merge() (result EvalOption) {
	for _, opt := range e {
		if len(opt.Globals) > 0 && result.Globals == nil {
			result.Globals = map[string]any{}
		}
		for k, v := range opt.Globals {
			result.Globals[k] = v
		}
	}
	return
}

type EvalOption struct {
	Globals map[string]any
}

func (e EvalOption) Complete() EvalOption {
	return e
}

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

func EvalExpr(ctx context.Context, expr Expression, opts ...EvalOption) (value.Value, bool, error) {
	opt := EvalOptions(opts).Merge().Complete()
	scope := Builtin.Push(ScopeData(opt.Globals), ScopeOption{
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
