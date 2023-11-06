package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/acorn-io/aml/pkg/errors"
	"github.com/acorn-io/aml/pkg/value"
)

type Parens struct {
	Comments Comments
	Expr     Expression
}

func (p *Parens) ToValue(ctx context.Context) (value.Value, bool, error) {
	return p.Expr.ToValue(ctx)
}

type Default struct {
	Comments Comments
	Expr     Expression
	Pos      value.Position
}

type defaulted struct {
	value.Deferred

	defaultValue value.Value
}

func (d defaulted) RightMergePriority() value.RightMergePriority {
	return value.DefaultedPriority
}

func (d defaulted) RightMerge(val value.Value) (value.Value, error) {
	return d.Merge(val)
}

func (d defaulted) Merge(val value.Value) (value.Value, error) {
	if err := value.AssertKindsMatch(d.defaultValue, val); err != nil {
		return nil, err
	}
	return val, nil
}

func (d *Default) ToValue(ctx context.Context) (value.Value, bool, error) {
	v, ok, err := d.Expr.ToValue(WithSchema(ctx, false))
	if err != nil || !ok {
		return nil, ok, err
	}
	return defaulted{
		Deferred: value.Deferred{
			Resolve: func() (value.Value, error) {
				return v, nil
			},
			KindResolver: func() value.Kind {
				return v.Kind()
			},
		},
		defaultValue: v,
	}, true, nil
}

type Op struct {
	Unary    bool
	Comments Comments
	Operator value.Operator
	Left     Expression
	Right    Expression
	Pos      value.Position
}

func (o *Op) ToValue(ctx context.Context) (value.Value, bool, error) {
	left, ok, err := o.Left.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}

	if o.Unary {
		newValue, err := value.UnaryOperation(o.Operator, left)
		return newValue, true, err
	}

	right, ok, err := o.Right.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}

	newValue, err := value.BinaryOperation(o.Operator, left, right)
	if err != nil {
		return nil, false, value.NewErrPosition(o.Pos, err)
	}
	return newValue, true, nil
}

type Lookup struct {
	Comments Comments
	Pos      value.Position
	Key      string
}

func (l *Lookup) ToValue(ctx context.Context) (value.Value, bool, error) {
	scope := GetScope(ctx)

	v, ok, err := scope.Get(ctx, l.Key)
	if nf := (*errors.ErrValueNotDefined)(nil); errors.As(err, &nf) {
		return value.Undefined{
			Err: newNotFound(l.Pos, l.Key, nil),
			Pos: l.Pos,
		}, true, nil
	}
	if err != nil {
		return nil, false, newNotFound(l.Pos, l.Key, err)
	}
	if !ok {
		return nil, false, newNotFound(l.Pos, l.Key, nil)
	}
	return v, true, nil
}

type ErrKeyNotFound struct {
	Key     string
	Message string
	Err     error
}

func (e *ErrKeyNotFound) Error() string {
	return e.Message
}

func (e *ErrKeyNotFound) Unwrap() error {
	return e.Err
}

func newNotFound(pos value.Position, key any, err error) error {
	if err != nil {
		e := fmt.Errorf("key not found \"%s\": %w", key, err)
		return value.NewErrPosition(pos,
			&ErrKeyNotFound{
				Key:     fmt.Sprint(key),
				Err:     e,
				Message: e.Error(),
			})
	}
	e := fmt.Errorf("key not found \"%s\"", key)
	return value.NewErrPosition(pos,
		&ErrKeyNotFound{
			Key:     fmt.Sprint(key),
			Err:     e,
			Message: e.Error(),
		})
}

type Selector struct {
	Comments Comments
	Pos      value.Position
	Base     Expression
	Key      Expression
}

func (s *Selector) ToValue(ctx context.Context) (_ value.Value, _ bool, retErr error) {
	defer func() {
		retErr = value.NewErrPosition(s.Pos, retErr)
	}()

	key, ok, err := s.Key.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}

	v, ok, err := s.Base.ToValue(ctx)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	newValue, ok, err := value.Lookup(v, key)
	if nf := (*errors.ErrValueNotDefined)(nil); errors.As(err, &nf) {
		return value.Undefined{
			Err: newNotFound(s.Pos, key, nil),
			Pos: s.Pos,
		}, true, nil
	} else if err != nil {
		return nil, false, newNotFound(s.Pos, key, err)
	}
	if !ok {
		return &value.Undefined{
			Err: newNotFound(s.Pos, key, nil),
			Pos: s.Pos,
		}, true, nil
	}

	return newValue, true, nil
}

