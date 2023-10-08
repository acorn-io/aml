package value

import (
	"fmt"
	"slices"
	"strings"

	"github.com/acorn-io/aml/pkg/schema"
)

type SchemaContext struct {
	seen  map[string]struct{}
	index int
}

func (s *SchemaContext) SetIndex(i int) {
	s.index = i
}

func (s *SchemaContext) GetIndex() int {
	return s.index
}

func (s *SchemaContext) haveSeen(path string) bool {
	_, ok := s.seen[path]
	return ok
}

func (s *SchemaContext) remoteSeen(path string) {
	delete(s.seen, path)
}

func (s *SchemaContext) addSeen(path string) {
	if s.seen == nil {
		s.seen = map[string]struct{}{}
	}
	s.seen[path] = struct{}{}
}

type TypeSchema struct {
	Position     Position
	KindValue    Kind
	Object       *ObjectSchema
	Array        *ArraySchema
	Constraints  []Checker
	Alternates   []TypeSchema
	DefaultValue Value
}

func (n *TypeSchema) isMergeableObject() bool {
	return n.KindValue == ObjectKind &&
		n.Object != nil &&
		n.Array == nil &&
		len(n.Constraints) == 0 &&
		len(n.Alternates) == 0 &&
		n.DefaultValue == nil
}

func NewMatchTypeWithDefault(pos Position, v Value) Value {
	return &TypeSchema{
		Position:     pos,
		KindValue:    TargetKind(v),
		DefaultValue: v,
	}
}

func NewDefault(pos Position, v Value) Value {
	return &TypeSchema{
		Position:     pos,
		KindValue:    TargetKind(v),
		DefaultValue: v,
		Constraints: []Checker{
			&Constraint{
				Op:    "==",
				Right: v,
			},
		},
	}
}

type Condition func(val Value) (Value, error)

func (n *TypeSchema) GetContract() (Contract, bool) {
	if n.Object != nil {
		return n.Object.GetContract()
	}
	return nil, false
}

func (n *TypeSchema) String() string {
	return fmt.Sprintf("(%s %s)", n.KindValue, SchemaKind)
}

func (n *TypeSchema) Kind() Kind {
	return SchemaKind
}

func (n *TypeSchema) TargetKind() Kind {
	return n.KindValue
}

func checkerToConstraint(ctx SchemaContext, checker Checker) (result schema.Constraint, _ bool, _ error) {
	right, ok, err := checker.RightNative()
	if err != nil {
		return result, false, err
	} else if !ok {
		right = nil
	}

	if ts, ok := right.(*TypeSchema); ok {
		ft, ok, err := typeSchemaToFieldType(ctx, ts)
		if err != nil || !ok {
			return result, ok, err
		}
		right = ft
	}

	return schema.Constraint{
		ID:          checker.ID(),
		Description: checker.Description(),
		Op:          checker.OpString(),
		Right:       right,
	}, true, nil
}

func typeSchemaToFieldType(ctx SchemaContext, n *TypeSchema) (result schema.FieldType, ok bool, err error) {
	result.Kind = schema.Kind(n.KindValue)

	if n.DefaultValue != nil {
		def, ok, err := NativeValue(n.DefaultValue)
		if err != nil || !ok {
			return result, ok, err
		}
		result.Default = def
	}

	for _, checker := range n.Constraints {
		constraint, ok, err := checkerToConstraint(ctx, checker)
		if err != nil {
			return result, false, err
		} else if !ok {
			continue
		}

		result.Contstraints = append(result.Contstraints, constraint)
	}

	for _, alt := range n.Alternates {
		altType, ok, err := typeSchemaToFieldType(ctx, &alt)
		if err != nil || !ok {
			return result, ok, err
		}
		result.Alternates = append(result.Alternates, altType)
	}

	if n.Object != nil {
		result.Object, ok, err = n.Object.DescribeObject(ctx)
		if err != nil || !ok {
			return result, ok, err
		}
	}

	if n.Array != nil {
		result.Array, ok, err = n.Array.DescribeArray(ctx)
		if err != nil || !ok {
			return result, ok, err
		}
	}

	return result, true, nil
}

func addObjectKeys(keySeen map[string]struct{}, obj Value) (keys []string, _ error) {
	objKeys, err := Keys(obj)
	if err != nil {
		return nil, err
	}
	for _, key := range objKeys {
		if _, seen := keySeen[key]; seen {
			continue
		}
		keySeen[key] = struct{}{}
		keys = append(keys, key)
	}
	return
}

