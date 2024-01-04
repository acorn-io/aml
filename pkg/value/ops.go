package value

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func NewValue(v any) Value {
	if v == nil {
		return NewNull()
	}
	switch x := v.(type) {
	case Value:
		return x
	case json.Number:
		return Number(x)
	case int:
		return Number(strconv.Itoa(x))
	case int8:
		return Number(strconv.Itoa(int(x)))
	case int32:
		return Number(strconv.Itoa(int(x)))
	case int64:
		return Number(strconv.Itoa(int(x)))
	case float32:
		return Number(strconv.FormatFloat(float64(x), 'f', -1, 64))
	case float64:
		return Number(strconv.FormatFloat(x, 'f', -1, 64))
	case string:
		return (String)(x)
	case bool:
		return (Boolean)(x)
	case map[string]any:
		return NewObject(x)
	case map[string]Value:
		data := map[string]any{}
		for k, v := range x {
			data[k] = v
		}
		return NewObject(data)
	case []any:
		return NewArray(x)
	case []Value:
		return Array(x)
	case []string:
		var ret []any
		for _, i := range x {
			ret = append(ret, i)
		}
		return NewArray(ret)
	case nil:
		return NewNull()
	default:
		panic(fmt.Sprintf("invalid value: %T", v))
	}
}

func ToKind(v Value, kind Kind) (any, error) {
	if err := assertType(v, kind); err != nil {
		return nil, err
	}
	nv, ok, err := NativeValue(v)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("value of kind %s did not produce a native value as expected", kind)
	}
	return nv, nil
}

func ToValueArray(v Value) ([]Value, error) {
	arrayValue, ok := v.(interface {
		ToValues() []Value
	})
	if ok {
		return arrayValue.ToValues(), nil
	}
	array, err := ToArray(v)
	if err != nil {
		return nil, err
	}

	result := make([]Value, 0, len(array))
	for _, item := range array {
		result = append(result, NewValue(item))
	}

	return result, nil
}

func ToArray(v Value) ([]any, error) {
	ret, err := ToKind(v, ArrayKind)
	if err != nil {
		return nil, err
	}
	return ret.([]any), nil
}

func ToString(v Value) (string, error) {
	ret, err := ToKind(v, StringKind)
	if err != nil {
		return "", err
	}
	return ret.(string), nil
}

type ToInter interface {
	ToInt() (int64, error)
}

func ToInt(v Value) (int64, error) {
	if toInt, ok := v.(ToInter); ok {
		return toInt.ToInt()
	}
	return 0, fmt.Errorf("value kind %s can not be converted to an int", v.Kind())
}

type ToFloater interface {
	ToFloat() (float64, error)
}

func ToFloat(v Value) (float64, error) {
	if toInt, ok := v.(ToFloater); ok {
		return toInt.ToFloat()
	}
	return 0, fmt.Errorf("value kind %s can not be converted to a float", v.Kind())
}

type LookupValue interface {
	LookupValue(key Value) (Value, bool, error)
}

func IsLookupSupported(left Value) bool {
	if undef := IsUndefined(left); undef != nil {
		return true
	}
	_, ok := left.(LookupValue)
	return ok
}

func Lookup(left, key Value) (Value, bool, error) {
	if undef := IsUndefined(left, key); undef != nil {
		return undef, true, nil
	}
	adder, ok := left.(LookupValue)
	if ok {
		return adder.LookupValue(key)
	}
	return nil, false, fmt.Errorf("value kind %s does not support lookup operation", left.Kind())
}

type Indexer interface {
	Index(key Value) (Value, bool, error)
}

func Index(left, key Value) (Value, bool, error) {
	if undef := IsUndefined(left, key); undef != nil {
		return undef, true, nil
	}
	if index, ok := left.(Indexer); ok {
		return index.Index(key)
	}
	return nil, false, fmt.Errorf("value kind %s does not support index operation", left.Kind())
}

type Lener interface {
	Len() (Value, error)
}

func Len(left Value) (Value, error) {
	if undef := IsUndefined(left); undef != nil {
		return undef, nil
	}
	if index, ok := left.(Lener); ok {
		return index.Len()
	}
	return nil, fmt.Errorf("value kind %s does not support len operation", left.Kind())
}

type Slicer interface {
	Slice(start, end int) (Value, bool, error)
}

func GetUndefined(vals ...Value) Value {
	for _, val := range vals {
		if isUndef, ok := val.(interface {
			GetUndefined() Value
		}); ok {
			return isUndef.GetUndefined()
		} else if val != nil && val.Kind() == UndefinedKind {
			return val
		}
	}
	return nil
}

// IsUndefined is a small helper to check if any of the passed values are undefined
func IsUndefined(vals ...Value) Value {
	for _, val := range vals {
		if val != nil && val.Kind() == UndefinedKind {
			return val
		}
	}
	return nil
}

