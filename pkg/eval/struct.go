package eval

import (
	"errors"
	"strings"

	"github.com/acorn-io/aml/pkg/value"
)

type Struct struct {
	Position Position
	Comments Comments
	Fields   []Field
}

func (s *Struct) ScopeLookup(scope Scope, key string) (value.Value, bool, error) {
	var values []value.Value

	for _, field := range s.Fields {
		val, ok, err := field.ToValueForKey(scope, key)
		if c := (*ErrKeyUndefined)(nil); errors.As(err, &c) {
			continue
		} else if err != nil {
			return nil, false, err
		} else if !ok {
			continue
		}
		values = append(values, val)
	}

	result, err := value.Merge(values...)
	return result, result != nil, err
}

func (s *Struct) ToValue(scope Scope) (value.Value, bool, error) {
	if scope.IsSchema() {
		return value.NewValue(&contract{
			s:     s,
			scope: scope.Push(s),
		}), true, nil
	}

	scope = scope.Push(s)
	result, err := FieldsToValue(scope, s.Fields)
	if err != nil {
		return nil, false, err
	}

	if result == nil {
		return value.NewObject(nil), true, nil
	}
	return result, true, nil
}

type Comments struct {
	Comments [][]string
}

func (c Comments) Last() string {
	if len(c.Comments) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(c.Comments[0], "\n"))
}
