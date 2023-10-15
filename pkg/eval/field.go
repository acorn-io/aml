package eval

import (
	"fmt"

	"github.com/acorn-io/aml/pkg/errors"
	"github.com/acorn-io/aml/pkg/schema"
	"github.com/acorn-io/aml/pkg/value"
)

// type assertions
var (
	_ Field = (*KeyValue)(nil)
)

// FieldSchema are the methods used only in a schema context
// Implementateon can assume that scope.IsSchema will be true
type FieldSchema interface {
	AllKeys(scope Scope) ([]string, error)
	RequiredKeys(scope Scope) ([]string, error)
	DescribeFields(ctx value.SchemaContext, scope Scope) ([]schema.Field, error)
	// ToValueForMatch should return a value only if this field is a match field
	ToValueForMatch(scope Scope, key string) (value.Value, bool, error)
}

type Field interface {
	Expression
	FieldSchema
	// ToValueForKey should return value where the key is equals to but ignoring any match fields
	ToValueForKey(scope Scope, key string) (value.Value, bool, error)
	Position() Position
}

type KeyValue struct {
	Comments Comments
	Key      FieldKey
	Value    Expression
	Pos      Position
	Local    bool
	Optional bool
}

func (k *KeyValue) Position() Position {
	return k.Pos
}

func (k *KeyValue) DescribeFields(ctx value.SchemaContext, scope Scope) ([]schema.Field, error) {
	var (
		key string
	)

	if k.Local {
		return nil, nil
	}

	if k.Key.IsMatch() {
		v, ok, err := k.Key.Match.ToValue(scope)
		if err != nil || !ok {
			return nil, nil
		}
		str, err := value.ToString(v)
		if err != nil {
			return nil, err
		}
		key = str
	} else {
		str, undef, ok, err := k.Key.ToString(scope)
		if err != nil {
			return nil, err
		} else if !ok {
			return nil, nil
		} else if undef != nil {
			return nil, errors.NewErrEval(value.Position(k.Key.Pos), fmt.Errorf("can not describe schema of undefined field from undefined at %s", undef))
		}
		key = str
	}

	v, ok, err := k.getValueValue(scope, key)
	if err != nil {
		return nil, err
	} else if !ok {
		return nil, nil
	}

	ft, err := value.DescribeFieldType(ctx, v)
	if err != nil {
		return nil, errors.NewErrEval(value.Position(k.Pos), err)
	}

	return []schema.Field{
		{
			Name:        key,
			Match:       k.Key.IsMatch(),
			Description: k.Comments.Last(),
			Optional:    k.Optional,
			Type:        ft,
		},
	}, nil
}

func (k *KeyValue) ToValueForKey(scope Scope, key string) (value.Value, bool, error) {
	if ok, err := k.Key.Equals(scope, key); err != nil || !ok {
		return nil, ok, err
	}
	return k.getValueValue(scope, key)
}

func (k *KeyValue) ToValueForMatch(scope Scope, key string) (value.Value, bool, error) {
	if ok, err := k.Key.PatternMatches(scope, key); err != nil || !ok {
		return nil, ok, err
	}
	return k.getValueValue(scope, key)
}

func (k *KeyValue) RequiredKeys(scope Scope) ([]string, error) {
	if k.Local || k.Optional {
		return nil, nil
	}
	s, undef, ok, err := k.Key.ToString(scope)
	if err != nil || !ok {
		return nil, err
	} else if undef != nil {
		return nil, errors.NewErrEval(value.Position(k.Key.Pos), fmt.Errorf("can not gather required keys due to undefined key (undefined %s)", undef))
	}
	return []string{s}, nil
}

func (k *KeyValue) AllKeys(scope Scope) ([]string, error) {
	if k.Local {
		return nil, nil
	}
	s, undef, ok, err := k.Key.ToString(scope)
	if err != nil || !ok {
		return nil, err
	} else if undef != nil {
		return nil, errors.NewErrEval(value.Position(k.Key.Pos), fmt.Errorf("can not gather all keys due to undefined key (undefined %s)", undef))
	}
	return []string{s}, nil
}

