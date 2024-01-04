package value

import "fmt"

var (
	True  = Boolean(true)
	False = Boolean(false)
)

type Boolean bool

func (n Boolean) String() string {
	return fmt.Sprint(bool(n))
}

func (n Boolean) IsTrue() bool {
	return bool(n)
}

func (n Boolean) Kind() Kind {
	return BoolKind
}

func (n Boolean) NativeValue() (any, bool, error) {
	return (bool)(n), true, nil
}

func (n Boolean) Not() (Value, error) {
	return NewValue(!n), nil
}

func (n Boolean) Eq(right Value) (Value, error) {
	if err := assertType(right, BoolKind); err != nil {
		return nil, err
	}
	rightBool, err := ToBool(right)
	if err != nil {
		return nil, err
	}
	return NewValue(bool(n) == rightBool), nil
}

func (n Boolean) Neq(right Value) (Value, error) {
	if err := assertType(right, BoolKind); err != nil {
		return nil, err
	}
	rightBool, err := ToBool(right)
	if err != nil {
		return nil, err
	}
	return NewValue(bool(n) != rightBool), nil
}

func (n Boolean) And(deferredRight Valuer) (Value, error) {
	if !n {
		return False, nil
	}
	right, err := deferredRight()
	if err != nil {
		return nil, err
	}
	if undef := IsUndefined(right); undef != nil {
		return undef, nil
	}
	b, err := ToBool(right)
	if err != nil {
		return nil, err
	}
	return NewValue((bool)(n) && b), nil
}

func (n Boolean) Or(deferredRight Valuer) (Value, error) {
	if n {
		return True, nil
	}
	right, err := deferredRight()
	if err != nil {
		return nil, err
	}
	if undef := IsUndefined(right); undef != nil {
		return undef, nil
	}
	b, err := ToBool(right)
	if err != nil {
		return nil, err
	}
	return NewValue((bool)(n) || b), nil
}
