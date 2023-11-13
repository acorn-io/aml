package eval

import (
	"context"

	amlerrors "github.com/acorn-io/aml/pkg/errors"
	"github.com/acorn-io/aml/pkg/value"
)

type structScopeStorage struct {
	knownKeys  map[string]struct{}
	keyMiss    map[string]struct{}
	s          *Struct
	scopeValue value.Value
}

func newStructScopeStorage(s *Struct) *structScopeStorage {
	return &structScopeStorage{
		knownKeys: map[string]struct{}{},
		keyMiss:   map[string]struct{}{},
		s:         s,
	}
}

func mergeUndef(left, right value.Value) (newValue value.Value, changed bool, _ error) {
	if left.Kind() == value.UndefinedKind && right.Kind() != value.UndefinedKind {
		// Yeah progress is made
		return right, true, nil
	}

	if left.Kind() == value.ObjectKind && right.Kind() == value.ObjectKind {
		return value.MergeObjects(left, right, true, mergeUndef)
	}

	return right, false, nil
}

func (s *structScopeStorage) Store(v value.Value) (retry bool, err error) {
	var (
		newValue value.Value
	)

	if v == nil {
		s.scopeValue = v
		return false, nil
	}

	if s.scopeValue == nil {
		newValue = v
		retry = true
	} else if value.IsObjectLike(s.scopeValue) && value.IsObjectLike(v) {
		newValue, retry, err = value.MergeObjects(s.scopeValue, v, true, mergeUndef)
		if err != nil {
			return false, err
		}
	} else if q, err := value.Eq(s.scopeValue, v); err != nil {
		return false, err
	} else if eq, err := value.ToBool(q); err != nil {
		return false, err
	} else if eq {
		newValue = v
		retry = false
	} else {
		retry = true
		newValue, err = value.Merge(s.scopeValue, v)
		if err != nil {
			return false, err
		}
	}

	newKeys, err := value.KeysIfSupported(newValue)
	if err != nil {
		return false, err
	}

	s.scopeValue = newValue

	for _, newKey := range newKeys {
		s.knownKeys[newKey] = struct{}{}
		if _, ok := s.keyMiss[newKey]; ok {
			// blank state and try again
			retry = true
			s.keyMiss = map[string]struct{}{}
			s.scopeValue = nil
		}
	}

	return retry, nil
}

func (s *structScopeStorage) lookup(key string) (value.Value, bool, error) {
	if s.scopeValue == nil {
		return nil, false, nil
	}
	if value.IsLookupSupported(s.scopeValue) {
		return value.Lookup(s.scopeValue, value.NewValue(key))
	}
	return nil, false, nil
}

func (s *structScopeStorage) Lookup(ctx context.Context, key string, parent Scope) (value.Value, bool, error) {
	if v, ok, err := s.lookup(key); err != nil {
		return nil, false, err
	} else if ok {
		return v, true, nil
	}

	if _, ok := s.knownKeys[key]; ok {
		return nil, false, amlerrors.NewErrValueNotDefined(s.s.Position, key)
	}

	if parent == nil {
		return nil, false, amlerrors.NewErrValueNotDefined(s.s.Position, key)
	}

	v, ok, err := parent.Get(ctx, key)
	if err != nil {
		return nil, ok, err
	} else if !ok {
		return nil, false, amlerrors.NewErrValueNotDefined(s.s.Position, key)
	}

	s.keyMiss[key] = struct{}{}
	return v, true, nil
}
