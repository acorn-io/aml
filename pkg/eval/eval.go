package eval

import (
	"context"
	"fmt"

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

func EvalSchema(ctx context.Context, expr Expression) (value.Schema, bool, error) {
	ctx = WithSchema(ctx, true)
	val, ok, err := expr.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}
	s, ok := val.(value.Schema)
	if !ok {
		v, ok, err := value.NativeValue(val)
		if err != nil || !ok {
			return nil, ok, err
		}
		return nil, false, fmt.Errorf("expected schema but evaluated to: %v", v)
	}
	return s, true, nil
}
