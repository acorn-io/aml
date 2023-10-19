package eval

import (
	"context"
	"fmt"

	"github.com/acorn-io/aml/pkg/value"
)

const (
	evalLoopMax = 100
)

type Struct struct {
	Position value.Position
	Comments Comments
	Fields   []Field
}

func (s *Struct) ToValue(ctx context.Context) (value.Value, bool, error) {
	var (
		storage = newStructScopeStorage(s)
	)

	_, ctx = GetScope(ctx).NewScope(ctx, storage)

	for i := 0; i < evalLoopMax; i++ {
		ret, retry, err := s.evaluateFields(ctx, storage)
		if err != nil {
			return nil, false, err
		}
		if !retry {
			if ts, ok := ret.(*value.TypeSchema); ok {
				if ts.Object != nil {
					ts.Object.Description = s.Comments.Last()
				}
				ts.Path = value.GetPath(ctx)
			}
			return ret, true, nil
		}
	}

	return nil, false, value.NewErrPosition(s.Position, fmt.Errorf("evaluation loop count exceeded count %d", evalLoopMax))
}

// evaluateFields will return an array of length of Fields that contains the value for the field or nil if that
// field contributed no value
func (s *Struct) evaluateFields(ctx context.Context, storage *structScopeStorage) (_ value.Value, retry bool, _ error) {
	var (
		undefined   value.Value
		scopeValue  value.Value
		returnValue value.Value
		loopControl *LoopControl
	)

	for i, field := range s.Fields {
		v, ok, err := field.ToValueForIndex(ctx, i)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			continue
		}

		if lc, ok := v.(*LoopControl); ok {
			v = lc.Value
			loopControl = loopControl.combine(lc)
			if loopControl.Skip {
				break
			}
			if v == nil {
				continue
			}
		}

		if v.Kind() == value.UndefinedKind {
			undefined = value.IsUndefined(undefined, v)
			continue
		}

		if field.IsForLookup(ctx) {
			scopeValue, err = value.Merge(scopeValue, v)
			if err != nil {
				return nil, false, value.NewErrPosition(field.Position(), err)
			}
		}

		if field.IsForValue(ctx) {
			returnValue, err = value.Merge(returnValue, v)
			if err != nil {
				return nil, false, value.NewErrPosition(field.Position(), err)
			}
		}
	}

	if undefined != nil {
		returnValue = undefined
	}

	if returnValue == nil {
		if IsSchema(ctx) {
			returnValue = value.NewOpenObject()
		} else {
			returnValue = value.NewObject(nil)
		}
	}

	mustRetry, err := storage.Store(scopeValue)
	if err != nil {
		return nil, false, err
	}

	return loopControl.withValue(returnValue), mustRetry, nil
}
