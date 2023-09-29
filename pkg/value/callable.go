package value

import (
	"context"
	"fmt"
)

type CallArgument struct {
	Positional bool
	Value      Value
}

type Caller interface {
	Call(ctx context.Context, args []CallArgument) (Value, bool, error)
}

func Call(ctx context.Context, value Value, args ...CallArgument) (_ Value, _ bool, ret error) {
	if value.Kind() == UndefinedKind {
		return value, true, nil
	}
	if caller, ok := value.(Caller); ok {
		return caller.Call(ctx, args)
	}
	return nil, false, fmt.Errorf("kind %s is not callable", value.Kind())
}
