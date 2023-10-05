package value

import "fmt"

func mergeNative(left, right Value) (Value, error) {
	if err := assertKindsMatch(left, right); err != nil {
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

type Merger interface {
	Merge(val Value) (Value, error)
}

func Merge(values ...Value) (result Value, err error) {
	for _, item := range values {
		if item.Kind() == UndefinedKind {
			return item, nil
		}
		if result == nil {
			result = item
		} else {
			if m, ok := result.(Merger); ok {
				result, err = m.Merge(item)
				if err != nil {
					return nil, err
				}
			} else {
				result, err = mergeNative(result, item)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return result, nil
}

func assertType(val Value, kind Kind) error {
	if val == nil {
		return fmt.Errorf("expected kind %s, got nil", kind)
	}
	if val.Kind() != kind {
		return fmt.Errorf("expected kind %s, got %s (value: %s)", kind, val.Kind(), val)
	}
	return nil
}

func assertKindsMatch(left, right Value) error {
	if left.Kind() != right.Kind() {
		return fmt.Errorf("can not override field %s [%s] with %s [%s]",
			left.Kind(), left, right.Kind(), right)
	}
	return nil
}
