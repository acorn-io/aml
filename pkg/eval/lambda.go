package eval

import (
	"context"

	"github.com/acorn-io/aml/pkg/value"
)

type LambdaDefinition struct {
	Comments Comments
	Pos      value.Position
	Vars     []string
	Body     Expression
}

func (f *LambdaDefinition) ToValue(ctx context.Context) (value.Value, bool, error) {
	var argNames value.Names
	for _, name := range f.Vars {
		argNames = append(argNames, value.Name{
			Name: name,
		})
	}
	scope := GetScope(ctx)
	return &Function{
		Pos:            f.Pos,
		Scope:          scope,
		Body:           f.Body,
		ArgsSchema:     value.NewOpenObject(),
		ArgNames:       argNames,
		ProfilesSchema: value.NewOpenObject(),
		UnscopedArgs:   true,
		ReturnBody:     true,
	}, true, nil
}
