package eval

import (
	"context"

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
		if opt.GlobalsLookup != nil {
			result.GlobalsLookup = opt.GlobalsLookup
		}
	}
	return
}

type EvalOption struct {
	Globals       map[string]any
	GlobalsLookup ScopeLookuper
}

func (e EvalOption) Complete() EvalOption {
	return e
}

func EvalExpr(ctx context.Context, expr Expression, opts ...EvalOption) (value.Value, bool, error) {
	opt := EvalOptions(opts).Merge().Complete()
	scope := Builtin.Push(ScopeData(opt.Globals), ScopeOption{
		Context: ctx,
	})
	if opt.GlobalsLookup != nil {
		scope = scope.Push(opt.GlobalsLookup)
	}
	return expr.ToValue(scope)
}

func EvalSchema(ctx context.Context, expr Expression) (value.Value, bool, error) {
	scope := Builtin.Push(nil, ScopeOption{
		Schema:  true,
		Context: ctx,
	})
	return expr.ToValue(scope)
}