func getTypeSchemaFromChecker(constraint Checker) (*TypeSchema, bool) {
	if c, ok := constraint.(*Constraint); ok {
		if ts, ok := c.Right.(*TypeSchema); ok {
			return ts, true
		}
	}
	return nil, false
}

func (n *TypeSchema) Keys() ([]string, error) {
	var (
		keys    []string
		keySeen = map[string]struct{}{}
	)

	if n.Object != nil {
		newKeys, err := addObjectKeys(keySeen, n.Object)
		if err != nil {
			return nil, err
		}
		keys = append(keys, newKeys...)
	} else if n.KindValue == ObjectKind {
	} else {
		return nil, fmt.Errorf("schema for kind %s does not support keys call", n.KindValue)
	}

	for _, checker := range n.Constraints {
		if ts, ok := getTypeSchemaFromChecker(checker); ok {
			newKeys, err := addObjectKeys(keySeen, ts)
			if err != nil {
				return nil, err
			}
			keys = append(keys, newKeys...)
		}
	}

	return keys, nil
}

func (n *TypeSchema) LookupValue(key Value) (Value, bool, error) {
	var values []Value
	if n.Object != nil {
		v, ok, err := Lookup(n.Object, key)
		if err != nil {
			return nil, false, err
		}
		if ok {
			values = append(values, v)
		}
	}
	for _, checker := range n.Constraints {
		if ts, ok := getTypeSchemaFromChecker(checker); ok {
			v, ok, err := ts.LookupValue(key)
			if err != nil {
				return nil, false, err
			}
			if ok {
				values = append(values, v)
			}
		}
	}

	if len(values) > 0 {
		v, err := Merge(values...)
		return v, true, err
	}

	for _, alt := range n.Alternates {
		v, ok, err := alt.LookupValue(key)
		if err != nil {
			return nil, false, err
		}
		if ok {
			return v, true, nil
		}
	}

	return nil, false, nil
}

func (n *TypeSchema) DescribeObject(ctx SchemaContext) (result *schema.Object, ok bool, err error) {
	if n.Object != nil {
		result, ok, err = n.Object.DescribeObject(ctx)
		if err != nil || !ok {
			return nil, ok, err
		}
	} else if n.KindValue == ObjectKind {
		result = &schema.Object{
			AllowNewKeys: true,
		}
	} else {
		return nil, false, nil
	}
	for _, checker := range n.Constraints {
		if ts, ok := getTypeSchemaFromChecker(checker); ok {
			obj, ok, err := ts.DescribeObject(ctx)
			if err != nil {
				return nil, false, err
			} else if !ok {
				continue
			}
			result, err = result.Merge(obj)
			if err != nil {
				return nil, false, err
			}
		}
	}
	return result, true, nil
}

func (n *TypeSchema) DescribeFieldType(ctx SchemaContext) (result schema.FieldType, _ error) {
	fieldType, ok, err := typeSchemaToFieldType(ctx, n)
	if err != nil {
		return result, err
	} else if !ok {
		return result, fmt.Errorf("failed to yield value to determin field type on")
	}
	return fieldType, nil
}

