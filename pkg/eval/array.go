package eval

import (
	"fmt"

	"github.com/acorn-io/aml/pkg/value"
)

type Array struct {
	Pos      Position
	Comments Comments
	Items    []Expression
}

func (a *Array) ToValue(scope Scope) (value.Value, bool, error) {
	var objs []any

	for i, item := range a.Items {
		scope := scope.Push(nil, ScopeOption{
			Path: fmt.Sprint(i),
		})
		v, ok, err := item.ToValue(scope)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			continue
		}
		if value.IsSimpleKind(v.Kind()) && scope.IsSchema() {
			v = value.NewMatchTypeWithDefault(v)
		}
		objs = append(objs, v)
	}

	arr := value.NewArray(objs)
	if scope.IsSchema() {
		return value.NewArraySchema(arr), true, nil
	}
	return arr, true, nil
}
