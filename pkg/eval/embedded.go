package eval

import (
	"fmt"

	"github.com/acorn-io/aml/pkg/errors"
	"github.com/acorn-io/aml/pkg/schema"
	"github.com/acorn-io/aml/pkg/value"
)

// type assertions
var (
	_ Field = (*Embedded)(nil)
)

type Embedded struct {
	Pos        Position
	Comments   Comments
	Expression Expression

	disallowedKeys []string
}

func (e *Embedded) DescribeFields(ctx value.SchemaContext, scope Scope) ([]schema.Field, error) {
	// Get unique path so that schema references work correctly
	scope = scope.Push(nil, ScopeOption{
		Path: fmt.Sprintf("embedded.%d", ctx.GetIndex()),
	})

	v, ok, err := e.ToValue(scope)
	if err != nil {
		return nil, err
	} else if !ok {
		return nil, nil
	}

	obj, err := value.DescribeObject(ctx, v)
	if err != nil {
		return nil, err
	}

	return obj.Fields, nil
}

func (e *Embedded) IsPositionalArgument() bool {
	return true
}

func (e *Embedded) ToValueForKey(scope Scope, key string) (value.Value, bool, error) {
	v, ok, err := e.ToValue(scope)
	if err != nil || !ok {
		return nil, ok, err
	}
	if v.Kind() == value.UndefinedKind {
		e.disallowedKeys = append(e.disallowedKeys, key)
		return nil, false, errors.NewErrEval(value.Position(e.Pos), &ErrKeyUndefined{
			Key:       key,
			Undefined: v,
		})
	}
	return value.Lookup(v, value.NewValue(key))
}

func (e *Embedded) RequiredKeys(scope Scope) ([]string, error) {
	v, ok, err := e.ToValue(scope)
	if err != nil || !ok {
		return nil, err
	}
	if c, ok := value.GetContract(v); ok {
		return c.RequiredKeys()
	}
	return nil, nil
}

func (e *Embedded) AllKeys(scope Scope) ([]string, error) {
	v, ok, err := e.ToValue(scope)
	if err != nil || !ok {
		return nil, err
	}
	if c, ok := value.GetContract(v); ok {
		return c.AllKeys()
	}
	return nil, nil
}

func (e *Embedded) ToValueForMatch(scope Scope, key string) (value.Value, bool, error) {
	v, ok, err := e.ToValue(scope)
	if err != nil || !ok {
		return nil, ok, err
	}
	if c, ok := value.GetContract(v); ok {
		return c.LookupValueForKeyPatternMatch(key)
	}
	return nil, false, nil
}

func (e *Embedded) checkKeys(v value.Value) error {
	if len(e.disallowedKeys) == 0 {
		return nil
	}

	keys, err := value.Keys(v)
	if err != nil {
		// Ignore errors looking up keys. Either keys are supported or there is something wrong with
		// this value in which it's already invalid
		return nil
	}

	for _, check := range e.disallowedKeys {
		for _, key := range keys {
			if check == key {
				return errors.NewErrEval(value.Position(e.Pos),
					fmt.Errorf("invalid cycle detected in key %s", key))
			}
		}
	}

	return nil
}

func (e *Embedded) ToValue(scope Scope) (value.Value, bool, error) {
	v, ok, err := e.Expression.ToValue(scope)
	if err != nil || !ok {
		return nil, ok, err
	} else if t := value.TargetKind(v); scope.IsSchema() && t != value.ObjectKind && t != value.UndefinedKind {
		return nil, false, errors.NewErrEval(value.Position(e.Pos),
			fmt.Errorf("in schemas embedded expressions must evaluate to kind object, not %s", t))
	}
	if len(e.disallowedKeys) > 0 {
		keys, err := value.Keys(v)
		if err == nil {
			for _, check := range e.disallowedKeys {
				for _, key := range keys {
					if check == key {
						return nil, false, errors.NewErrEval(value.Position(e.Pos),
							fmt.Errorf("invalid cycle detected in key %s", key))
					}
				}
			}
		}
	}
	return v, true, e.checkKeys(v)
}
