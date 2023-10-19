package value

import (
	"context"
	"encoding/json"
)

type FuncSchema struct {
	Args         []ObjectSchemaField `json:"args"`
	ProfileNames Names               `json:"profileNames"`
	Returns      ToSchemaFunc        `json:"returns,omitempty"`
}

func (f *FuncSchema) MarshalJSON() ([]byte, error) {
	data := map[string]any{
		"args":         f.Args,
		"profileNames": f.ProfileNames,
	}
	if f.Returns != nil {
		v, ok, err := f.Returns(context.Background())
		if err != nil {
			return nil, err
		}
		if ok {
			data["returns"] = v
		}
	}
	return json.Marshal(data)
}

type ToSchemaFunc func(context.Context) (Schema, bool, error)

type Names []Name

type Name struct {
	Name        string
	Description string
}

func (n Names) Describe() (result Names) {
	for _, name := range n {
		result = append(result, name)
	}
	return
}
