package value

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type TypeSchema struct {
	Positions    []Position    `json:"-"`
	KindValue    Kind          `json:"kindValue"`
	Object       *ObjectSchema `json:"object"`
	Array        *ArraySchema  `json:"array"`
	FuncSchema   *FuncSchema   `json:"func"`
	Constraints  []Constraint  `json:"constraints"`
	Alternates   []Schema      `json:"alternates"`
	DefaultValue Value         `json:"defaultValue"`
	Path         Path          `json:"path"`
	Reference    bool          `json:"reference"`

	rendering bool
}

func (n *TypeSchema) GetPositions() []Position {
	return n.Positions
}

func (n *TypeSchema) ValidArrayItems() (result []Schema) {
	if n.Array == nil {
		panic("Array is nil")
	}

	for _, valid := range n.Array.Valid {
		result = append(result, valid)
	}

	return result
}

func (n *TypeSchema) GetPath() Path {
	return n.Path
}

func (n *TypeSchema) Call(ctx context.Context, args []CallArgument) (Value, bool, error) {
	if n.KindValue == FuncKind && n.FuncSchema != nil && n.DefaultValue != nil {
		return Call(ctx, n.DefaultValue, args...)
	}

	for _, arg := range args {
		if arg.Self || !arg.Positional {
			continue
		}
		v, err := n.Validate(ctx, arg.Value)
		return v, true, err
	}
	return nil, false, nil
}

func (n *TypeSchema) Keys() (result []string, _ error) {
	if n.Object == nil {
		return nil, nil
	}
	for _, field := range n.Object.Fields {
		if field.Match {
			continue
		}
		result = append(result, field.Key)
	}
	return result, nil
}

func (n *TypeSchema) LookupValue(key Value) (Value, bool, error) {
	if n.Object == nil {
		return nil, false, nil
	}

	s, err := ToString(key)
	if err != nil {
		return nil, false, err
	}

	for _, field := range n.Object.Fields {
		if !field.Match && field.Key == s {
			return field.Schema, true, nil
		}
	}
	return nil, false, nil
}

func NewMatchTypeWithDefault(pos Position, v Value) Value {
	return &TypeSchema{
		Positions:    []Position{pos},
		KindValue:    TargetKind(v),
		DefaultValue: v,
	}
}

func NewDefault(pos Position, v Value) Value {
	return &TypeSchema{
		Positions:    []Position{pos},
		KindValue:    TargetKind(v),
		DefaultValue: v,
		Constraints: []Constraint{
			{
				Op:    "==",
				Right: v,
			},
		},
	}
}

type Condition func(val Value) (Value, error)

func (n *TypeSchema) String() string {
	pathString := n.Path.String()
	if pathString != "" {
		return fmt.Sprintf("(%s %s %s)", n.KindValue, SchemaKind, pathString)
	}
	return fmt.Sprintf("(%s %s)", n.KindValue, SchemaKind)
}

func (n *TypeSchema) Kind() Kind {
	return SchemaKind
}

func (n *TypeSchema) TargetKind() Kind {
	return n.KindValue
}

