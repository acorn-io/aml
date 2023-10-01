// +k8s:deepcopy-gen=package

package schema

import "fmt"

type File struct {
	Args         Object
	ProfileNames Names
}

type Names []Name

type Name struct {
	Name        string
	Description string
}

// +k8s:deepcopy-gen=false

type Schema interface {
	GetFields() []Field
}

type Object struct {
	Path         string  `json:"path,omitempty"`
	Reference    bool    `json:"reference,omitempty"`
	Description  string  `json:"description,omitempty"`
	Fields       []Field `json:"fields,omitempty"`
	AllowNewKeys bool    `json:"allowNewKeys,omitempty"`
}

func (o *Object) Merge(right *Object) (_ *Object, err error) {
	if o.Reference {
		return nil, fmt.Errorf("can not merge schema.Object reference, left [%s] is reference", o.Path)
	}
	if right.Reference {
		return nil, fmt.Errorf("can not merge schema.Object reference, right [%s] is reference", right.Path)
	}

	result := Object{
		AllowNewKeys: o.AllowNewKeys || right.AllowNewKeys,
		Description:  mergeDescription(o.Description, right.Description),
	}

	if o.Path == right.Path {
		result.Path = o.Path
	}

	fieldsByIndex := map[string]int{}
	for i, field := range o.Fields {
		fieldsByIndex[field.Name] = i
		result.Fields = append(result.Fields, field)
	}

	for _, field := range right.Fields {
		if i, ok := fieldsByIndex[field.Name]; ok {
			result.Fields[i], err = result.Fields[i].Merge(field)
			if err != nil {
				return nil, err
			}
		} else {
			fieldsByIndex[field.Name] = len(result.Fields)
			result.Fields = append(result.Fields, field)
		}
	}

	return &result, nil
}

type Array struct {
	Types []FieldType `json:"types,omitempty"`
}

func (o *Object) GetFields() []Field {
	return o.Fields
}

type Field struct {
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description,omitempty"`
	Type        FieldType `json:"type,omitempty"`
	Match       bool      `json:"match,omitempty"`
	Optional    bool      `json:"optional,omitempty"`
}

func MergeFields(fields []Field) (result []Field, err error) {
	fieldIndex := map[string]int{}

	for _, schemaField := range fields {
		if i, exists := fieldIndex[schemaField.Name]; exists {
			result[i], err = result[i].Merge(schemaField)
			if err != nil {
				return nil, err
			}
		} else {
			fieldIndex[schemaField.Name] = len(result)
			result = append(result, schemaField)
		}
	}

	return
}

func mergeDescription(left, right string) string {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	return left + "\n" + right
}

func (f *Field) Merge(right Field) (result Field, err error) {
	result = *f
	result.Description = mergeDescription(f.Description, right.Description)
	result.Match = f.Match && right.Match
	result.Optional = f.Optional && right.Optional
	result.Type, err = f.Type.Merge(right.Type)
	return result, err
}

func (f *Field) GetFields() []Field {
	return []Field{*f}
}

type FieldType struct {
	Kind       Kind         `json:"kind,omitempty"`
	Object     *Object      `json:"object,omitempty"`
	Array      *Array       `json:"array,omitempty"`
	Constraint []Constraint `json:"constraint,omitempty"`
	Default    any          `json:"default,omitempty"`
	Alternates []FieldType  `json:"alternates,omitempty"`
}

func kindOrUnion(left, right Kind) Kind {
	if left == right {
		return left
	}
	return UnionKind
}

func firstValue(left, right any) any {
	if left != nil {
		return left
	}
	return right
}

// Merge works like doing an AND condition between the two.
func (f FieldType) Merge(right FieldType) (result FieldType, err error) {
	if f.Object != nil && right.Object != nil &&
		f.Default == nil &&
		right.Default == nil &&
		len(f.Constraint) == 0 &&
		len(right.Constraint) == 0 &&
		len(f.Alternates) == 0 &&
		len(right.Alternates) == 0 {
		result = f
		result.Object, err = f.Object.Merge(right.Object)
		return result, err
	}
	return FieldType{
		Kind:    kindOrUnion(f.Kind, right.Kind),
		Default: firstValue(f.Default, right.Default),
		Constraint: []Constraint{
			{
				Op:    "type",
				Right: f,
			},
			{
				Op:    "type",
				Right: right,
			},
		},
	}, nil
}

type Constraint struct {
	Description string `json:"description,omitempty"`
	Op          string `json:"op,omitempty"`
	Right       any    `json:"right,omitempty"`
}
