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
	for _, kind := range value.Kinds {
		if kind == value.UndefinedKind {
			continue
		} else if kind == value.SchemaKind {
			data["type"] = &value.TypeSchema{
				KindValue: kind,
			}
			continue
		}
		data[string(kind)] = &value.TypeSchema{
			KindValue: kind,
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

func addStd(data map[string]any) any {
	expr, err := Build(std.File)
	if err != nil {
		panic(err)
	}

	stdValue, ok, err := expr.ToValue(EmptyScope{}.Push(ScopeData(data)))
	if err != nil || !ok {
		panic(fmt.Sprintf("invalid std library: %v", err))
	}

	return stdValue
}

func init() {
	data := statics()
	data["len"] = NativeFuncValue(Len)
	data["keys"] = NativeFuncValue(Keys)
	data["enum"] = NativeFuncValue(Enum)
	data["int"] = Int()
	data["any"] = Any(data)
	data["std"] = addStd(data)

	Builtin = EmptyScope{}.Push(ScopeData(data))
}

type nativeCallable struct {
	f NativeFunc
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
