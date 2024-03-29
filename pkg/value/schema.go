package value

import (
	"context"
	"fmt"
)

type Schema interface {
	Value
	Orer

	GetPositions() []Position
	Validate(ctx context.Context, v Value) (Value, error)
	TargetKind() Kind
	MergeType(right Schema) (Schema, error)
	DefaultWithImplicit(renderImplicit bool) (Value, bool, error)

	ValidArrayItems() []Schema
	GetPath() Path
}

func Validate(ctx context.Context, schema Value, v Value) (Value, error) {
	if undef := IsUndefined(v); undef != nil {
		return undef, nil
	}
	if s, ok := schema.(Schema); ok {
		return s.Validate(ctx, v)
	}
	return nil, fmt.Errorf("value kind %s can not be used for validation", v.Kind())
}
