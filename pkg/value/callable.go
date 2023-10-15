package value

import (
	"context"
	"fmt"
)

type CallArgument struct {
	Positional bool
	Self       bool
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
		for _, arg := range args {
			if !IsDefined(arg.Value) {
				v := GetUndefined(arg.Value)
				if v == nil {
					return nil, false, fmt.Errorf("failed to find undefined value in non-defined object during call, entry (value %s)", arg.Value)
				}
				return v, true, nil
			}
		}
		return caller.Call(ctx, args)
	}
	return nil, false, fmt.Errorf("kind %s is not callable", value.Kind())
}
