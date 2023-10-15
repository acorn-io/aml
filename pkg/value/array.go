package value

import (
	"encoding/json"
	"fmt"
)

type Array []Value

func NewArray(objs []any) Array {
	a := make([]Value, 0, len(objs))
	for _, obj := range objs {
		a = append(a, NewValue(obj))
	}
	return a
}

func (a Array) GetUndefined() Value {
	for _, item := range a {
		if undef := GetUndefined(item); undef != nil {
			return undef
		}
	}
	return nil
}

func (a Array) IsDefined() bool {
	for _, item := range a {
		if !IsDefined(item) {
			return false
		}
	}
	return true
}

func (a Array) Eq(right Value) (Value, error) {
	if right.Kind() != ArrayKind {
		return nil, fmt.Errorf("can not compare array with kind %s", right.Kind())
	}

	leftLen := int64(len(a))

	rightLenVal, err := Len(right)
	if err != nil {
		return nil, err
	}

	rightLen, err := ToInt(rightLenVal)
	if err != nil {
		return nil, err
	}

	if leftLen != rightLen {
		return False, err
	}

	for i := int64(0); i < leftLen; i++ {
		rightValue, ok, err := Index(right, NewValue(i))
		if err != nil {
			return nil, err
		} else if !ok {
			return False, err
		}
		bValue, err := Eq(a[i], rightValue)
		if err != nil {
			return nil, err
		}
		b, err := ToBool(bValue)
		if err != nil {
			return nil, err
		}
		if !b {
			return False, err
		}
	}

	return True, nil
}

func (a Array) Add(right Value) (Value, error) {
	if right.Kind() != ArrayKind {
		return nil, fmt.Errorf("can not add array to invalid kind %s", right.Kind())
	}

	rightArray, err := ToValueArray(right)
	if err != nil {
		return nil, err
	}

	return append(a, rightArray...), nil
}

func (a Array) Slice(start, end int) (Value, bool, error) {
	if start >= len(a) || end > len(a) || start < 0 || end < 0 || start > end {
		return nil, false, fmt.Errorf("index %d:%d out of bound, len %d", start, end, len(a))
	}
	return a[start:end], true, nil
}

func (a Array) Index(idxValue Value) (Value, bool, error) {
	idx, err := ToInt(idxValue)
	if err != nil {
		return nil, false, err
	}
	if int(idx) >= len(a) || idx < 0 {
		return nil, false, fmt.Errorf("index %d out of bound, len %d", idx, len(a))
	}
	return a[idx], true, nil
}

func (a Array) ToValues() []Value {
	return a
}

func (a Array) Kind() Kind {
	return ArrayKind
}

func (a Array) String() string {
	data, _ := json.Marshal(a)
	return string(data)
}

func (a Array) NativeValue() (any, bool, error) {
	result := make([]any, 0, len(a))
	for _, v := range a {
		item, ok, err := NativeValue(v)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			continue
		}
		result = append(result, item)
	}
	return result, true, nil
}

func (a Array) Len() (Value, error) {
	return NewValue(len(a)), nil
}
