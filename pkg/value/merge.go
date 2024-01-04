package value

import "fmt"

func mergeNative(left, right Value) (Value, error) {
	if err := AssertKindsMatch(left, right); err != nil {
		return nil, err
	}
	bValue, err := Eq(left, right)
	if err != nil {
		return nil, err
	}
	b, err := ToBool(bValue)
	if err != nil {
		return nil, err
	}
	if !b {
		return nil, fmt.Errorf("can not override value [%s] with [%s]", left, right)
	}
	return right, nil
}

type RightMergePriority int

const (
	UndefinedPriority   = RightMergePriority(20)
	LoopControlPriority = RightMergePriority(10)
	DefaultedPriority   = RightMergePriority(0)
	TypeSchemaPriority  = RightMergePriority(5)
)

type RightMerger interface {
	RightMergePriority() RightMergePriority
	RightMerge(val Value) (Value, error)
}

type Merger interface {
	Merge(val Value) (Value, error)
}

func rightMerge(left Value, right Value) (Value, bool, error) {
	rm, ok := right.(RightMerger)
	if !ok {
		return nil, false, nil
	}

	lrm, lrmOk := left.(RightMerger)
	lm, lmOk := left.(Merger)
	if lrmOk && lmOk && lrm.RightMergePriority() >= rm.RightMergePriority() {
		v, err := lm.Merge(right)
		return v, true, err
	}

	v, err := rm.RightMerge(left)
	return v, true, err
}

func Merge(values ...Value) (result Value, err error) {
	if undef := IsUndefined(values...); undef != nil {
		return undef, nil
	}

	for _, item := range values {
		if item == nil {
			continue
		}
		if result == nil {
			result = item
		} else if v, ok, err := rightMerge(result, item); err != nil {
			return nil, err
		} else if ok {
			return v, nil
		} else if m, ok := result.(Merger); ok {
			result, err = m.Merge(item)
			if err != nil {
				return nil, err
			}
		} else {
			result, err = mergeNative(result, item)
			if err != nil {
				return nil, fmt.Errorf("can not merge values: %w", err)
			}
		}
	}

	return result, nil
}

type TargetCompatibler interface {
	TargetCompatible(target Value) bool
}

func TargetCompatible(schema Schema, target Value) bool {
	if schema.TargetKind() == target.Kind() {
		return true
	}
	if tg, ok := schema.(TargetCompatibler); ok {
		return tg.TargetCompatible(target)
	}
	return false
}

type Compatibler interface {
	Compatible(kind Kind) bool
}

func Compatible(val Value, kind Kind) bool {
	if val.Kind() == kind {
		return true
	}
	if c, ok := val.(Compatibler); ok {
		return c.Compatible(kind)
	}
	return false
}

func assertType(val Value, kind Kind) error {
	if val == nil {
		return fmt.Errorf("expected kind %s, got nil", kind)
	}
	if !Compatible(val, kind) {
		return fmt.Errorf("expected kind %s, got incompatible %s", kind, val.Kind())
	}
	return nil
}

func AssertKindsMatch(left, right Value) error {
	if left.Kind() != right.Kind() {
		return fmt.Errorf("can not override field %s [%s] with %s [%s]",
			left.Kind(), left, right.Kind(), right)
	}
	return nil
}