func Slice(left, start, end Value) (Value, bool, error) {
	if undef := IsUndefined(left, start, end); undef != nil {
		return undef, true, nil
	}
	if index, ok := left.(Slicer); ok {
		var (
			startInt, endInt int64
			err              error
		)
		if start != nil {
			startInt, err = ToInt(start)
			if err != nil {
				return nil, false, err
			}
		}
		if end == nil {
			lenVal, err := Len(left)
			if err != nil {
				return nil, false, err
			}
			endInt, err = ToInt(lenVal)
			if err != nil {
				return nil, false, err
			}
		} else {
			endInt, err = ToInt(end)
			if err != nil {
				return nil, false, err
			}
		}
		return index.Slice(int(startInt), int(endInt))
	}
	return nil, false, fmt.Errorf("value kind %s does not support slice operation", left.Kind())
}

func ToBool(v Value) (bool, error) {
	ret, err := ToKind(v, BoolKind)
	if err != nil {
		return false, err
	}
	return ret.(bool), nil
}

func UnaryOperation(op Operator, val Value) (Value, error) {
	if undef := IsUndefined(val); undef != nil {
		return undef, nil
	}

	switch op {
	case AddOp, SubOp:
		return BinaryOperation(op, NewValue(0), func() (Value, error) {
			return val, nil
		})
	case NotOp:
		return Not(val)
	default:
		return nil, fmt.Errorf("unsupported unary operator %s", op)
	}
}

type Operator string

const (
	AddOp  = Operator("+")
	SubOp  = Operator("-")
	MulOp  = Operator("*")
	DivOp  = Operator("/")
	AndOp  = Operator("&&")
	OrOp   = Operator("||")
	LtOp   = Operator("<")
	LeOp   = Operator("<=")
	GtOp   = Operator(">")
	GeOp   = Operator(">=")
	EqOp   = Operator("==")
	NeqOp  = Operator("!=")
	NotOp  = Operator("!")
	MatOp  = Operator("=~")
	NmatOp = Operator("!~")
)

type AllUnaryOps interface {
	Adder
	Suber
	Noter
}

type AllBinaryOps interface {
	Adder
	Suber
	Muler
	Diver
	DeferredAnder
	DeferredOrer
	Lter
	Leer
	Gter
	Geer
	Eqer
	Neqer
	Mater
	Nmater
}

type AllOps interface {
	AllUnaryOps
	AllBinaryOps

	Slicer
	Caller
	Indexer
	Lener
	LookupValue
	ToFloater
	ToInter
	ToNative
	Merger
	Defaulter
	Matcher
	Keyser

	// Don't want to include this by default, but you may want to in your implementation of this interface
	// RightMerger
}

type Valuer func() (Value, error)

func BinaryOperation(op Operator, left Value, deferredRight Valuer) (Value, error) {
	if undef := IsUndefined(left); undef != nil {
		return undef, nil
	}

	var (
		right Value
		err   error
	)

	if op != AndOp && op != OrOp {
		right, err = deferredRight()
		if err != nil {
			return nil, err
		}

		if undef := IsUndefined(right); undef != nil {
			return undef, nil
		}
	}

	switch op {
	case AddOp:
		return Add(left, right)
	case SubOp:
		return Sub(left, right)
	case MulOp:
		return Mul(left, right)
	case DivOp:
		return Div(left, right)
	case AndOp:
		return And(left, deferredRight)
	case OrOp:
		return Or(left, deferredRight)
	case LtOp:
		return Lt(left, right)
	case LeOp:
		return Le(left, right)
	case GtOp:
		return Gt(left, right)
	case GeOp:
		return Ge(left, right)
	case EqOp:
		return Eq(left, right)
	case NeqOp:
		return Neq(left, right)
	case MatOp:
		return Mat(left, right)
	case NmatOp:
		return Nmat(left, right)
	default:
		return nil, fmt.Errorf("unsupported operator %s", op)
	}
}

type Adder interface {
	Add(right Value) (Value, error)
}

func Add(left, right Value) (Value, error) {
	adder, ok := left.(Adder)
	if ok {
		return adder.Add(right)
	}
	return nil, fmt.Errorf("value kind %s does not support + operation", left.Kind())
}

type Noter interface {
	Not() (Value, error)
}

func Not(right Value) (Value, error) {
	adder, ok := right.(Noter)
	if ok {
		return adder.Not()
	}
	return nil, fmt.Errorf("value kind %s does not support ! operation", right.Kind())
}

type Suber interface {
	Sub(right Value) (Value, error)
}

func Sub(left, right Value) (Value, error) {
	adder, ok := left.(Suber)
	if ok {
		return adder.Sub(right)
	}
	return nil, fmt.Errorf("value kind %s does not support - operation", left.Kind())
}

type Muler interface {
	Mul(right Value) (Value, error)
}

func Mul(left, right Value) (Value, error) {
	adder, ok := left.(Muler)
	if ok {
		return adder.Mul(right)
	}
	return nil, fmt.Errorf("value kind %s does not support * operation", left.Kind())
}

type Diver interface {
	Div(right Value) (Value, error)
}

func Div(left, right Value) (Value, error) {
	adder, ok := left.(Diver)
	if ok {
		return adder.Div(right)
	}
	return nil, fmt.Errorf("value kind %s does not support / operation", left.Kind())
}

