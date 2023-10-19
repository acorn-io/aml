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
	GlobalsLookup ScopeFunc
}

func (e EvalOption) Complete() EvalOption {
	return e
}

func EvalExpr(ctx context.Context, expr Expression, opts ...EvalOption) (value.Value, bool, error) {
	opt := EvalOptions(opts).Merge().Complete()
	scope, ctx := GetScope(ctx).NewScope(ctx, ScopeData(opt.Globals))
	if opt.GlobalsLookup != nil {
		scope, ctx = scope.NewScope(ctx, opt.GlobalsLookup)
	}
	return expr.ToValue(ctx)
}

func EvalSchema(ctx context.Context, expr Expression) (value.Value, bool, error) {
	ctx = WithSchema(ctx, true)
	return expr.ToValue(ctx)
}
