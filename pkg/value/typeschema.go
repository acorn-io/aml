package value

import (
	"errors"
	"fmt"
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
	Alternate    *TypeSchema
	DefaultValue Value
}

func NewMatchTypeWithDefault(v Value) Value {
	return &TypeSchema{
		KindValue:    TargetKind(v),
		DefaultValue: v,
	}
}

func NewDefault(v Value) Value {
	return &TypeSchema{
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

func checkerToConstraint(checker Checker) (result schema.Constraint, _ bool, _ error) {
	right, ok, err := checker.RightNative()
	if err != nil {
		return result, false, err
	} else if !ok {
		right = nil
	}

	left, ok, err := checker.LeftNative()
	if err != nil {
		return result, ok, err
	} else if !ok {
		left = nil
	}

	return schema.Constraint{
		Description: checker.Description(),
		Op:          checker.OpString(),
		Left:        left,
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
		constraint, ok, err := checkerToConstraint(checker)
		if err != nil {
			return result, false, err
		} else if !ok {
			continue
		}

		result.Constraint = append(result.Constraint, constraint)
	}

	if n.Alternate != nil {
		alt, ok, err := typeSchemaToFieldType(ctx, n.Alternate)
		if err != nil || !ok {
			return result, ok, err
		}
		result.Alternate = &alt
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

func (n *TypeSchema) Keys() ([]string, error) {
	if n.Object != nil {
		return Keys(n.Object)
	} else if n.KindValue == ObjectKind {
		return nil, nil
	}
	return nil, fmt.Errorf("schema for kind %s does not support keys call", n.KindValue)
}

func (n *TypeSchema) LookupValue(key Value) (Value, bool, error) {
	if n.Object != nil {
		return Lookup(n.Object, key)
	}
	return nil, false, fmt.Errorf("schema for kind %s does not support lookup", n.KindValue)
}

func (n *TypeSchema) DescribeObject(ctx SchemaContext) (*schema.Object, bool, error) {
	if n.Object != nil {
		return n.Object.DescribeObject(ctx)
	} else if n.KindValue == ObjectKind {
		return &schema.Object{
			AllowNewKeys: true,
		}, true, nil
	}
	return nil, false, nil
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

	cp := *n
	cp.Alternate = mergeAlternate(&cp, rightSchema.Alternate)
	cp.Constraints = append(cp.Constraints, rightSchema.Constraints...)
	if cp.DefaultValue == nil {
		cp.DefaultValue = rightSchema.DefaultValue
	} else if rightSchema.DefaultValue != nil {
		eq, err := Eq(cp.DefaultValue, rightSchema.DefaultValue)
		if err != nil {
			return nil, err
		}
		b, err := ToBool(eq)
		if err != nil {
			return nil, err
		}
		if !b {
			return nil, fmt.Errorf("can not have two default values for schema kind %s, %s and %s", cp.TargetKind(), cp.DefaultValue, rightSchema.DefaultValue)
		}
	}

	if n.KindValue == ObjectKind {
		if n.Object == nil {
			cp.Object = rightSchema.Object
		} else if rightSchema.Object != nil {
			obj, err := Merge(n.Object, rightSchema.Object)
			if err != nil {
				return nil, err
			}
			cp.Object = obj.(*TypeSchema).Object
		}
	}

	if n.KindValue == ArrayKind {
		if n.Array == nil {
			cp.Array = rightSchema.Array
		} else if rightSchema.Array != nil {
			obj, err := Merge(n.Array, rightSchema.Array)
			if err != nil {
				return nil, err
			}
			cp.Array = obj.(*TypeSchema).Array
		}
	}

	return &cp, nil
}

func mergeAlternate(left, right *TypeSchema) *TypeSchema {
	cp := *left
	if cp.Alternate == nil {
		cp.Alternate = right
	} else {
		cp.Alternate = mergeAlternate(left.Alternate, right)
	}
	return &cp
}

func (n *TypeSchema) Or(right Value) (Value, error) {
	rightSchema, ok := right.(*TypeSchema)
	if !ok {
		rightSchema = NewDefault(right).(*TypeSchema)
	}

	return mergeAlternate(n, rightSchema), nil
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
	if n.DefaultValue != nil {
		return n.DefaultValue, true, nil
	}
	if n.Alternate != nil {
		return n.Alternate.Default()
	}
	if n.Object != nil {
		return n.renderDefaultObject()
	}
	if n.Array != nil {
		return n.renderDefaultArray()
	}
	return nil, false, nil
}

type ErrUnmatchedType struct {
	Position  Position
	Errs      []error
	Alternate *ErrUnmatchedType
}

func (e *ErrUnmatchedType) Unwrap() []error {
	if e.Alternate == nil {
		return e.Errs
	}
	return append(e.Errs, e.Alternate.Unwrap()...)
}

func (e *ErrUnmatchedType) errors() (result []string) {
	result = append(result, e.checkErr())
	if e.Alternate != nil {
		result = append(result, e.Alternate.errors()...)
		return result
	}
	return result
}

func (e *ErrUnmatchedType) checkErr() string {
	posStr := ""
	if e.Position != NoPosition {
		posStr = fmt.Sprintf(" (%s)", e.Position)
	}
	return fmt.Sprintf("%v%s", errors.Join(e.Errs...), posStr)
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
			buf.WriteString(",\n")
		}
		buf.WriteString(fmt.Sprintf("option %d: [%s]", i+1, errString))
	}

	return buf.String()
}

func checkType(schema *TypeSchema, right Value) (Value, error) {
	var errs []error

	if schema.TargetKind() == right.Kind() {
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

	if len(errs) == 0 {
		return right, nil
	}

	if schema.Alternate != nil {
		ret, newErrs := checkType(schema.Alternate, right)
		if newErrs == nil {
			return ret, nil
		}
		alt, ok := newErrs.(*ErrUnmatchedType)
		if ok {
			return nil, &ErrUnmatchedType{
				Position:  schema.Position,
				Errs:      errs,
				Alternate: alt,
			}
		} else {
			errs = append(errs, newErrs)
		}
	}

	if len(errs) > 0 {
		return nil, &ErrUnmatchedType{
			Position: schema.Position,
			Errs:     errs,
		}
	}

	return right, nil
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
