package eval

import "github.com/acorn-io/aml/pkg/value"

// NewRootScope will create a scope that allow "$" key to be looked. It is expected that
// this scope be passed to the ToValue method of a eval.Struct instance. Any other
// usage is undefined
func NewRootScope(pos Position, parent Scope, data map[string]any) Scope {
	return &rootScopeFactory{
		Pos:            pos,
		Scope:          parent,
		additionalData: value.NewObject(data),
	}
}

type deferredLookup struct {
	Pos            Position
	scope          Scope
	lookup         ScopeLookuper
	cycle          map[string]struct{}
	additionalData value.Value
}

func (d deferredLookup) LookupValue(key value.Value) (value.Value, bool, error) {
	s, err := value.ToString(key)
	if err != nil {
		return nil, false, err
	}

	if _, seen := d.cycle[s]; seen {
		return value.Undefined{
			Pos: value.Position(d.Pos),
		}, true, nil
	}

	d.cycle[s] = struct{}{}
	defer delete(d.cycle, s)

	if d.additionalData != nil {
		v, ok, err := value.Lookup(d.additionalData, key)
		if err != nil {
			return nil, false, err
		} else if ok {
			return v, true, nil
		}
	}

	return d.lookup.ScopeLookup(d.scope, s)
}

func (d deferredLookup) Kind() value.Kind {
	return value.ObjectKind
}

type rootScopeFactory struct {
	Scope
	Pos            Position
	child          Scope
	lookup         ScopeLookuper
	additionalData value.Value
}

func (r *rootScopeFactory) Push(lookup ScopeLookuper, opts ...ScopeOption) Scope {
	// holy looping madness batman! The parent scope of the nested scope you are created needs to have
	// a reference to it's child. So we essentially clone ourself, set ourself as the new child scope
	// and then save the newly created nested scope.
	cp := *r
	child := scopePush(&cp, lookup, opts...)
	cp.child = child
	cp.lookup = lookup
	return child
}

func (r *rootScopeFactory) Get(key string) (value.Value, bool, error) {
	if key == "$" && r.child != nil {
		return deferredLookup{
			Pos:            r.Pos,
			scope:          r.child,
			lookup:         r.lookup,
			cycle:          map[string]struct{}{},
			additionalData: r.additionalData,
		}, true, nil
	}
	return r.Scope.Get(key)
}
