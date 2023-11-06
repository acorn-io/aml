package value

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type ArraySchema struct {
	Positions   []Position `json:"-"`
	Description string     `json:"description"`
	Valid       []Schema   `json:"valid"`
}

func (n *ArraySchema) ImpliedDefault() (Value, bool, error) {
	return NewArray(nil), true, nil
}

func (a *ArraySchema) Validate(ctx context.Context, right Value) (Value, error) {
	if err := assertType(right, ArrayKind); err != nil {
		return nil, err
	}

	var resultValues []Value

	values, err := ToValueArray(right)
	if err != nil {
		return nil, err
	}

valueLoop:
	for i, value := range values {
		var (
			errs []error
			ctx  = WithDataIndexPath(ctx, i)
		)
		for _, validater := range a.Valid {
			if valid, err := validater.Validate(ctx, value); err == nil {
				resultValues = append(resultValues, valid)
				continue valueLoop
			} else {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			return nil, errors.Join(errs...)
		}
		if len(a.Valid) == 0 {
			resultValues = append(resultValues, value)
		}
	}
	return NewValue(resultValues), nil
}

func (a *ArraySchema) Merge(right *ArraySchema) (*ArraySchema, error) {
	if a == nil {
		return right, nil
	} else if right == nil {
		return a, nil
	}

	if len(a.Valid) != len(right.Valid) {
		return nil, NewErrPosition(lastPos(a.Positions, right.Positions),
			fmt.Errorf("can not merge two array schemas with different lengths [%d %s] and [%d %s]",
				len(a.Valid), lastPos(a.Positions, nil),
				len(right.Valid), lastPos(right.Positions, nil)))
	}

	result := &ArraySchema{
		Positions:   mergePositions(a.Positions, right.Positions),
		Description: mergeDescription(a.Description, right.Description),
	}

	for i, leftType := range a.Valid {
		merged, err := leftType.MergeType(right.Valid[i])
		if err != nil {
			return nil, err
		}
		result.Valid = append(result.Valid, merged)
	}

	return result, nil
}

func mergePositions(left, right []Position) []Position {
	result := left
	for _, right := range right {
		found := false
		for _, left := range left {
			if left == right {
				found = true
				break
			}
		}
		if !found {
			result = append(result, right)
		}
	}
	return result
}

// mergeDescription is a dumb attempt at making merging descriptions idempotent
func mergeDescription(left, right string) string {
	if len(left) == 0 {
		return right
	} else if len(right) == 0 {
		return left
	}
	if strings.Contains(left, right) {
		return left
	}
	if strings.Contains(right, left) {
		return right
	}
	return strings.Join([]string{left, right}, "\n")
}

func lastPos(left, right []Position) Position {
	if len(right) != 0 {
		return right[len(right)-1]
	} else if len(left) != 0 {
		return left[len(left)-1]
	}
	return NoPosition
}
