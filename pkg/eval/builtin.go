package eval

import (
	"context"
	"fmt"

	"github.com/acorn-io/aml/pkg/std"
	"github.com/acorn-io/aml/pkg/value"
)

var Builtin Scope

func kinds() map[string]any {
	data := map[string]any{}
	for _, kind := range value.BuiltinKinds {
		if kind == value.ObjectKind {
			data[string(kind)] = value.NewOpenObject()
		} else if kind == value.ArrayKind {
			data[string(kind)] = &value.TypeSchema{
				KindValue: kind,
				Array:     &value.ArraySchema{},
			}
		} else {
			data[string(kind)] = &value.TypeSchema{
				KindValue: kind,
			}
		}
	}
	return data
}

func statics() map[string]any {
	data := kinds()

	data["builtin"] = map[string]any{
		"__internal": nativeFuncs,
	}

	return data
}

func addStd(ctx context.Context, data map[string]any) any {
	expr, err := Build(std.File)
	if err != nil {
		panic(err)
	}

	_, ctx = EmptyScope(ctx, data)

	stdValue, ok, err := expr.ToValue(ctx)
	if err != nil || !ok {
		panic(fmt.Sprintf("invalid std library: %v", err))
	}

	return stdValue
}

func init() {
	ctx := context.Background()

	data := statics()
	data["len"] = NativeFuncValue(Len)
	data["keys"] = NativeFuncValue(Keys)
	data["enum"] = NativeFuncValue(Enum)
	data["int"] = Int()
	data["any"] = Any(data)
	data["std"] = addStd(ctx, data)
	data["skip"] = Skip()
	data["break"] = Break()

	Builtin, _ = EmptyScope(ctx, data)
}

type nativeCallable struct {
	f NativeFunc
}

func (n nativeCallable) Eq(right value.Value) (value.Value, error) {
	if rf, ok := right.(nativeCallable); ok {
		return value.NewValue(n.String() == rf.String()), nil
	}
	return value.False, nil
}

func (n nativeCallable) Kind() value.Kind {
	return value.FuncKind
}

func (n nativeCallable) String() string {
	return fmt.Sprint(n.f)
}

func (n nativeCallable) Call(ctx context.Context, args []value.CallArgument) (value.Value, bool, error) {
	var argValues []value.Value
	for _, arg := range args {
		if arg.Self {
			continue
		}
		if !arg.Positional {
			return nil, false, fmt.Errorf("only positional arguements are valid for native functions")
		}
		if arg.Value.Kind() == value.UndefinedKind {
			return arg.Value, true, nil
		}
		argValues = append(argValues, arg.Value)
	}
	return n.f(ctx, argValues)
}

type NativeFunc func(context.Context, []value.Value) (value.Value, bool, error)

func NativeFuncValue(f func(context.Context, []value.Value) (value.Value, bool, error)) value.Value {
	return nativeCallable{
		f: f,
	}
}