type Ander interface {
	And(right Value) (Value, error)
}

type DeferredAnder interface {
	And(right Valuer) (Value, error)
}

func And(left Value, right Valuer) (Value, error) {
	adder, ok := left.(Ander)
	if ok {
		right, err := right()
		if err != nil {
			return nil, err
		}
		if undef := IsUndefined(right); undef != nil {
			return undef, nil
		}
		return adder.And(right)
	}
	deferredAdder, ok := left.(DeferredAnder)
	if ok {
		return deferredAdder.And(right)
	}
	return nil, fmt.Errorf("value kind %s does not support && operation", left.Kind())
}

type Orer interface {
	Or(right Value) (Value, error)
}

type DeferredOrer interface {
	Or(right Valuer) (Value, error)
}

func Or(left Value, right Valuer) (Value, error) {
	adder, ok := left.(Orer)
	if ok {
		right, err := right()
		if err != nil {
			return nil, err
		}
		if undef := IsUndefined(right); undef != nil {
			return undef, nil
		}
		return adder.Or(right)
	}
	deferredOrer, ok := left.(DeferredOrer)
	if ok {
		return deferredOrer.Or(right)
	}
	return nil, fmt.Errorf("value kind %s does not support || operation", left.Kind())
}

type Leer interface {
	Le(right Value) (Value, error)
}

func Le(left, right Value) (Value, error) {
	adder, ok := left.(Leer)
	if ok {
		return adder.Le(right)
	}
	return nil, fmt.Errorf("value kind %s does not support <= operation", left.Kind())
}

type Lter interface {
	Lt(right Value) (Value, error)
}

func Lt(left, right Value) (Value, error) {
	adder, ok := left.(Lter)
	if ok {
		return adder.Lt(right)
	}
	return nil, fmt.Errorf("value kind %s does not support < operation", left.Kind())
}

type Gter interface {
	Gt(right Value) (Value, error)
}

func Gt(left, right Value) (Value, error) {
	adder, ok := left.(Gter)
	if ok {
		return adder.Gt(right)
	}
	return nil, fmt.Errorf("value kind %s does not support > operation", left.Kind())
}

type Geer interface {
	Ge(right Value) (Value, error)
}

func Ge(left, right Value) (Value, error) {
	adder, ok := left.(Geer)
	if ok {
		return adder.Ge(right)
	}
	return nil, fmt.Errorf("value kind %s does not support >= operation", left.Kind())
}

type Eqer interface {
	Eq(right Value) (Value, error)
}

func eqToBool(left, right Value) (bool, error) {
	if v, err := Eq(left, right); err != nil {
		return false, err
	} else if b, err := ToBool(v); err != nil {
		return false, err
	} else {
		return b, nil
	}
}

func Eq(left, right Value) (Value, error) {
	adder, ok := left.(Eqer)
	if ok {
		return adder.Eq(right)
	}
	return nil, fmt.Errorf("value kind %s does not support == operation", left.Kind())
}

type Neqer interface {
	Neq(right Value) (Value, error)
}

func Neq(left, right Value) (Value, error) {
	adder, ok := left.(Neqer)
	if ok {
		return adder.Neq(right)
	}
	nadder, ok := left.(Eqer)
	if ok {
		v, err := nadder.Eq(right)
		if err != nil {
			return nil, err
		}
		return UnaryOperation(NotOp, v)
	}
	return nil, fmt.Errorf("value kind %s does not support != operation", left.Kind())
}

type Mater interface {
	Mat(right Value) (Value, error)
}

func Mat(left, right Value) (Value, error) {
	adder, ok := left.(Mater)
	if ok {
		return adder.Mat(right)
	}
	return nil, fmt.Errorf("value kind %s does not support =~ operation", left.Kind())
}

type Nmater interface {
	Nmat(right Value) (Value, error)
}

func Nmat(left, right Value) (Value, error) {
	adder, ok := left.(Nmater)
	if ok {
		return adder.Nmat(right)
	}
	return nil, fmt.Errorf("value kind %s does not support !~ operation", left.Kind())
}

type Keyser interface {
	Keys() ([]string, error)
}

func KeysIfSupported(right Value) ([]string, error) {
	adder, ok := right.(Keyser)
	if ok {
		return adder.Keys()
	}
	return nil, nil
}

func Keys(right Value) ([]string, error) {
	adder, ok := right.(Keyser)
	if ok {
		return adder.Keys()
	}
	return nil, fmt.Errorf("value kind %s does not support keys operation", right.Kind())
}

type ToNative interface {
	NativeValue() (any, bool, error)
}

func NativeValue(v Value) (any, bool, error) {
	if nv, ok := v.(ToNative); ok {
		return nv.NativeValue()
	}
	return nil, false, nil
}

type Matcher interface {
	Match(value Value) (bool, error)
}

func Match(pattern, value Value) (bool, error) {
	if nv, ok := pattern.(Matcher); ok {
		return nv.Match(value)
	}
	return false, fmt.Errorf("value kind %s does not support matching", pattern.Kind())
}
