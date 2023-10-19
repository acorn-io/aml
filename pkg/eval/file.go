package eval

import (
	"context"
	"sort"

	"github.com/acorn-io/aml/pkg/value"
)

type File struct {
	PositionalArgs []any
	Args           map[string]any
	Profiles       []string
	Body           *Struct
}

func (f *File) Describe(ctx context.Context) (*value.FuncSchema, error) {
	fileSchema, _, err := f.toFunction(WithSchema(ctx, true), false)
	if err != nil {
		return nil, err
	}
	return fileSchema.(*value.TypeSchema).FuncSchema, nil
}

func (f *File) toFunction(ctx context.Context, root bool) (value.Value, bool, error) {
	def := &FunctionDefinition{
		Pos:        f.Body.Position,
		Body:       f.Body,
		ReturnBody: true,
		AssignRoot: root,
	}
	return def.ToValue(ctx)
}

func (f *File) ToValue(ctx context.Context) (value.Value, bool, error) {
	call, ok, err := f.toFunction(ctx, true)
	if err != nil || !ok {
		return nil, ok, err
	}

	return value.Call(ctx, call, f.CallArgs()...)
}

func (f *File) CallArgs() (result []value.CallArgument) {
	var keys []string
	for k := range f.Args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, arg := range f.PositionalArgs {
		result = append(result, value.CallArgument{
			Positional: true,
			Value:      value.NewValue(arg),
		})
	}

	for _, key := range keys {
		result = append(result, value.CallArgument{
			Value: value.NewValue(map[string]any{
				key: f.Args[key],
			}),
		})
	}

	var profiles []any
	for _, profile := range f.Profiles {
		profiles = append(profiles, profile)
	}

	if len(profiles) > 0 {
		result = append(result, value.CallArgument{
			Value: value.NewValue(map[string]any{
				"profiles": value.NewValue(profiles),
			}),
		})
	}

	return
}
