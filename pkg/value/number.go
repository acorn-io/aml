package value

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Number string

var multipliers map[string]float64

func init() {
	multipliers = map[string]float64{}
	for i, v := range []string{"k", "m", "g", "t", "p"} {
		// 10^3, 10^6, 10^9, etc
		decimal := math.Pow10((i + 1) * 3)
		// 2^(10*1), 2^(10*2), 2^(10*3), etc
		binary := math.Pow(2, float64((i+1)*10))

		// k, m, g, t, p
		multipliers[v] = decimal
		// ki, mi, gi, ti, pi
		multipliers[v+"i"] = binary

		v = strings.ToUpper(v)
		// K, M, G, T, P
		multipliers[v] = decimal
		// Ki, Mi, Gi, Ti, Pi
		multipliers[v+"i"] = binary
	}
}

func (n Number) Kind() Kind {
	return NumberKind
}

func (n Number) NativeValue() (any, bool, error) {
	return n, true, nil
}

func toNum(n Value) (reti *int64, retf *float64, err error) {
	i, err := ToInt(n)
	if err == nil {
		reti = &i
	}

	f, err := ToFloat(n)
	if err == nil {
		retf = &f
	}

	if reti == nil && retf == nil {
		return nil, nil, fmt.Errorf("invalid number %s, not parsable as int or float", n)
	}

	return
}

func (n Number) binCompare(right Value, opName string, intFunc func(int64, int64) bool, floatFunc func(float64, float64) bool) (Value, error) {
	if right.Kind() != NumberKind {
		return nil, fmt.Errorf("can not compare (%s) number to invalid kind %s", opName, right.Kind())
	}

	li, lf, err := toNum(n)
	if err != nil {
		return nil, err
	}

	ri, rf, err := toNum(right)
	if err != nil {
		return nil, err
	}

	if li != nil && ri != nil {
		return NewValue(intFunc(*li, *ri)), nil
	} else if lf != nil && rf != nil {
		return NewValue(floatFunc(*lf, *rf)), nil
	} else {
		return nil, fmt.Errorf("can not compare (%s) incompatible numbers %s and %s", opName, n, right)
	}
}

func (n Number) binOp(right Value, opName string, intFunc func(int64, int64) int64, floatFunc func(float64, float64) float64) (Value, error) {
	if right.Kind() != NumberKind {
		return nil, fmt.Errorf("can not %s number to invalid kind %s", opName, right.Kind())
	}

	li, lf, err := toNum(n)
	if err != nil {
		return nil, err
	}

	ri, rf, err := toNum(right)
	if err != nil {
		return nil, err
	}

	if li != nil && ri != nil {
		return NewValue(intFunc(*li, *ri)), nil
	} else if lf != nil && rf != nil {
		return NewValue(floatFunc(*lf, *rf)), nil
	} else {
		return nil, fmt.Errorf("can not %s incompatible numbers %s and %s", opName, n, right)
	}
}

func (n Number) Sub(right Value) (Value, error) {
	return n.binOp(right, "subtract", func(i int64, i2 int64) int64 {
		return i - i2
	}, func(f float64, f2 float64) float64 {
		return f - f2
	})
}

func (n Number) Add(right Value) (Value, error) {
	return n.binOp(right, "add", func(i int64, i2 int64) int64 {
		return i + i2
	}, func(f float64, f2 float64) float64 {
		return f + f2
	})
}

func (n Number) Mul(right Value) (Value, error) {
	return n.binOp(right, "multiply", func(i int64, i2 int64) int64 {
		return i * i2
	}, func(f float64, f2 float64) float64 {
		return f * f2
	})
}

func (n Number) Div(right Value) (Value, error) {
	return n.binOp(right, "divide", func(i int64, i2 int64) int64 {
		return i / i2
	}, func(f float64, f2 float64) float64 {
		return f / f2
	})
}

func (n Number) Lt(right Value) (Value, error) {
	return n.binCompare(right, "less than", func(i int64, i2 int64) bool {
		return i < i2
	}, func(f float64, f2 float64) bool {
		return f < f2
	})
}

func (n Number) Gt(right Value) (Value, error) {
	return n.binCompare(right, "greater than", func(i int64, i2 int64) bool {
		return i > i2
	}, func(f float64, f2 float64) bool {
		return f > f2
	})
}

func (n Number) Le(right Value) (Value, error) {
	return n.binCompare(right, "less than equal", func(i int64, i2 int64) bool {
		return i <= i2
	}, func(f float64, f2 float64) bool {
		return f <= f2
	})
}

func (n Number) Ge(right Value) (Value, error) {
	return n.binCompare(right, "greater than equal", func(i int64, i2 int64) bool {
		return i >= i2
	}, func(f float64, f2 float64) bool {
		return f >= f2
	})
}

func (n Number) Eq(right Value) (Value, error) {
	if right.Kind() != NumberKind {
		return False, nil
	}
	return n.binCompare(right, "equals", func(i int64, i2 int64) bool {
		return i == i2
	}, func(f float64, f2 float64) bool {
		return f == f2
	})
}

func (n Number) Neq(right Value) (Value, error) {
	if right.Kind() != NumberKind {
		return False, nil
	}
	return n.binCompare(right, "not equals", func(i int64, i2 int64) bool {
		return i != i2
	}, func(f float64, f2 float64) bool {
		return f != f2
	})
}

func extraMultiplierAndNormalize(n string) (string, float64) {
	for suffix, multiplier := range multipliers {
		if strings.HasSuffix(n, suffix) {
			return strings.ReplaceAll(strings.TrimSuffix(n, suffix), "_", ""), multiplier
		}
	}
	return strings.ReplaceAll(n, "_", ""), 1
}

func (n Number) ToInt() (int64, error) {
	str, m := extraMultiplierAndNormalize(string(n))
	ret, err := strconv.ParseInt(str, 10, 64)
	return ret * int64(m), err
}

func (n Number) ToFloat() (float64, error) {
	str, m := extraMultiplierAndNormalize(string(n))
	ret, err := strconv.ParseFloat(str, 64)
	return ret * m, err
}

func (n Number) MarshalJSON() ([]byte, error) {
	str, m := extraMultiplierAndNormalize(string(n))
	// avoid converting to a number if we can so that we might not accidentally lose precision or something
	if len(n) == len(str) {
		return []byte(n), nil
	} else if m == 1 {
		return json.Marshal(json.Number(str))
	}

	i, err := n.ToInt()
	if err == nil {
		return json.Marshal(i)
	}

	f, err := n.ToFloat()
	if err != nil {
		return nil, err
	}
	return json.Marshal(f)
}
