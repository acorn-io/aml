package eval

import (
	"github.com/acorn-io/aml/pkg/value"
)

type LambdaDefinition struct {
	Comments Comments
	Pos      Position
	Vars     []string
	Body     Expression
}

func (f *LambdaDefinition) ToValue(scope Scope) (value.Value, bool, error) {
	var argNames Names
	for _, name := range f.Vars {
		argNames = append(argNames, Name{
			Name: name,
		})
	}
	return &Function{
		Pos:            f.Pos,
		Scope:          scope,
		Body:           f.Body,
		ArgsSchema:     value.NewObject(nil),
		ArgNames:       argNames,
		ProfilesSchema: value.NewObject(nil),
		UnscopedArgs:   true,
		ReturnBody:     true,
	}, true, nil
}
