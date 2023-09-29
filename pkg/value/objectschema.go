package value

import (
	"errors"
	"fmt"
	"strings"

	"github.com/acorn-io/aml/pkg/schema"
)

type DescribeObjecter interface {
	DescribeObject(ctx SchemaContext) (*schema.Object, bool, error)
}

func DescribeObject(ctx SchemaContext, val Value) (*schema.Object, error) {
	if err := assertType(val, SchemaKind); err != nil {
		return nil, err
	}
	if s, ok := val.(DescribeObjecter); ok {
		schema, ok, err := s.DescribeObject(ctx)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("value kind %s did not provide a schema object description", val.Kind())
		}
		return schema, nil
	}
	return nil, fmt.Errorf("value kind %s can not be converted to schema object description", val.Kind())
}

type ObjectSchema struct {
	Contract Contract
}

type Contract interface {
	Position() Position
	Path() string
	Description() string
	Fields(ctx SchemaContext) ([]schema.Field, error)
	AllKeys() ([]string, error)
	RequiredKeys() ([]string, error)
	LookupValueForKeyEquals(key string) (Value, bool, error)
	LookupValueForKeyPatternMatch(key string) (Value, bool, error)
	AllowNewKeys() bool
}

func NewClosedObject() *TypeSchema {
	return &TypeSchema{
		KindValue: ObjectKind,
		Object: &ObjectSchema{
			Contract: noFields{},
		},
	}
}

func NewObjectSchema(contract Contract) *TypeSchema {
	return &TypeSchema{
		Position:  contract.Position(),
		KindValue: ObjectKind,
		Object: &ObjectSchema{
			Contract: contract,
		},
	}
}

func (n *ObjectSchema) GetContract() (Contract, bool) {
	return n.Contract, true
}

func (n *ObjectSchema) TargetKind() Kind {
	return ObjectKind
}

func (n *ObjectSchema) Kind() Kind {
	return SchemaKind
}

func (n *ObjectSchema) Fields(ctx SchemaContext) (result []schema.Field, _ error) {
	fields, err := n.Contract.Fields(ctx)
	if err != nil {
		return nil, err
	}

	var (
		fieldNames   = map[string]int{}
		mergedFields []schema.Field
	)

	for _, field := range fields {
		if i, ok := fieldNames[field.Name]; ok {
			mergedFields[i] = mergedFields[i].Merge(field)
		} else {
			fieldNames[field.Name] = len(mergedFields)
			mergedFields = append(mergedFields, field)
		}
	}

	return mergedFields, nil
}

func (n *ObjectSchema) DescribeObject(ctx SchemaContext) (*schema.Object, bool, error) {
	if ctx.haveSeen(n.Contract.Path()) {
		return &schema.Object{
			Description:  n.Contract.Description(),
			Path:         n.Contract.Path(),
			Reference:    true,
			AllowNewKeys: n.Contract.AllowNewKeys(),
		}, true, nil
	}

	ctx.addSeen(n.Contract.Path())

	fields, err := n.Fields(ctx)
	if err != nil {
		return nil, false, err
	}

	return &schema.Object{
		Description:  n.Contract.Description(),
		Path:         n.Contract.Path(),
		Fields:       fields,
		AllowNewKeys: n.Contract.AllowNewKeys(),
	}, true, nil
}

func (n *ObjectSchema) Keys() ([]string, error) {
	return n.Contract.AllKeys()
}

func (n *ObjectSchema) LookupValue(key Value) (Value, bool, error) {
	s, err := ToString(key)
	if err != nil {
		return nil, false, err
	}
	return n.Contract.LookupValueForKeyEquals(s)
}

func (n *ObjectSchema) getSchemaForKey(key string) (Value, bool, error) {
	schemaValue, ok, err := n.Contract.LookupValueForKeyEquals(key)
	if err != nil {
		return nil, false, err
	} else if ok {
		return schemaValue, true, nil
	}

	return n.Contract.LookupValueForKeyPatternMatch(key)
}

