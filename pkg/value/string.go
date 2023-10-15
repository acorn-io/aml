package value

import (
	"fmt"
	"regexp"
)

type String string

func (s String) Kind() Kind {
	return StringKind
}

func (s String) NativeValue() (any, bool, error) {
	return (string)(s), true, nil
}

func (s String) Len() (Value, error) {
	return NewValue(len(s)), nil
}

func (s String) Index(key Value) (Value, bool, error) {
	idx, err := ToInt(key)
	if err != nil {
		return nil, false, err
	}
	if idx < 0 || int(idx) >= len(s) {
		return nil, false, fmt.Errorf("index %d out of bound, len %d", idx, len(s))
	}
	return NewValue(s[idx : idx+1]), true, nil
}

func (s String) Add(right Value) (Value, error) {
	if err := assertType(right, StringKind); err != nil {
		return nil, err
	}

	rightString, err := ToString(right)
	if err != nil {
		return nil, err
	}

	return NewValue(string(s) + rightString), nil
}

func (s String) Mat(right Value) (Value, error) {
	if err := assertType(right, StringKind); err != nil {
		return nil, err
	}
	rightString, err := ToString(right)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(rightString)
	if err != nil {
		return nil, err
	}

	m := re.FindStringIndex(string(s))
	return NewValue(m != nil), nil
}

func (s String) Nmat(right Value) (Value, error) {
	if err := assertType(right, StringKind); err != nil {
		return nil, err
	}
	rightString, err := ToString(right)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(rightString)
	if err != nil {
		return nil, err
	}
	return NewValue(!re.MatchString(string(s))), nil
}

func (s String) Eq(right Value) (Value, error) {
	if err := assertType(right, StringKind); err != nil {
		return nil, err
	}
	rightString, err := ToString(right)
	if err != nil {
		return nil, err
	}
	return NewValue(string(s) == rightString), nil
}

func (s String) Neq(right Value) (Value, error) {
	if err := assertType(right, StringKind); err != nil {
		return nil, err
	}
	rightString, err := ToString(right)
	if err != nil {
		return nil, err
	}
	return NewValue(string(s) != rightString), nil
}

func (s String) Match(right Value) (bool, error) {
	if err := assertType(right, StringKind); err != nil {
		return false, err
	}

	rightString, err := ToString(right)
	if err != nil {
		return false, err
	}

	re, err := regexp.Compile(string(s))
	if err != nil {
		return false, err
	}

	m := re.FindStringIndex(rightString)
	return m != nil, nil
}
