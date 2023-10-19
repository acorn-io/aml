package eval

import (
	"context"
	"fmt"

	"github.com/acorn-io/aml/pkg/value"
)

type Array struct {
	Pos      value.Position
	Comments Comments
	Items    []Expression
}

func (a *Array) ToValue(ctx context.Context) (value.Value, bool, error) {
	var (
		objs      []any
		kinds     = map[value.Kind]struct{}{}
		schema    []*value.TypeSchema
		allSimple = true
	)

	for i, item := range a.Items {
		ctx := value.WithIndexPath(ctx, i)

		v, ok, err := item.ToValue(ctx)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			continue
		}

		// Undefined element makes the entire array undefined
		if undef := value.IsUndefined(v); undef != nil {
			return undef, true, nil
		}

		objs = append(objs, v)
		if IsSchema(ctx) {
			if value.IsSimpleKind(v.Kind()) {
				if _, ok := kinds[v.Kind()]; ok {
					continue
				} else {
					kinds[v.Kind()] = struct{}{}
					v = &value.TypeSchema{
						KindValue: v.Kind(),
						Path:      value.GetPath(ctx),
					}
				}
			} else {
				allSimple = false
			}
			if ts, ok := v.(*value.TypeSchema); ok {
				schema = append(schema, ts)
			} else {
				return nil, false, value.NewErrPosition(a.Pos,
					fmt.Errorf("schema for array must contain only schema values, not kind %s [%T]", v.Kind(), v))
			}
		}
	}

	if IsSchema(ctx) {
		var def value.Value
		if allSimple && len(objs) > 0 {
			def = value.NewArray(objs)
		}
		return &value.TypeSchema{
			Positions: []value.Position{a.Pos},
			Path:      value.GetPath(ctx),
			KindValue: value.ArrayKind,
			Array: &value.ArraySchema{
				Positions:   []value.Position{a.Pos},
				Description: a.Comments.Last(),
				Valid:       schema,
			},
			DefaultValue: def,
		}, true, nil
	}

	return value.NewValue(objs), true, nil
}
