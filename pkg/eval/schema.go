package eval

import (
	"github.com/acorn-io/aml/pkg/value"
)

type Schema struct {
	Comments       Comments
	Struct         *Struct
	AllowNewFields bool
}

func (s *Schema) ToValue(scope Scope) (value.Value, bool, error) {
	return s.Struct.ToValue(scope.Push(nil, ScopeOption{
		Schema:       true,
		AllowNewKeys: s.AllowNewFields,
	}))
}