type Index struct {
	Comments Comments
	Pos      value.Position
	Base     Expression
	Index    Expression
}

func (i *Index) ToValue(ctx context.Context) (value.Value, bool, error) {
	base, ok, err := i.Base.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}

	indexValue, ok, err := i.Index.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}

	if indexValue.Kind() == value.StringKind {
		v, ok, err := value.Lookup(base, indexValue)
		if err != nil {
			return nil, false, err
		} else if !ok {
			return nil, false, newNotFound(i.Pos, indexValue, nil)
		}
		return v, ok, err
	}

	result, ok, err := value.Index(base, indexValue)
	if err != nil {
		return nil, false, value.NewErrPosition(i.Pos, err)
	}
	return result, ok, nil
}

type Slice struct {
	Comments Comments
	Pos      value.Position
	Base     Expression
	Start    Expression
	End      Expression
}

func (s *Slice) ToValue(ctx context.Context) (value.Value, bool, error) {
	var (
		start, end value.Value
	)

	v, ok, err := s.Base.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}

	if s.Start != nil {
		start, ok, err = s.Start.ToValue(ctx)
		if err != nil || !ok {
			return nil, ok, err
		}
	}

	if s.End != nil {
		end, ok, err = s.End.ToValue(ctx)
		if err != nil || !ok {
			return nil, ok, err
		}
	}

	newValue, ok, err := value.Slice(v, start, end)
	if err != nil || !ok {
		return nil, ok, err
	}

	return newValue, true, nil
}

type Call struct {
	Comments Comments
	Pos      value.Position
	Func     Expression
	Args     []Field
}

func (c *Call) ToValue(ctx context.Context) (value.Value, bool, error) {
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
	}

	v, ok, err := c.Func.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}

	// Disable schema evaluation
	ctx = WithSchema(ctx, false)

	var args []value.CallArgument
	for _, field := range c.Args {
		var arg value.CallArgument
		if posArg, ok := field.(IsPositionalArgument); ok {
			arg.Positional = posArg.IsPositionalArgument()
		}
		v, ok, err := field.ToValueForIndex(ctx, 0)
		if err != nil {
			return nil, false, err
		} else if !ok {
			continue
		}
		arg.Value = v
		args = append(args, arg)
	}

	v, ok, err = value.Call(ctx, v, args...)
	if err != nil {
		return v, ok, value.NewErrPosition(c.Pos, err)
	}
	return v, ok, nil
}

type If struct {
	Pos       value.Position
	Comments  Comments
	Condition Expression
	Value     Expression
	Else      Expression
}

func (i *If) ToValue(ctx context.Context) (ret value.Value, ok bool, err error) {
	v, ok, err := i.Condition.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}

	if v.Kind() == value.UndefinedKind {
		return v, true, nil
	}

	b, err := value.ToBool(v)
	if err != nil {
		return nil, false, value.NewErrPosition(i.Pos, err)
	}
	if !b {
		if i.Else != nil {
			return i.Else.ToValue(ctx)
		}
		return nil, false, nil
	}

	return i.Value.ToValue(ctx)
}

type Interpolation struct {
	Parts []any
}

func (i *Interpolation) ToValue(ctx context.Context) (value.Value, bool, error) {
	var result []string
	for _, part := range i.Parts {
		switch v := part.(type) {
		case string:
			result = append(result, v)
		case Expression:
			val, ok, err := v.ToValue(ctx)
			if err != nil {
				return nil, false, err
			}
			if !ok {
				continue
			}
			if val.Kind() == value.UndefinedKind {
				return val, true, nil
			}

			// This might be a schema val which don't over NativeValues, but it might have a default which
			// does, so resolve to default
			defVal, ok, err := value.DefaultValue(val)
			if err != nil {
				return nil, false, err
			} else if ok {
				val = defVal
			}

			nv, ok, err := value.NativeValue(val)
			if err != nil || !ok {
				return nil, ok, err
			}
			result = append(result, value.Escape(fmt.Sprint(nv)))
		}
	}
	s, err := value.Unquote(strings.Join(result, ""))
	return value.NewValue(s), true, err
}

