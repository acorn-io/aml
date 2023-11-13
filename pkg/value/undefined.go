package value

import "fmt"

type Position struct {
	Filename string `json:"filename,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
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
	Err error
	Pos Position
}

func (u Undefined) Eq(v Value) (Value, error) {
	return u, nil
}

func (u Undefined) IsDefined() bool {
	return false
}

func (u Undefined) NativeValue() (any, bool, error) {
	if u.Err != nil {
		return nil, false, u.Err
	}
	return nil, false, fmt.Errorf("undefined value from %s (typically caused by a missing key or circular reference) can not be turned into a native value", u.Pos)
}

func (u Undefined) String() string {
	if u.Err != nil {
		return u.Err.Error()
	}
	if u.Pos.Offset != 0 {
		return fmt.Sprint("undefined ", u.Pos)
	}
	return "undefined"
}

func (u Undefined) LookupValue(key Value) (Value, bool, error) {
	return u, true, nil
}

func (u Undefined) Kind() Kind {
	return UndefinedKind
}

func (u Undefined) RightMergePriority() RightMergePriority {
	return UndefinedPriority
}

func (u Undefined) RightMerge(val Value) (Value, error) {
	return u, nil
}

func (u Undefined) Merge(val Value) (Value, error) {
	return u, nil
}
