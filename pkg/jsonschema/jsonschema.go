// +k8s:deepcopy-gen=package

package jsonschema

import "encoding/json"

// +k8s:openapi-gen=true
type Schema struct {
	Property

	ID         string              `json:"$id,omitempty"`
	Title      string              `json:"title,omitempty"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
	Defs       map[string]Schema   `json:"defs,omitempty"`

	AdditionalProperties bool `json:"additionalProperties,omitempty"`
}

//type Any any

// +k8s:openapi-gen=true
type Property struct {
	Description string `json:",omitempty"`
	Type        string `json:"type,omitempty"`
	Ref         string `json:"$ref,omitempty"`

	//Const Any   `json:"const,omitempty"`
	//Enum  []Any `json:"enum,omitempty"`

	// For arrays
	Items []Schema `json:"items,omitempty"`
}

type Type []string

func (t *Type) UnmarshalJSON(data []byte) error {
	switch data[0] {
	case '[':
		return json.Unmarshal(data, (*[]string)(t))
	case 'n':
		return json.Unmarshal(data, (*[]string)(t))
	default:
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*t = []string{s}
	}
	return nil
}

func (t *Type) MarshalJSON() ([]byte, error) {
	switch len(*t) {
	case 0:
		return json.Marshal(nil)
	case 1:
		return json.Marshal((*t)[0])
	default:
		return json.Marshal(*t)
	}
}
