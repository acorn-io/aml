package eval

import (
	"context"
	"fmt"

	"github.com/acorn-io/aml/pkg/errors"
	"github.com/acorn-io/aml/pkg/value"
)

type Scope interface {
	Depth() int
	IsRoot(ctx context.Context) (bool, error)
	Get(ctx context.Context, key string) (value.Value, bool, error)
	NewScope(ctx context.Context, data ScopeStorage) (Scope, context.Context)
}

type ScopeFunc func(ctx context.Context, key string, parent Scope) (value.Value, bool, error)

func (s ScopeFunc) Lookup(ctx context.Context, key string, parent Scope) (value.Value, bool, error) {
	return s(ctx, key, parent)
}

type ScopeStorage interface {
	Lookup(ctx context.Context, key string, parent Scope) (value.Value, bool, error)
}

type ScopeData map[string]any

func (m ScopeData) Lookup(ctx context.Context, key string, parent Scope) (value.Value, bool, error) {
	ret, ok := m[key]
	if !ok && parent != nil {
		return parent.Get(ctx, key)
	}
	return value.NewValue(ret), ok, nil
}

func EmptyScope(ctx context.Context, data map[string]any) (Scope, context.Context) {
	scope := &nested{
		storage: ScopeData(data),
	}
	return scope, WithScope(ctx, scope)
}

type ScopeAsValue struct {
	Ctx   context.Context
	Scope Scope
}

func (s ScopeAsValue) LookupValue(key value.Value) (value.Value, bool, error) {
	str, err := value.ToString(key)
	if err != nil {
		return nil, false, err
	}
	return s.Scope.Get(s.Ctx, str)
}

func (s ScopeAsValue) Kind() value.Kind {
	return value.ObjectKind
}

type nested struct {
	depth   int
	parent  Scope
	storage ScopeStorage
}

func (n *nested) IsRoot(ctx context.Context) (bool, error) {
	if n.storage == nil {
		return false, nil
	}
	_, ok, err := n.storage.Lookup(ctx, "__root", nil)
	if e := (*errors.ErrValueNotDefined)(nil); errors.As(err, &e) {
		return false, nil
	}
	return ok, err
}

func (n *nested) Depth() int {
	return n.depth
}

func (n *nested) Get(ctx context.Context, key string) (ret value.Value, ok bool, err error) {
	if n.depth > MaxCallDepth {
		return nil, false, fmt.Errorf("exceeded max scope depth %d > %d", n.depth, MaxCallDepth)
	}

	if key == "$" && n.parent != nil {
		root, err := n.parent.IsRoot(ctx)
		if err != nil {
			return nil, false, err
		}
		if root {
			return ScopeAsValue{
				Ctx:   ctx,
				Scope: n,
			}, true, nil
		}
	}

	if n.storage != nil {
		parent := n.parent
		v, ok, err := n.storage.Lookup(ctx, key, parent)
		if err != nil {
			return nil, false, err
		} else if ok {
			return v, ok, nil
		}
	} else if n.parent != nil {
		return n.parent.Get(ctx, key)
	}

	return nil, false, nil
}

func scopePush(n Scope, storage ScopeStorage) *nested {
	newScope := &nested{
		parent:  n,
		storage: storage,
		depth:   n.Depth() + 1,
	}

	return newScope
}

func (n *nested) NewScope(ctx context.Context, storage ScopeStorage) (Scope, context.Context) {
	newScope := scopePush(n, storage)
	return newScope, WithScope(ctx, newScope)
}

type ValueScopeLookup struct {
	Value value.Value
}

func (v ValueScopeLookup) Lookup(ctx context.Context, key string, parent Scope) (value.Value, bool, error) {
	ret, ok, err := value.Lookup(v.Value, value.NewValue(key))
	if err != nil {
		return nil, false, err
	} else if !ok && parent != nil {
		return parent.Get(ctx, key)
	}
	return ret, ok, nil
}
