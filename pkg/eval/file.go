package eval

import (
	"sort"

	"github.com/acorn-io/aml/pkg/schema"
	"github.com/acorn-io/aml/pkg/value"
)

type File struct {
	PositionalArgs []any
	Args           map[string]any
	Profiles       []string
	Body           *Struct
}

func (f *File) DescribeFile() (*schema.File, error) {
	fun, ok, err := f.ToFunction(Builtin)
	if err != nil {
		return nil, err
	} else if !ok {
		return &schema.File{}, nil
	}

	args := fun.(*Function).ArgsSchema
	profiles := fun.(*Function).ProfileNames

	argsSchema, err := value.DescribeObject(value.SchemaContext{}, args)
	if err != nil {
		return nil, err
	}

	return &schema.File{
		Args:         *argsSchema,
		ProfileNames: profiles.Describe(),
	}, nil
}

func (f *File) ToFunction(scope Scope) (value.Value, bool, error) {
	def := &FunctionDefinition{
		Body:             f.Body,
		ReturnBody:       true,
		AllowUnknownArgs: true,
		AssignRoot:       true,
	}
	return def.ToValue(scope)
}

func (f *File) ToValue(scope Scope) (value.Value, bool, error) {
	call, ok, err := f.ToFunction(scope)
	if err != nil || !ok {
		return nil, ok, err
	}

	return value.Call(scope.Context(), call, f.CallArgs()...)
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