func (k *KeyValue) getValueValue(scope Scope, key string) (ret value.Value, _ bool, _ error) {
	scope = scope.Push(nil, ScopeOption{
		Path: key,
	})
	v, ok, err := k.Value.ToValue(scope)
	if err != nil || !ok {
		return nil, ok, err
	}
	if value.IsSimpleKind(v.Kind()) && scope.IsSchema() {
		return value.NewMatchTypeWithDefault(value.Position(k.Pos), v), true, nil
	}
	return v, true, nil
}

func (k *KeyValue) IsArgumentDefinition() bool {
	if v, ok := k.Key.Key.(Value); ok {
		if s, ok := v.Value.(value.String); ok {
			return string(s) == "args" || string(s) == "profiles"
		}
	}
	return false
}

func (k *KeyValue) ToValue(scope Scope) (value.Value, bool, error) {
	if k.Local || k.Key.IsMatch() {
		return nil, false, nil
	}

	var (
		v   value.Value
		ok  bool
		err error
	)

	key, undef, ok, err := k.Key.ToString(scope)
	if err != nil || !ok {
		return nil, ok, err
	} else if undef != nil {
		return undef, true, nil
	}

	v, ok, err = k.getValueValue(scope, key)
	if err != nil || !ok {
		return nil, ok, err
	}

	return &value.Object{
		Entries: []value.Entry{{
			Key:   key,
			Value: v,
		}},
	}, true, nil
}

func FieldsToValue(scope Scope, fields []Field) (result value.Value, _ error) {
	for _, field := range fields {
		v, ok, err := field.ToValue(scope)
		if err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		if result == nil {
			result = v
		} else {
			result, err = value.Merge(result, v)
			if err != nil {
				return nil, errors.NewErrEval(value.Position(field.Position()), err)
			}
		}
	}
	return
}

type FieldKey struct {
	Match Expression
	Key   Expression
	Pos   Position

	disallowedKeys []string
}

func (k *FieldKey) IsMatch() bool {
	return k.Match != nil
}

func (k *FieldKey) checkKey(key string) error {
	for _, check := range k.disallowedKeys {
		if key == check {
			return errors.NewErrEval(value.Position(k.Pos),
				fmt.Errorf("invalid cycle detected in key %s", key))
		}
	}
	return nil
}

func (k *FieldKey) ToString(scope Scope) (_ string, undef value.Value, ok bool, _ error) {
	if k.IsMatch() {
		// Match fields should not have a "string" equivalent, they are virtual thingies that only
		// exist in the magical schema realm
		return "", nil, false, nil
	}

	v, ok, err := k.Key.ToValue(scope)
	if err != nil || !ok {
		return "", nil, ok, err
	}

	if v.Kind() == value.UndefinedKind {
		return "", v, true, nil
	}

	s, err := value.ToString(v)
	if err != nil {
		return "", nil, false, err
	}
	return s, nil, true, k.checkKey(s)
}

type ErrKeyUndefined struct {
	Key       string
	Undefined value.Value
}

func (e *ErrKeyUndefined) Error() string {
	return fmt.Sprintf("undefined key %s (%s)", e.Key, e.Undefined)
}

func (k *FieldKey) Equals(scope Scope, key string) (_ bool, returnErr error) {
	if k.IsMatch() {
		return false, nil
	}

	v, ok, err := k.Key.ToValue(scope)
	if err != nil || !ok {
		return false, err
	} else if v.Kind() == value.UndefinedKind {
		k.disallowedKeys = append(k.disallowedKeys, key)
		return false, errors.NewErrEval(value.Position(k.Pos), &ErrKeyUndefined{
			Key:       key,
			Undefined: v,
		})
	}

	keyPattern, err := value.ToString(v)
	if err != nil || !ok {
		return ok, err
	}

	return keyPattern == key, nil
}

func (k *FieldKey) PatternMatches(scope Scope, key string) (_ bool, returnErr error) {
	if !k.IsMatch() {
		return false, nil
	}
	v, ok, err := k.Match.ToValue(scope)
	if err != nil || !ok {
		return ok, err
	}
	return value.Match(v, value.NewValue(key))
}