func (n *TypeSchema) Eq(right Value) (Value, error) {
	if right.Kind() == SchemaKind {
		_, err := n.Merge(right)
		return NewValue(err == nil), nil
	}
	result := *n
	result.Constraints = append(result.Constraints, Constraint{
		Op:    "==",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Neq(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, Constraint{
		Op:    "!=",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Gt(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, Constraint{
		Op:    ">",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Ge(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, Constraint{
		Op:    ">=",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Le(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, Constraint{
		Op:    "<=",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Lt(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, Constraint{
		Op:    "<",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Mat(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, Constraint{
		Op:    "=~",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Nmat(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, Constraint{
		Op:    "!~",
		Right: right,
	})
	return &result, nil
}

func TargetKind(v Value) Kind {
	if tk, ok := v.(interface {
		TargetKind() Kind
	}); ok {
		return tk.TargetKind()
	}
	return v.Kind()
}

type posError struct {
	Position Position
	Err      error
}

func (e *posError) Unwrap() error {
	return e.Err
}

func (e *posError) Error() string {
	return e.Err.Error()
}

func (e *posError) Pos() Position {
	return e.Position
}

func checkNoMultipleDefault(left, right Schema) error {
	leftDef, ok, err := left.DefaultWithImplicit(false)
	if err != nil {
		return err
	}
	if ok {
		rightDef, ok, err := right.DefaultWithImplicit(false)
		if err != nil {
			return err
		}
		if ok {
			if undef := IsUndefined(leftDef, rightDef); undef != nil {
				return nil
			}
			return &posError{
				Position: lastPos(left.GetPositions(), right.GetPositions()),
				Err:      fmt.Errorf("multiple defaults can not be defined (%s %s)", lastPos(left.GetPositions(), nil), lastPos(right.GetPositions(), nil)),
			}
		}
	}
	return nil
}

func (n *TypeSchema) And(right Value) (Value, error) {
	rightSchema, ok := right.(*TypeSchema)
	if !ok {
		return nil, NewErrPosition(lastPos(n.Positions, nil),
			fmt.Errorf("expected kind %s, got %s", n.Kind(), right.Kind()))
	}
	if n.TargetKind() != UnionKind && rightSchema.TargetKind() != UnionKind && n.TargetKind() != rightSchema.TargetKind() {
		return nil, NewErrPosition(lastPos(n.Positions, rightSchema.Positions),
			fmt.Errorf("invalid schema condition (%s %s) && (%s %s) incompatible",
				n.TargetKind(), lastPos(n.Positions, nil),
				rightSchema.TargetKind(), lastPos(rightSchema.Positions, nil)))
	}

	if err := checkNoMultipleDefault(n, rightSchema); err != nil {
		return nil, err
	}

	return &TypeSchema{
		Positions: mergePositions(n.Positions, rightSchema.Positions),
		KindValue: n.KindValue,
		Constraints: []Constraint{
			{
				Op:    MustMatchSchema,
				Right: n,
			},
			{
				Op:    MustMatchSchema,
				Right: rightSchema,
			},
		},
	}, nil
}

func typeOrUnion(left, right Kind) Kind {
	if left == right {
		return left
	}
	return UnionKind
}

func (n *TypeSchema) Or(right Value) (Value, error) {
	return SchemaOr(n, right)
}

func SchemaOr(left Schema, right Value) (Value, error) {
	rightSchema, ok := right.(Schema)
	if !ok {
		rightSchema = NewDefault(lastPos(left.GetPositions(), nil), right).(Schema)
	}
	if err := checkNoMultipleDefault(left, rightSchema); err != nil {
		return nil, err
	}
	return &TypeSchema{
		Positions:   mergePositions(left.GetPositions(), rightSchema.GetPositions()),
		KindValue:   typeOrUnion(left.TargetKind(), rightSchema.TargetKind()),
		Constraints: MustMatchAlternate(),
		Alternates: []Schema{
			left, rightSchema,
		},
	}, nil
}

func (n *TypeSchema) Default() (Value, bool, error) {
	return n.DefaultWithImplicit(true)
}

func (n *TypeSchema) DefaultWithImplicit(renderImplicit bool) (Value, bool, error) {
	v, ok, err := n.getDefault(false)
	if err != nil || ok {
		return v, ok, err
	}
	if renderImplicit {
		return n.getDefault(true)
	}
	return nil, false, nil
}

func (n *TypeSchema) getDefault(renderImplicit bool) (Value, bool, error) {
	if n.DefaultValue != nil {
		return n.DefaultValue, true, nil
	}

	for _, checker := range n.Constraints {
		if ts, ok := checker.Right.(*TypeSchema); ok && checker.Op == MustMatchSchema {
			v, ok, err := ts.getDefault(renderImplicit)
			if err != nil {
				return nil, false, err
			}
			if ok {
				return v, true, nil
			}
		}
	}

	for _, alt := range n.Alternates {
		v, ok, err := alt.DefaultWithImplicit(renderImplicit)
		if err != nil {
			return nil, false, err
		}
		if ok {
			return v, true, nil
		}
	}

	if renderImplicit {
		if n.rendering {
			return nil, false, fmt.Errorf("invalid circular schema, can not render default (%s)", lastPos(n.Positions, nil))
		}
		n.rendering = true
		defer func() {
			n.rendering = false
		}()
		if n.Object != nil {
			return n.Object.ImpliedDefault()
		} else if n.Array != nil {
			return n.Array.ImpliedDefault()
		} else if n.TargetKind() == ObjectKind {
			return NewObject(nil), true, nil
		} else if n.TargetKind() == ArrayKind {
			return NewArray(nil), true, nil
		}
	}

	return nil, false, nil
}

type ErrUnmatchedType struct {
	Position   Position
	Errs       []error
	Alternates []error
}

func (e *ErrUnmatchedType) Pos() Position {
	return e.Position
}

func (e *ErrUnmatchedType) Unwrap() []error {
	return append(e.Errs, e.Alternates...)
}

func (e *ErrUnmatchedType) errors() (result []string) {
	result = append(result, e.checkErr())
	for _, altErr := range e.Alternates {
		result = append(result, altErr.Error())
	}
	return result
}

func (e *ErrUnmatchedType) checkErr() string {
	filtered := slices.DeleteFunc(e.Errs, func(err error) bool {
		return err == ErrMustMatchAlternate
	})
	switch len(filtered) {
	case 0:
		return ""
	case 1:
		return filtered[0].Error()
	}
	var result strings.Builder
	for _, err := range filtered {
		result.WriteString("\n    ")
		result.WriteString(err.Error())
	}
	return result.String()
}

func (e *ErrUnmatchedType) Error() string {
	var (
		errorStrings []string
		seen         = map[string]struct{}{}
	)

	for _, key := range e.errors() {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		if key == "" {
			continue
		}
		errorStrings = append(errorStrings, key)
	}

	if len(errorStrings) == 0 {
		return "unspecified error"
	} else if len(errorStrings) == 1 {
		return errorStrings[0]
	}

	buf := strings.Builder{}
	for i, errString := range errorStrings {
		if buf.Len() > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(fmt.Sprintf("\n\toption %d: [%s]", i+1, errString))
	}

	return buf.String()
}

func checkType(ctx context.Context, schema *TypeSchema, right Value) (Value, error) {
	var errs []error

	if TargetCompatible(schema, right) || schema.TargetKind() == UnionKind {
		if schema.Object != nil {
			v, err := schema.Object.Validate(ctx, right, schema.Path)
			if err == nil {
				right = v
			} else {
				errs = append(errs, err)
			}
		}

		if schema.Array != nil {
			v, err := schema.Array.Validate(ctx, right)
			if err == nil {
				right = v
			} else {
				errs = append(errs, err)
			}
		}

		if err := Constraints(schema.Constraints).Check(ctx, right); err != nil {
			errs = append(errs, err)
		}
	} else {
		suffix := ""
		if schema.TargetKind() == SchemaKind && right.Kind() == ObjectKind {
			suffix = " (Are you missing the \"define\" keyword?)"
		}
		errs = append(errs, fmt.Errorf("expected kind %s but got kind %s%s", schema.TargetKind(), right.Kind(), suffix))
	}

	if len(errs) == 0 && schema.TargetKind() != UnionKind {
		return right, nil
	} else if len(errs) == 0 && schema.TargetKind() == UnionKind && len(schema.Alternates) == 0 {
		return right, nil
	}

	retErr := &ErrUnmatchedType{
		Position: lastPos(schema.Positions, nil),
		Errs:     errs,
	}

	for _, alt := range schema.Alternates {
		ret, newErr := alt.Validate(ctx, right)
		if newErr == nil {
			return ret, nil
		}
		retErr.Alternates = append(retErr.Alternates, newErr)
	}

	return nil, retErr
}

func (n *TypeSchema) RightMergePriority() RightMergePriority {
	return TypeSchemaPriority
}

func (n *TypeSchema) RightMerge(right Value) (Value, error) {
	if ts, ok := right.(*TypeSchema); ok {
		return ts.MergeType(n)
	}
	return n.Merge(right)
}

func (n *TypeSchema) Merge(right Value) (Value, error) {
	if ts, ok := right.(*TypeSchema); ok {
		return n.MergeType(ts)
	}
	return nil, NewErrPosition(lastPos(n.Positions, nil),
		fmt.Errorf("can not merge kinds %s and %s", SchemaKind, right.Kind()))
}

func (n *TypeSchema) MergeType(rightSchema Schema) (Schema, error) {
	if n == nil {
		return rightSchema, nil
	} else if rightSchema == nil {
		return n, nil
	}

	right, ok := rightSchema.(*TypeSchema)
	if !ok {
		return nil, fmt.Errorf("Can not merge incompatible go structs %T and %T", n, rightSchema)
	}

	if n.KindValue != right.KindValue {
		return nil, NewErrPosition(lastPos(n.Positions, right.Positions),
			fmt.Errorf("can not merge two schema of different types [%s %s] and [%s %s]",
				n.KindValue, lastPos(n.Positions, nil),
				right.KindValue, lastPos(right.Positions, nil)))
	}

	obj, err := n.Object.Merge(right.Object)
	if err != nil {
		return nil, err
	}

	arr, err := n.Array.Merge(right.Array)
	if err != nil {
		return nil, err
	}

	def, err := Merge(n.DefaultValue, right.DefaultValue)
	if err != nil {
		return nil, err
	}

	con, err := mergeConstraints(n.Constraints, right.Constraints)
	if err != nil {
		return nil, err
	}

	alts, err := mergeAlternates(n.Alternates, right.Alternates)
	if err != nil {
		return nil, err
	}

	return &TypeSchema{
		Positions:    mergePositions(n.Positions, right.Positions),
		KindValue:    n.KindValue,
		Path:         mergePath(n.Path, right.Path),
		Object:       obj,
		Array:        arr,
		Constraints:  con,
		Alternates:   alts,
		DefaultValue: def,
	}, nil
}

func mergeAlternates(left, right []Schema) ([]Schema, error) {
	if len(left) != len(right) {
		return nil, fmt.Errorf("can not merge schemas with different alternates length %d != %d",
			len(left), len(right))
	}

	var result []Schema
	for i, left := range left {
		newValue, err := left.MergeType(right[i])
		if err != nil {
			return nil, err
		}
		result = append(result, newValue)
	}

	return result, nil
}

func mergeConstraints(left, right []Constraint) ([]Constraint, error) {
	if len(left) != len(right) {
		return nil, fmt.Errorf("can not merge schemas with different constraints length %d != %d",
			len(left), len(right))
	}

	var result []Constraint
	for i, left := range left {
		right := right[i]
		if left.Op != right.Op {
			return nil, fmt.Errorf("can not merge schemas with different constraints ops %s != %s",
				left.Op, right.Op)
		}

		if left.Right == nil && right.Right == nil {
			result = append(result, left)
			continue
		} else if left.Right == nil {
			return nil, fmt.Errorf("can not merge schemas with different constraints values %v != %v",
				left.Right, right.Right)
		} else if right.Right == nil {
			return nil, fmt.Errorf("can not merge schemas with different constraints values %v != %v",
				left.Right, right.Right)
		}
		if left.Op == MustMatchSchema {
			merged, err := Merge(left.Right, right.Right)
			if err != nil {
				return nil, err
			}
			result = append(result, Constraint{
				Op:    MustMatchSchema,
				Right: merged,
			})
		} else {
			if v, err := Eq(left.Right, right.Right); err != nil {
				return nil, err
			} else if b, err := ToBool(v); err != nil {
				return nil, err
			} else if !b {
				return nil, fmt.Errorf("can not merge schemas with different constraints values %v != %v",
					left.Right, right.Right)
			}
			result = append(result, left)
		}
	}

	return result, nil
}

func (n *TypeSchema) Validate(ctx context.Context, right Value) (Value, error) {
	return checkType(ctx, n, right)
}

type Defaulter interface {
	Default() (Value, bool, error)
}

func DefaultValue(v Value) (Value, bool, error) {
	if v == nil {
		return nil, false, nil
	}
	if v, ok := v.(Defaulter); ok {
		return v.Default()
	}
	if v.Kind() == SchemaKind {
		return nil, false, nil
	}
	return v, true, nil
}

func (n *TypeSchema) NativeValue() (any, bool, error) {
	jsonSchema, err := jsonSchemaConvert(n.Path.String(), Summarize(n))
	return jsonSchema, jsonSchema != nil, err
}