type For struct {
	Comments   Comments
	Key        string
	Value      string
	Collection Expression
	Body       Expression
	Else       Expression
	Merge      bool
	Position   value.Position
}

type entry struct {
	Key   value.Value
	Value value.Value
}

func toList(v value.Value) (result []entry, _ error) {
	if v.Kind() == value.ArrayKind {
		list, err := value.ToValueArray(v)
		if err != nil {
			return nil, err
		}
		for i, item := range list {
			result = append(result, entry{
				Key:   value.NewValue(i),
				Value: item,
			})
		}
		return
	} else if v.Kind() == value.ObjectKind {
		keys, err := value.Keys(v)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			v, ok, err := value.Lookup(v, value.NewValue(key))
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			result = append(result, entry{
				Key:   value.NewValue(key),
				Value: v,
			})
		}
	} else {
		result = append(result, entry{
			Key:   value.NewValue(0),
			Value: v,
		})
	}

	return
}

func (f *For) ToValue(ctx context.Context) (value.Value, bool, error) {
	collection, ok, err := f.Collection.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}

	if undef := value.IsUndefined(collection); undef != nil {
		return undef, true, nil
	}

	list, err := toList(collection)
	if err != nil {
		return nil, false, err
	}

	var (
		array     = value.Array{}
		prev      value.Value
		elseValue value.Value
	)

	if f.Else != nil {
		newValue, ok, err := f.Else.ToValue(ctx)
		if err != nil {
			return nil, ok, err
		} else if ok {
			elseValue = newValue
			prev = elseValue
		}
	}

	for i, item := range list {
		select {
		case <-ctx.Done():
			return nil, false, value.NewErrPosition(f.Position,
				fmt.Errorf("aborting loop: %w", ctx.Err()))
		default:
		}

		data := map[string]any{}
		if f.Key != "" {
			data[f.Key] = item.Key
		}
		if f.Value != "" {
			data[f.Value] = item.Value
		}
		if prev == nil {
			data["prev"] = value.NewObject(nil)
		} else {
			data["prev"] = prev
		}

		ctx := value.WithIndexPath(ctx, i)
		_, ctx = GetScope(ctx).NewScope(ctx, ScopeData(data))

		newValue, ok, err := f.Body.ToValue(ctx)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			continue
		}

		var (
			shouldSkip  bool
			shouldBreak bool
		)

		if lc, ok := newValue.(*LoopControl); ok {
			shouldSkip = lc.Skip
			if lc.Break {
				newValue = lc.Value
				shouldBreak = true
			}
		}

		if !shouldSkip {
			prev, err = appendValue(prev, newValue)
			if err != nil {
				return nil, false, err
			}
			array = append(array, newValue)
		}

		if shouldBreak {
			break
		}
	}

	if len(array) == 0 && elseValue != nil {
		prev = elseValue
		array = append(array, elseValue)
	}

	if f.Merge {
		return prev, prev != nil, nil
	}

	return array, true, nil
}

func appendValue(left, right value.Value) (value.Value, error) {
	if undef := value.IsUndefined(left, right); undef != nil {
		return undef, nil
	}

	if left == nil {
		return right, nil
	} else if right == nil {
		return left, nil
	}

	if left.Kind() != value.ObjectKind || right.Kind() != value.ObjectKind {
		return right, nil
	}

	merged := map[string]any{}

	entries, err := value.Entries(left)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		merged[entry.Key] = entry.Value
	}

	entries, err = value.Entries(right)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		merged[entry.Key] = entry.Value
	}

	return value.NewValue(merged), nil
}

type Expression interface {
	ToValue(ctx context.Context) (value.Value, bool, error)
}

type IsPositionalArgument interface {
	IsPositionalArgument() bool
}

type Value struct {
	Value value.Value
}

func (v Value) ToValue(_ context.Context) (value.Value, bool, error) {
	return v.Value, true, nil
}
