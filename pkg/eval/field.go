package eval

import (
	"context"

	"github.com/acorn-io/aml/pkg/value"
)

// type assertions
var (
	_ Field = (*KeyValue)(nil)
)

type Field interface {
	ToValueForIndex(ctx context.Context, idx int) (value.Value, bool, error)
	Position() value.Position

	// IsForLookup Should this field be added to the current scope and it's keys available as scope keys
	IsForLookup(ctx context.Context) bool
	// IsForValue Should this fields value be contributed to the rendered data
	IsForValue(ctx context.Context) bool
}

type KeyValue struct {
	Comments Comments
	Key      FieldKey
	Value    Expression
	Pos      value.Position
	Local    bool
	Optional bool
}

func (k *KeyValue) IsForLookup(_ context.Context) bool {
	return k.Key.Match == nil
}

func (k *KeyValue) IsForValue(ctx context.Context) bool {
	if k.Local {
		return false
	}
	if !IsSchema(ctx) && k.Key.Match != nil {
		return false
	}
	return true
}

func (k *KeyValue) Position() value.Position {
	return k.Pos
}

func (k *KeyValue) returnTuple(ctx context.Context, key string) (value.Value, bool, error) {
	v, ok, err := k.getValueValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}

	if undef := value.IsUndefined(v); undef != nil && IsSchema(ctx) {
		return undef, true, nil
	}

	if IsSchema(ctx) {
		var resultTS value.Schema
		if ts, ok := v.(value.Schema); ok {
			resultTS = ts
		} else {
			resultTS = value.NewMatchTypeWithDefault(k.Pos, v).(*value.TypeSchema)
		}
		return &value.TypeSchema{
			Positions: []value.Position{k.Pos},
			KindValue: value.ObjectKind,
			Path:      value.GetPath(ctx),
			Object: &value.ObjectSchema{
				Positions:    []value.Position{k.Pos},
				AllowNewKeys: IsAllowNewKeys(ctx),
				Fields: []value.ObjectSchemaField{
					{
						Key:         key,
						Match:       k.Key.Match != nil,
						Optional:    k.Optional,
						Description: k.Comments.Last(),
						Schema:      resultTS,
					},
				},
			},
		}, true, nil
	}

	return &value.Object{
		Entries: []value.Entry{{
			Key:   key,
			Value: v,
		}},
	}, true, nil
}

func (k *KeyValue) getValueValue(ctx context.Context) (ret value.Value, _ bool, _ error) {
	v, ok, err := k.Value.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}
	if value.IsSimpleKind(v.Kind()) && IsSchema(ctx) {
		return value.NewMatchTypeWithDefault(k.Pos, v), true, nil
	}
	return v, true, nil
}

func (k *KeyValue) IsArgumentDefinition() bool {
	s := k.Key.Key
	return string(s) == "args" || string(s) == "profiles"
}

func (k *KeyValue) ToValueForIndex(ctx context.Context, i int) (value.Value, bool, error) {
	// Match only makes sense when not in schema
	if k.Key.Match != nil && !IsSchema(ctx) {
		return nil, false, nil
	}

	key, undef, ok, err := k.Key.ToString(value.WithIndexPath(ctx, i))
	if err != nil || !ok {
		return nil, ok, err
	} else if undef != nil {
		return undef, true, nil
	}

	return k.returnTuple(value.WithKeyPath(ctx, key), key)
}

type FieldKey struct {
	Match         Expression
	Interpolation Expression
	Key           string
	Pos           value.Position
}

func (k *FieldKey) ToString(ctx context.Context) (_ string, undef value.Value, _ bool, _ error) {
	if k.Interpolation != nil {
		v, ok, err := k.Interpolation.ToValue(ctx)
		if err != nil || !ok {
			return "", nil, ok, err
		} else if undef := value.IsUndefined(v); undef != nil {
			return "", undef, true, nil
		}
		s, err := value.ToString(v)
		return s, nil, true, err
	} else if k.Match != nil {
		v, ok, err := k.Match.ToValue(ctx)
		if err != nil || !ok {
			return "", nil, ok, err
		} else if undef := value.IsUndefined(v); undef != nil {
			return "", undef, true, nil
		}
		s, err := value.ToString(v)
		return s, nil, true, err
	}
	return k.Key, nil, true, nil
}
