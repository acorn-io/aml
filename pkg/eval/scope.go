package eval

import (
	"context"
	"fmt"

	"github.com/acorn-io/aml/pkg/value"
)

type ScopeOption struct {
	Schema       bool
	AllowNewKeys bool
	Default      bool
	Call         bool
	Path         string
	Context      context.Context
}

func combine(opts []ScopeOption) (result ScopeOption) {
	for _, opt := range opts {
		if opt.Schema {
			result.Schema = true
		}
		if opt.AllowNewKeys {
			result.Schema = true
			result.AllowNewKeys = true
		}
		if opt.Default {
			result.Default = opt.Default
		}
		if opt.Call {
			result.Call = opt.Call
		}
		if opt.Path != "" {
			result.Path = opt.Path
		}
		if opt.Context != nil {
			result.Context = opt.Context
		}
	}
	if result.Context == nil {
		result.Context = context.Background()
	}
	return
}

type Scope interface {
	Context() context.Context
	Path() string
	Get(key string) (value.Value, bool, error)
	Push(lookup ScopeLookuper, opts ...ScopeOption) Scope
	IsSchema() bool
	AllowNewKeys() bool
}

type ScopeLookuper interface {
	ScopeLookup(scope Scope, key string) (value.Value, bool, error)
}

type ScopeData map[string]any

func (m ScopeData) ScopeLookup(_ Scope, key string) (value.Value, bool, error) {
	ret, ok := m[key]
	return value.NewValue(ret), ok, nil
}

func (m ScopeData) Get(key string) (value.Value, bool, error) {
	obj, ok := m[key]
	if !ok {
		return nil, ok, nil
	}
	return value.NewValue(obj), true, nil
}

type EmptyScope struct {
}

func (e EmptyScope) Path() string {
	return ""
}

func (e EmptyScope) Get(key string) (value.Value, bool, error) {
	return nil, false, nil
}

func (a EmptyScope) Context() context.Context {
	return context.Background()
}

func (e EmptyScope) Push(lookup ScopeLookuper, opts ...ScopeOption) Scope {
	return scopePush(e, lookup, opts...)
}

func (e EmptyScope) IsSchema() bool {
	return false
}

func (e EmptyScope) AllowNewKeys() bool {
	return false
}

type nested struct {
	depth    int
	path     string
	parent   Scope
	lookup   ScopeLookuper
	opts     ScopeOption
	keyCache map[string]value.Value
}

func (n nested) Depth() int {
	return n.depth
}

func (n nested) AllowNewKeys() bool {
	if n.opts.Default {
		return false
	}
	if n.opts.Schema {
		return n.opts.AllowNewKeys
	}
	return n.parent.AllowNewKeys()
}

func (n nested) IsSchema() bool {
	if n.opts.Call {
		return false
	}
	if n.opts.Default {
		return false
	}
	if n.opts.Schema {
		return true
	}
	return n.parent.IsSchema()
}

func (n nested) Context() context.Context {
	if n.opts.Context != nil {
		return n.opts.Context
	}
	return n.parent.Context()
}

func (n nested) Get(key string) (ret value.Value, ok bool, err error) {
	if n.depth > MaxCallDepth {
		return nil, false, fmt.Errorf("exceeded max scope depth %d > %d", n.depth, MaxCallDepth)
	}
	if v, ok := n.keyCache[key]; ok {
		return v, true, nil
	}

	v, ok, err := n.lookup.ScopeLookup(n, key)
	if err != nil {
		return nil, false, err
	} else if ok {
		if value.IsDefined(v) {
			n.keyCache[key] = v
		}
		return v, ok, nil
	}

	v, ok, err = n.parent.Get(key)
	if err == nil && ok && value.IsDefined(v) {
		n.keyCache[key] = v
	}
	return v, ok, err
}

func (n nested) Path() string {
	return n.path
}

func scopePush(n Scope, lookup ScopeLookuper, opts ...ScopeOption) Scope {
	if lookup == nil {
		lookup = ScopeData(nil)
	}
	o := combine(opts)
	newPath := appendPath(n.Path(), o.Path)
	newScope := nested{
		path:     newPath,
		parent:   n,
		lookup:   lookup,
		opts:     o,
		keyCache: make(map[string]value.Value),
	}

	ctx := n.Context()
	depth, _ := ctx.Value(depthKey{}).(int)
	depth = depth + 1

	newScope.opts.Context = context.WithValue(ctx, depthKey{}, depth)
	newScope.depth = depth
	return newScope
}

func (n nested) Push(lookup ScopeLookuper, opts ...ScopeOption) Scope {
	return scopePush(n, lookup, opts...)
}

func appendPath(current, next string) string {
	if next == "" {
		return current
	} else if current == "" {
		return next
	}
	return current + "." + next
}

type ValueScopeLookup struct {
	Value value.Value
}

func (v ValueScopeLookup) ScopeLookup(_ Scope, key string) (value.Value, bool, error) {
	return value.Lookup(v.Value, value.NewValue(key))
}
