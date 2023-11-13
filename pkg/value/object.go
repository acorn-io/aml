package value

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

type Object struct {
	Entries []Entry
}

func (n *Object) Len() (Value, error) {
	return NewValue(len(n.Entries)), nil
}

func NewObject(data map[string]any) *Object {
	o := &Object{}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		o.Entries = append(o.Entries, Entry{
			Key:   key,
			Value: NewValue(data[key]),
		})
	}

	return o
}

func (n *Object) GetUndefined() Value {
	for _, entry := range n.Entries {
		if undef := GetUndefined(entry.Value); undef != nil {
			return undef
		}
	}
	return nil
}

func (n *Object) IsDefined() bool {
	for _, entry := range n.Entries {
		if !IsDefined(entry.Value) {
			return false
		}
	}
	return true
}

func (n *Object) LookupValue(key Value) (Value, bool, error) {
	if sKey, ok := key.(String); ok {
		for _, e := range n.Entries {
			if e.Key == (string)(sKey) {
				return e.Value, true, nil
			}
		}
		return nil, false, nil
	}

	for _, e := range n.Entries {
		b, err := Eq(key, NewValue(e.Key))
		if err != nil {
			return nil, false, err
		}

		if b, err := ToBool(b); err != nil {
			return nil, false, err
		} else if b {
			return e.Value, true, nil
		}
	}

	return nil, false, nil
}

func (n *Object) Add(right Value) (Value, error) {
	if right.Kind() != ObjectKind {
		return nil, fmt.Errorf("can not add kind %s and %s", n.Kind(), right.Kind())
	}

	v, _, err := MergeObjects(n, right, true, func(left, right Value) (newValue Value, changed bool, _ error) {
		if left.Kind() == ObjectKind && right.Kind() == ObjectKind {
			nv, err := Add(left, right)
			return nv, false, err
		}
		return right, false, nil
	})
	return v, err
}

func (n *Object) Eq(right Value) (Value, error) {
	if right.Kind() != ObjectKind {
		return nil, fmt.Errorf("can not compare object with kind %s", right.Kind())
	}

	rightKeys, err := Keys(right)
	if err != nil {
		return nil, err
	}

	leftKeys, err := n.Keys()
	if err != nil {
		return nil, err
	}

	if len(rightKeys) != len(leftKeys) {
		return False, nil
	}

	sort.Strings(rightKeys)
	sort.Strings(leftKeys)

	for i, key := range rightKeys {
		if leftKeys[i] != key {
			return False, nil
		}

		leftValue, ok, err := n.LookupValue(NewValue(key))
		if err != nil || !ok {
			return False, err
		}

		rightValue, ok, err := Lookup(right, NewValue(key))
		if err != nil || !ok {
			return False, err
		}

		if undef := IsUndefined(leftValue, rightValue); undef != nil {
			return undef, nil
		}

		bValue, err := Eq(leftValue, rightValue)
		if err != nil {
			return nil, err
		}

		if undef := IsUndefined(bValue); undef != nil {
			return undef, nil
		}

		b, err := ToBool(bValue)
		if err != nil {
			return nil, err
		}
		if !b {
			return False, nil
		}
	}

	return True, nil
}

func (n *Object) Kind() Kind {
	return ObjectKind
}

func (n *Object) MarshalJSON() ([]byte, error) {
	result := map[string]any{}
	for _, entry := range n.Entries {
		result[entry.Key] = entry.Value
	}
	return json.Marshal(result)
}

func (n *Object) String() string {
	data, _ := n.MarshalJSON()
	return string(data)
}

func (n *Object) NativeValue() (any, bool, error) {
	result := map[string]any{}
	for _, entry := range n.Entries {
		nv, ok, err := NativeValue(entry.Value)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			continue
		}
		result[entry.Key] = nv
	}
	return result, true, nil
}

func (n *Object) Keys() ([]string, error) {
	result := make([]string, 0, len(n.Entries))
	for _, entry := range n.Entries {
		result = append(result, entry.Key)
	}
	return result, nil
}

func Entries(val Value) (result []Entry, _ error) {
	keys, err := Keys(val)
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		v, ok, err := Lookup(val, NewValue(key))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		result = append(result, Entry{
			Key:   key,
			Value: v,
		})
	}

	return
}

type ObjectMerge func(left, right Value) (newValue Value, changed bool, _ error)

func MergeObjectMerger(left, right Value) (newValue Value, changed bool, _ error) {
	v, err := Merge(left, right)
	return v, false, err
}

func IsObjectLike(v Value) bool {
	if _, ok := v.(Keyser); !ok {
		return false
	}
	if _, ok := v.(LookupValue); !ok {
		return false
	}
	return true
}

func MergeObjects(left, right Value, allowNewKeys bool, merger ObjectMerge) (merged Value, changed bool, _ error) {
	var (
		result   []Entry
		keysSeen = map[string]int{}
	)

	leftEntries, err := Entries(left)
	if err != nil {
		return nil, false, err
	}

	for _, entry := range leftEntries {
		keysSeen[entry.Key] = len(result)
		result = append(result, entry)
	}

	keys, err := Keys(right)
	if err != nil {
		return nil, false, fmt.Errorf("failed to merge kind %s with %s: %w", ObjectKind, right.Kind(), err)
	}

	var (
		mergeChanged bool
	)
	for _, key := range keys {
		rightValue, ok, err := Lookup(right, NewValue(key))
		if err != nil {
			return nil, false, err
		}
		if !ok {
			continue
		}

		if i, ok := keysSeen[key]; ok {
			rightValue, mergeChanged, err = merger(result[i].Value, rightValue)
			if err != nil {
				return nil, false, err
			}
			if mergeChanged {
				changed = true
			}
			result[i].Value = rightValue
		} else if allowNewKeys {
			changed = true
			result = append(result, Entry{
				Key:   key,
				Value: rightValue,
			})
		} else {
			return nil, false, &ErrUnknownField{
				Key: key,
			}
		}
	}

	return &Object{
		Entries: result,
	}, changed, nil
}

func (n *Object) Merge(right Value) (Value, error) {
	if err := AssertKindsMatch(n, right); err != nil {
		return nil, err
	}

	v, _, err := MergeObjects(n, right, true, MergeObjectMerger)
	return v, err
}

type Entry struct {
	Key   string
	Value Value
}

type ObjectFunc struct {
	Self *Object
	Func Value
}

func (o ObjectFunc) Kind() Kind {
	return FuncKind
}

func (o ObjectFunc) Eq(right Value) (Value, error) {
	if rf, ok := right.(ObjectFunc); ok {
		return Eq(o.Func, rf.Func)
	}
	return False, nil
}

func (o ObjectFunc) Call(ctx context.Context, args []CallArgument) (Value, bool, error) {
	return Call(ctx, o.Func, append(args, CallArgument{
		Self:  true,
		Value: o.Self,
	})...)
}
