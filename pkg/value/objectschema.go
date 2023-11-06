package value

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type ObjectSchema struct {
	Positions    []Position          `json:"-"`
	AllowNewKeys bool                `json:"allowNewKeys"`
	Description  string              `json:"description"`
	Fields       []ObjectSchemaField `json:"fields"`
}

type ObjectSchemaField struct {
	Key         string `json:"key"`
	Match       bool   `json:"match"`
	Optional    bool   `json:"optional"`
	Description string `json:"description"`
	Schema      Schema `json:"schema"`
}

func NewOpenObject() *TypeSchema {
	return &TypeSchema{
		KindValue: ObjectKind,
		Object: &ObjectSchema{
			AllowNewKeys: true,
		},
	}
}

func NewClosedObject() *TypeSchema {
	return &TypeSchema{
		KindValue: ObjectKind,
		Object:    &ObjectSchema{},
	}
}

func mergePath(left, right Path) Path {
	return left
}

func (n *ObjectSchema) Merge(right *ObjectSchema) (*ObjectSchema, error) {
	if n == nil {
		return right, nil
	} else if right == nil {
		return n, nil
	}

	fields, err := mergeObjectSchemaFields(n, right)
	if err != nil {
		return nil, err
	}
	return &ObjectSchema{
		Positions:    mergePositions(n.Positions, right.Positions),
		AllowNewKeys: n.AllowNewKeys || right.AllowNewKeys,
		Description:  mergeDescription(n.Description, right.Description),
		Fields:       fields,
	}, nil
}

func mergeObjectSchemaFields(left, right *ObjectSchema) (result []ObjectSchemaField, err error) {
	if left == nil && right == nil {
		return nil, nil
	} else if left == nil {
		return right.Fields, nil
	} else if right == nil {
		return left.Fields, nil
	}

	existing := map[string]int{}
	for _, leftField := range left.Fields {
		existing[leftField.Key] = len(result)
		result = append(result, leftField)
	}

	for _, rightField := range right.Fields {
		if leftFieldIndex, ok := existing[rightField.Key]; ok {
			leftField := result[leftFieldIndex]
			schema, err := leftField.Schema.MergeType(rightField.Schema)
			if err != nil {
				return nil, err
			}
			mergedField := ObjectSchemaField{
				Key:         rightField.Key,
				Match:       leftField.Match || rightField.Match,
				Optional:    leftField.Optional && rightField.Optional,
				Description: mergeDescription(leftField.Description, rightField.Description),
				Schema:      schema,
			}
			result[leftFieldIndex] = mergedField
		} else {
			result = append(result, rightField)
		}
	}

	return result, nil
}

func (n *ObjectSchema) ImpliedDefault() (Value, bool, error) {
	data := map[string]any{}
	for _, field := range n.Fields {
		if field.Match || field.Optional {
			continue
		}
		v, ok, err := DefaultValue(field.Schema)
		if err != nil {
			return nil, false, err
		} else if !ok {
			return nil, false, nil
		}
		data[field.Key] = v
	}

	return NewValue(data), true, nil
}

func (n *ObjectSchema) validateKey(ctx context.Context, key string, value Value, schemaPath Path) (newValue Value, matched bool, _ error) {
	for _, field := range n.Fields {
		// look for matches next
		if field.Match {
			continue
		}

		if field.Key == key {
			v, err := field.Schema.Validate(ctx, value)
			if err != nil {
				return nil, true, &ErrSchemaViolation{
					Key:        key,
					DataPath:   GetDataPath(ctx),
					SchemaPath: schemaPath,
					Err:        err,
				}
			}
			return v, true, nil
		}
	}

	for _, field := range n.Fields {
		// Only looking for matches next
		if !field.Match {
			continue
		}

		if matched, err := regexp.MatchString(field.Key, key); err != nil {
			return nil, true, &ErrSchemaViolation{
				Key:        key,
				DataPath:   GetDataPath(ctx),
				SchemaPath: schemaPath,
				Err:        err,
			}
		} else if !matched {
			continue
		}

		v, err := field.Schema.Validate(ctx, value)
		if err != nil {
			return v, true, &ErrSchemaViolation{
				Key:        key,
				DataPath:   GetDataPath(ctx),
				SchemaPath: schemaPath,
				Err:        err,
			}
		}
		return v, true, nil
	}

	return nil, false, nil
}

type ErrSchemaViolation struct {
	DataPath   Path
	SchemaPath Path
	Key        string
	Err        error
}

