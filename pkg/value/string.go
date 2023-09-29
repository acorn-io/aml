package value

import "regexp"

type String string

func (s String) Kind() Kind {
	return StringKind
}

func (s String) NativeValue() (any, bool, error) {
	return (string)(s), true, nil
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