func (n *TypeSchema) Eq(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, &Constraint{
		Op:    "==",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Neq(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, &Constraint{
		Op:    "!=",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Gt(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, &Constraint{
		Op:    ">",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Ge(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, &Constraint{
		Op:    ">=",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Le(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, &Constraint{
		Op:    "<=",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Lt(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, &Constraint{
		Op:    "<",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Mat(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, &Constraint{
		Op:    "=~",
		Right: right,
	})
	return &result, nil
}

func (n *TypeSchema) Nmat(right Value) (Value, error) {
	result := *n
	result.Constraints = append(result.Constraints, &Constraint{
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

func checkNoMultipleDefault(left, right *TypeSchema) error {
	_, ok, err := left.getDefault(false)
	if err != nil {
		return err
	}
	if ok {
		_, ok, err = right.getDefault(false)
		if err != nil {
			return err
		}
		if ok {
			return &posError{
				Position: right.Position,
				Err:      fmt.Errorf("multiple defaults can not be defined"),
			}
		}
	}
	return nil
}

func (n *TypeSchema) And(right Value) (Value, error) {
	if n.TargetKind() == SchemaKind && right.Kind() == SchemaKind {
		return right, nil
	}

	rightSchema, ok := right.(*TypeSchema)
	if !ok {
		return nil, fmt.Errorf("expected kind %s, got %s", n.Kind(), right.Kind())
	}
	if n.TargetKind() != rightSchema.TargetKind() {
		return nil, fmt.Errorf("invalid schema condition %s && %s incompatible", n.TargetKind(), rightSchema.TargetKind())
	}

	if err := checkNoMultipleDefault(n, rightSchema); err != nil {
		return nil, err
	}

	if n.isMergeableObject() && rightSchema.isMergeableObject() {
		return &TypeSchema{
			Position:  rightSchema.Position,
			KindValue: ObjectKind,
			Object:    n.Object.MergeContract(rightSchema.Object),
		}, nil
	}

	return &TypeSchema{
		Position:  rightSchema.Position,
		KindValue: n.KindValue,
		Constraints: []Checker{
			&Constraint{
				Op:    "type",
				Right: n,
			},
			&Constraint{
				Op:    "type",
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
	rightSchema, ok := right.(*TypeSchema)
	if !ok {
		rightSchema = NewDefault(n.Position, right).(*TypeSchema)
	}
	if err := checkNoMultipleDefault(n, rightSchema); err != nil {
		return nil, err
	}
	return &TypeSchema{
		Position:    rightSchema.Position,
		KindValue:   typeOrUnion(n.KindValue, rightSchema.KindValue),
		Constraints: MustMatchAlternate(),
		Alternates: []TypeSchema{
			*n, *rightSchema,
		},
	}, nil
}

func (n *TypeSchema) renderDefaultObject() (_ Value, _ bool, retErr error) {
	v, err := Merge(n, NewObject(nil))
	return v, true, err
}

func (n *TypeSchema) renderDefaultArray() (_ Value, _ bool, retErr error) {
	v, err := Merge(n, NewArray(nil))
	return v, true, err
}

func (n *TypeSchema) Default() (Value, bool, error) {
	v, ok, err := n.getDefault(false)
	if err != nil || ok {
		return v, ok, err
	}
	return n.getDefault(true)
}

func (n *TypeSchema) getDefault(renderImplicit bool) (Value, bool, error) {
	if n.DefaultValue != nil {
		return n.DefaultValue, true, nil
	}

	for _, checker := range n.Constraints {
		if ts, ok := getTypeSchemaFromChecker(checker); ok {
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
		v, ok, err := alt.getDefault(renderImplicit)
		if err != nil {
			return nil, false, err
		}
		if ok {
			return v, true, nil
		}
	}

	if renderImplicit {
		if n.Object != nil {
			return n.renderDefaultObject()
		}
		if n.Array != nil {
			return n.renderDefaultArray()
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

func checkType(schema *TypeSchema, right Value) (Value, error) {
	var errs []error

	if schema.TargetKind() == right.Kind() || schema.TargetKind() == UnionKind {
		if schema.Object != nil {
			v, err := schema.Object.Merge(right)
			if err == nil {
				right = v
			} else {
				errs = append(errs, err)
			}
		}

		if schema.Array != nil {
			v, err := schema.Array.Merge(right)
			if err == nil {
				right = v
			} else {
				errs = append(errs, err)
			}
		}

		if err := Constraints(schema.Constraints).Check(right); err != nil {
			errs = append(errs, err)
		}

		if schema.DefaultValue != nil && !IsSimpleKind(right.Kind()) {
			v, err := Merge(schema.DefaultValue, right)
			if err == nil {
				right = v
			} else {
				errs = append(errs, err)
			}
		}
	} else {
		errs = append(errs, fmt.Errorf("expected kind %s but got %s with value (%v)", schema.TargetKind(), right.Kind(), right))
	}

	if schema.TargetKind() == UnionKind {
		var kinds []Kind
		for _, alt := range schema.Alternates {
			kinds = append(kinds, alt.KindValue)
		}
		if len(kinds) > 0 {
			errs = append(errs, fmt.Errorf("failed to resolve union type to kind %v from kind %s value (%v)", kinds, right.Kind(), right))
		}
	}

	if len(errs) == 0 {
		return right, nil
	}

	retErr := &ErrUnmatchedType{
		Position: schema.Position,
		Errs:     errs,
	}

	for _, alt := range schema.Alternates {
		ret, newErrs := checkType(&alt, right)
		if newErrs == nil {
			return ret, nil
		}
		retErr.Alternates = append(retErr.Alternates, newErrs)
	}

	return nil, retErr
}

func (n *TypeSchema) Merge(right Value) (Value, error) {
	if right.Kind() == SchemaKind {
		return And(n, right)
	}
	return checkType(n, right)
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
