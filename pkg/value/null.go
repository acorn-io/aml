package value

func NewNull() *Null {
	return &Null{}
}

type Null struct {
}

func (n *Null) Eq(right Value) (Value, error) {
	if right.Kind() == NullKind {
		return True, nil
	}
	return False, nil
}

func (n *Null) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

func (n *Null) Neq(right Value) (Value, error) {
	if right.Kind() == NullKind {
		return False, nil
	}
	return True, nil
}

func (n *Null) Kind() Kind {
	return NullKind
}

func (n *Null) String() string {
	return "null"
}

func (n *Null) NativeValue() (any, bool, error) {
	return nil, true, nil
}
