package eval

import (
	"github.com/acorn-io/aml/pkg/schema"
	"github.com/acorn-io/aml/pkg/value"
)

var _ value.Contract = (*contract)(nil)

type contract struct {
	s     *Struct
	scope Scope
}

func (c *contract) Position() value.Position {
	return value.Position(c.s.Position)
}

func (c *contract) Description() string {
	return c.s.Comments.Last()
}

func (c *contract) Fields(ctx value.SchemaContext) (result []schema.Field, _ error) {
	var (
		scope = c.scope.Push(c.s)
	)

	for i, field := range c.s.Fields {
		ctx.SetIndex(i)
		schema, err := field.DescribeFields(ctx, scope)
		if err != nil {
			return nil, err
		}
		result = append(result, schema...)
	}
	return result, nil
}

func (c *contract) Path() string {
	return c.scope.Path()
}

func (c *contract) AllowNewKeys() bool {
	return c.scope.AllowNewKeys()
}

func (c *contract) RequiredKeys() (result []string, _ error) {
	var (
		keySeen = map[string]struct{}{}
		scope   = c.scope.Push(c.s)
	)

	for _, field := range c.s.Fields {
		keys, err := field.RequiredKeys(scope)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			if _, ok := keySeen[key]; ok {
				continue
			}
			keySeen[key] = struct{}{}
			result = append(result, key)
		}
	}
	return
}

func (c *contract) AllKeys() (result []string, _ error) {
	var (
		keySeen = map[string]struct{}{}
		scope   = c.scope.Push(c.s)
	)
	for _, field := range c.s.Fields {
		keys, err := field.AllKeys(scope)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			if _, ok := keySeen[key]; ok {
				continue
			}
			keySeen[key] = struct{}{}
			result = append(result, key)
		}
	}
	return
}

func (c *contract) LookupValueForKeyEquals(key string) (value.Value, bool, error) {
	return c.s.ScopeLookup(c.scope, key)
}

func (c *contract) LookupValueForKeyPatternMatch(key string) (value.Value, bool, error) {
	var (
		values []value.Value
		scope  = c.scope.Push(c.s)
	)

	for _, field := range c.s.Fields {
		val, ok, err := field.ToValueForMatch(scope, key)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			continue
		}
		values = append(values, val)
	}

	result, err := value.Merge(values...)
	return result, result != nil, err
}