type ErrSchemaViolation struct {
	Path string
	Key  string
	Err  error
}

func (e *ErrSchemaViolation) Unwrap() error {
	return e.Err
}

func (e *ErrSchemaViolation) Error() string {
	bottom := BottomLeftMost(e)
	s := fmt.Sprintf("schema violation %s.%s: %v", bottom.Path, bottom.Key, bottom.Err)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

type x interface {
	comparable
	error
}

func unwrapOnce(err error) error {
	next := errors.Unwrap(err)
	if next != nil {
		return next
	}

	list, ok := err.(interface{ Unwrap() []error })
	if ok {
		errs := list.Unwrap()
		if len(errs) > 0 {
			return errs[0]
		}
	}

	return nil
}

func BottomLeftMost[T x](start T) T {
	var (
		last       = start
		cur  error = start
	)

	for {
		next := unwrapOnce(cur)
		if next == nil {
			break
		}
		if x, ok := next.(T); ok {
			last = x
		}
		cur = next
	}

	return last
}

func (n *ObjectSchema) Merge(right Value) (Value, error) {
	var (
		head []Entry
		tail []Entry
	)

	if schema, ok := right.(*ObjectSchema); ok {
		return NewValue(&mergedContract{
			Left:  n.Contract,
			Right: schema.Contract,
		}), nil
	}

	if err := assertType(right, ObjectKind); err != nil {
		// This is a check that the schema doesn't have an invalid embeeded
		_, _, serr := n.DescribeObject(SchemaContext{})
		if serr != nil {
			return nil, errors.Join(err, serr)
		}
		return nil, err
	}

	requiredKeys, err := n.Contract.RequiredKeys()
	if err != nil {
		return nil, err
	}

	keys, err := Keys(right)
	if err != nil {
		return nil, err
	}

	keysSeen := map[string]struct{}{}

	for _, key := range keys {
		rightValue, ok, err := Lookup(right, NewValue(key))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		keysSeen[key] = struct{}{}

		schemaValue, ok, err := n.getSchemaForKey(key)
		if err != nil {
			return nil, err
		}
		if ok {
			rightValue, err = Merge(schemaValue, rightValue)
			if err != nil {
				return nil, &ErrSchemaViolation{
					Key:  key,
					Path: n.Contract.Path(),
					Err:  err,
				}
			}
		} else if !n.Contract.AllowNewKeys() {
			return nil, &ErrUnknownField{
				Path: n.Contract.Path(),
				Key:  key,
			}
		}

		tail = append(tail, Entry{
			Key:   key,
			Value: rightValue,
		})
	}

	var missingKeys []string
	for _, k := range requiredKeys {
		if _, seen := keysSeen[k]; seen {
			continue
		}
		def, ok, err := n.getSchemaForKey(k)
		if err != nil {
			return nil, err
		}
		if def, hasDefault, err := DefaultValue(def); err != nil {
			return nil, err
		} else if ok && hasDefault {
			head = append(head, Entry{
				Key:   k,
				Value: def,
			})
		} else {
			missingKeys = append(missingKeys, k)
		}
	}

	if len(missingKeys) > 0 {
		return nil, &ErrMissingRequiredKeys{
			Path: n.Contract.Path(),
			Keys: missingKeys,
		}
	}

	return &Object{
		Entries: append(head, tail...),
	}, nil
}

var _ Contract = (*mergedContract)(nil)

type mergedContract struct {
	Left, Right Contract
}

func (m *mergedContract) Position() Position {
	if pos := m.Left.Position(); pos != NoPosition {
		return pos
	}
	return m.Right.Position()
}

func (m *mergedContract) Description() string {
	left, right := m.Left.Description(), m.Right.Description()
	var parts []string
	if left != "" {
		parts = append(parts, left)
	}
	if right != "" {
		parts = append(parts, right)
	}
	return strings.Join(parts, "\n")
}

func (m *mergedContract) Fields(ctx SchemaContext) ([]schema.Field, error) {
	leftFields, err := m.Left.Fields(ctx)
	if err != nil {
		return nil, err
	}

	rightFields, err := m.Right.Fields(ctx)
	if err != nil {
		return nil, err
	}

	return append(leftFields, rightFields...), nil
}

func (m *mergedContract) Path() string {
	return m.Left.Path()
}

func (m *mergedContract) AllKeys() ([]string, error) {
	result, err := m.Left.AllKeys()
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, key := range result {
		seen[key] = struct{}{}
	}

	rightKeys, err := m.Right.AllKeys()
	if err != nil {
		return nil, err
	}

	for _, key := range rightKeys {
		if _, ok := seen[key]; !ok {
			result = append(result, key)
			seen[key] = struct{}{}
		}
	}

	return result, nil
}

func (m *mergedContract) RequiredKeys() ([]string, error) {
	result, err := m.Left.RequiredKeys()
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, key := range result {
		seen[key] = struct{}{}
	}

	rightKeys, err := m.Right.RequiredKeys()
	if err != nil {
		return nil, err
	}

	for _, key := range rightKeys {
		if _, ok := seen[key]; !ok {
			result = append(result, key)
			seen[key] = struct{}{}
		}
	}

	return result, nil
}

func (m *mergedContract) LookupValueForKeyEquals(key string) (Value, bool, error) {
	return m.lookupValue(
		m.Left.LookupValueForKeyEquals,
		m.Right.LookupValueForKeyEquals,
		key)
}

func (m *mergedContract) LookupValueForKeyPatternMatch(key string) (Value, bool, error) {
	return m.lookupValue(
		m.Left.LookupValueForKeyPatternMatch,
		m.Right.LookupValueForKeyPatternMatch,
		key)
}

func (m *mergedContract) lookupValue(leftLookup, rightLookup func(string) (Value, bool, error), key string) (Value, bool, error) {
	leftValue, ok, err := leftLookup(key)
	if err != nil {
		return nil, false, err
	}

	if !ok {
		return rightLookup(key)
	}

	rightValue, ok, err := rightLookup(key)
	if err != nil {
		return nil, false, err
	}

	if !ok {
		return leftValue, true, nil
	}

	result, err := Merge(leftValue, rightValue)
	return result, true, err
}

func (m *mergedContract) AllowNewKeys() bool {
	return m.Left.AllowNewKeys() || m.Right.AllowNewKeys()
}

type ErrUnknownField struct {
	Path string
	Key  string
}

func (e *ErrUnknownField) Error() string {
	return fmt.Sprintf("unknown field: %s.%s", e.Path, e.Key)
}

type ErrMissingRequiredKeys struct {
	Path string
	Keys []string
}

func (e *ErrMissingRequiredKeys) Error() string {
	var keys []string
	for _, key := range e.Keys {
		keys = append(keys, e.Path+"."+key)
	}
	return fmt.Sprintf("missing required key(s): %v", keys)
}

type noFields struct {
}

func (n noFields) Position() Position {
	return Position{}
}

func (n noFields) Path() string {
	return ""
}

func (n noFields) Description() string {
	return ""
}

func (n noFields) Fields(ctx SchemaContext) ([]schema.Field, error) {
	return nil, nil
}

func (n noFields) AllKeys() ([]string, error) {
	return nil, nil
}

func (n noFields) RequiredKeys() ([]string, error) {
	return nil, nil
}

func (n noFields) LookupValueForKeyEquals(key string) (Value, bool, error) {
	return nil, false, nil
}

func (n noFields) LookupValueForKeyPatternMatch(key string) (Value, bool, error) {
	return nil, false, nil
}

func (n noFields) AllowNewKeys() bool {
	return false
}