func (e *ErrSchemaViolation) Unwrap() error {
	return e.Err
}

func (e *ErrSchemaViolation) Error() string {
	var (
		cur      error = e
		keyPaths []string
		last     = e
	)

	if e.Key != "" {
		keyPaths = []string{e.Key}
	}

	for cur != nil {
		next := errors.Unwrap(cur)
		if next == nil {
			if l, ok := cur.(interface {
				Unwrap() []error
			}); ok {
				errs := l.Unwrap()
				if len(errs) > 0 {
					next = errs[0]
				}
			}
		}
		cur = next
		if ev, ok := cur.(*ErrSchemaViolation); ok {
			if ev.Key != "" {
				keyPaths = append(keyPaths, ev.Key)
			}
			last = ev
		}
	}

	var (
		keyPath = strings.Join(keyPaths, ".")
		suffix  = pathSuffix(last.DataPath, last.SchemaPath)
	)
	s := fmt.Sprintf("schema violation key %s: %v%s", keyPath, last.Err, suffix)
	if len(s) > 1000 {
		return s[:1000] + "..."
	}
	return s
}

func pathSuffix(dataPath, schemaPath Path) (suffix string) {
	if dataPathString := dataPath.String(); dataPathString != "" {
		suffix = fmt.Sprintf(" [path %s]", dataPathString)
	}
	if pathString := schemaPath.String(); pathString != "" {
		suffix += fmt.Sprintf(" [schema path %s]", pathString)
	}
	return
}

func (n *ObjectSchema) Validate(ctx context.Context, right Value, schemaPath Path) (Value, error) {
	var (
		head []Entry
		tail []Entry
	)

	if err := assertType(right, ObjectKind); err != nil {
		return nil, NewErrPosition(lastPos(n.Positions, nil), err)
	}

	keys, err := Keys(right)
	if err != nil {
		return nil, err
	}

	keysSeen := map[string]struct{}{}

	for _, key := range keys {
		ctx := WithDataKeyPath(ctx, key)
		rightValue, ok, err := Lookup(right, NewValue(key))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		keysSeen[key] = struct{}{}

		newValue, ok, err := n.validateKey(ctx, key, rightValue, schemaPath)
		if err != nil {
			return nil, err
		}
		if ok {
			rightValue = newValue
		} else if !n.AllowNewKeys {
			return nil, &ErrUnknownField{
				DataPath:   GetDataPath(ctx),
				SchemaPath: schemaPath,
				Key:        key,
			}
		}

		tail = append(tail, Entry{
			Key:   key,
			Value: rightValue,
		})
	}

	var missingKeys []string
	for _, field := range n.Fields {
		if field.Match || field.Optional {
			continue
		}
		if _, seen := keysSeen[field.Key]; seen {
			continue
		}
		if def, hasDefault, err := DefaultValue(field.Schema); err != nil {
			return nil, err
		} else if hasDefault {
			head = append(head, Entry{
				Key:   field.Key,
				Value: def,
			})
		} else {
			missingKeys = append(missingKeys, field.Key)
		}
	}

	if len(missingKeys) > 0 {
		return nil, &ErrMissingRequiredKeys{
			DataPath:   GetDataPath(ctx),
			SchemaPath: schemaPath,
			Keys:       missingKeys,
		}
	}

	result := &Object{
		Entries: append(head, tail...),
	}

	// Bind functions
	for i, entry := range result.Entries {
		if entry.Value.Kind() == FuncKind {
			result.Entries[i].Value = ObjectFunc{
				Self: result,
				Func: entry.Value,
			}
		}
	}

	return result, nil
}

type ErrUnknownField struct {
	SchemaPath Path
	DataPath   Path
	Key        string
}

func (e *ErrUnknownField) Error() string {
	return fmt.Sprintf("unknown field %s%s", e.Key, pathSuffix(e.DataPath, e.SchemaPath))
}

type ErrMissingRequiredKeys struct {
	SchemaPath Path
	DataPath   Path
	Keys       []string
}

func (e *ErrMissingRequiredKeys) Error() string {
	if len(e.Keys) == 1 {
		return fmt.Sprintf("missing required key %s%s", e.Keys[0], pathSuffix(e.DataPath, e.SchemaPath))
	}
	return fmt.Sprintf("missing required keys %v%s", e.Keys, pathSuffix(e.DataPath, e.SchemaPath))
}
