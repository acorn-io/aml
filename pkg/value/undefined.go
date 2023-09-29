package value

import "fmt"

type Position struct {
	Filename string
	Offset   int
	Line     int
	Column   int
}

var NoPosition = Position{}

func (p Position) String() string {
	if p.Filename == "" {
		return fmt.Sprintf("%d:%d", p.Line, p.Column)
	}
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}

func IsDefined(val Value) bool {
	if v, ok := val.(interface {
		IsDefined() bool
	}); ok {
		return v.IsDefined()
	}
	return true
}

type Undefined struct {
	Pos Position
}

func (u Undefined) IsDefined() bool {
	return false
}

func (u Undefined) NativeValue() (any, bool, error) {
	return nil, false, fmt.Errorf("undefined value from %s can not be turned into a native go value", u.Pos)
}

func (u Undefined) String() string {
	if u.Pos.Offset != 0 {
		return fmt.Sprint(u.Pos)
	}
	return "undefined"
}

func (u Undefined) LookupValue(key Value) (Value, bool, error) {
	return u, true, nil
}

func (u Undefined) Kind() Kind {
	return UndefinedKind
}

func (u Undefined) Merge(val Value) (Value, error) {
	return u, nil
}
