// +k8s:deepcopy-gen=package

package schema

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

type Array struct {
	Items FieldType `json:"item,omitempty"`
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

func (f *Field) Merge(right Field) (result Field) {
	result = *f
	if right.Description != "" {
		if result.Description != "" {
			result.Description = result.Description + "\n" + right.Description
		} else {
			result.Description = right.Description
		}
	}
	result.Match = f.Match && right.Match
	result.Optional = f.Optional && right.Optional
	result.Type = f.Type.Merge(right.Type)
	return result
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
	Alternate  *FieldType   `json:"alternate,omitempty"`
}

func mergeAlternate(left, right *FieldType) *FieldType {
	if left == nil {
		return right
	}

	cp := *left
	if cp.Alternate == nil {
		cp.Alternate = right
	} else {
		cp.Alternate = mergeAlternate(left.Alternate, right)
	}
	return &cp
}

// Merge works like doing an AND condition between the two. The Kind is assumed to already match
func (f FieldType) Merge(right FieldType) (result FieldType) {
	result = f
	if right.Object != nil {
		result.Object = right.Object
	}
	result.Constraint = append(f.Constraint, right.Constraint...)
	if right.Default != nil {
		f.Default = right.Default
	}

	result.Alternate = mergeAlternate(result.Alternate, right.Alternate)
	return result
}

type Constraint struct {
	Description string `json:"description,omitempty"`
	Op          string `json:"op,omitempty"`
	Left        any    `json:"left,omitempty"`
	Right       any    `json:"right,omitempty"`
}
