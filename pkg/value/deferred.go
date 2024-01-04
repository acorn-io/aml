package value

import (
	"context"
	"encoding/json"
	"fmt"
)

var _ AllOps = (*Deferred)(nil)

type Deferred struct {
	Resolve      func() (Value, error)
	KindResolver func() Kind
}

func (d Deferred) MarshalJSON() ([]byte, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func (d Deferred) Kind() Kind {
	return d.KindResolver()
}

func (d Deferred) Len() (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Len(v)
}

func (d Deferred) Add(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Add(v, right)
}

func (d Deferred) Sub(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Sub(v, right)
}

func (d Deferred) Not() (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Not(v)
}

func (d Deferred) Mul(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Mul(v, right)
}

func (d Deferred) Div(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Div(v, right)
}

func (d Deferred) And(right Valuer) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return And(v, right)
}

func (d Deferred) Or(right Valuer) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Or(v, right)
}

func (d Deferred) Lt(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Lt(v, right)
}

func (d Deferred) Le(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Le(v, right)
}

func (d Deferred) Gt(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Gt(v, right)
}

func (d Deferred) Ge(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Ge(v, right)
}

func (d Deferred) Eq(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Eq(v, right)
}

func (d Deferred) Neq(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Neq(v, right)
}

func (d Deferred) Mat(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Mat(v, right)
}

func (d Deferred) Nmat(right Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Nmat(v, right)
}

func (d Deferred) Slice(start, end int) (Value, bool, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, false, err
	}
	return Slice(v, NewValue(start), NewValue(end))
}

func (d Deferred) Call(ctx context.Context, args []CallArgument) (Value, bool, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, false, err
	}
	return Call(ctx, v, args...)
}

func (d Deferred) Index(key Value) (Value, bool, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, false, err
	}
	return Index(v, key)
}

func (d Deferred) LookupValue(key Value) (Value, bool, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, false, err
	}
	return Lookup(v, key)
}

func (d Deferred) ToFloat() (float64, error) {
	v, err := d.Resolve()
	if err != nil {
		return 0, err
	}
	return ToFloat(v)
}

func (d Deferred) ToInt() (int64, error) {
	v, err := d.Resolve()
	if err != nil {
		return 0, err
	}
	return ToInt(v)
}

func (d Deferred) NativeValue() (any, bool, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, false, err
	}
	return NativeValue(v)
}

func (d Deferred) Merge(val Value) (Value, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Merge(v, val)
}

func (d Deferred) Default() (Value, bool, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, false, err
	}
	return DefaultValue(v)
}

func (d Deferred) Match(value Value) (bool, error) {
	v, err := d.Resolve()
	if err != nil {
		return false, err
	}
	return Match(v, value)
}

func (d Deferred) Keys() ([]string, error) {
	v, err := d.Resolve()
	if err != nil {
		return nil, err
	}
	return Keys(v)
}

func (d Deferred) String() string {
	v, err := d.Resolve()
	if err != nil {
		return "deferred value"
	}
	return fmt.Sprintf("%s", v)
}
