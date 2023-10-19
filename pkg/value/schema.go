package value

import (
	"context"
	"fmt"
)

type Schema interface {
	Value
	Validate(ctx context.Context, v Value) (Value, error)
	TargetKind() Kind

	ValidArrayItems() []Schema
	GetPath() string
}

func Validate(ctx context.Context, schema Value, v Value) (Value, error) {
	if s, ok := schema.(Schema); ok {
		return s.Validate(ctx, v)
	}
	return nil, fmt.Errorf("value kind %s can not be used for validation", v.Kind())
}
