package value

type Summary struct {
	Types map[string]Schema `json:"types,omitempty"`
}

func makeReference(types map[string]Schema, schema Schema) Schema {
	result := &TypeSchema{
		Path:      schema.GetPath(),
		Reference: true,
	}

	s := schema.GetPath().String()
	if s == "" {
		return schema
	}
	if _, ok := types[s]; ok {
		return result
	}

	ts, ok := schema.(*TypeSchema)
	if ok && ts.Reference {
		return ts
	}

	cp := *ts
	if cp.Object != nil {
		var fields []ObjectSchemaField
		for _, field := range cp.Object.Fields {
			field.Schema = makeReference(types, field.Schema)
			fields = append(fields, field)
		}
		cp.Object.Fields = fields
	}
	if cp.Array != nil {
		var valids []Schema
		for _, valid := range cp.Array.Valid {
			valids = append(valids, makeReference(types, valid))
		}
		cp.Array.Valid = valids
	}

	types[s] = &cp

	return result
}

func Summarize(obj Schema) *Summary {
	result := &Summary{
		Types: map[string]Schema{},
	}

	if ts, ok := obj.(*TypeSchema); ok && len(obj.GetPath().String()) == 0 {
		root := "$"
		cp := *ts
		cp.Path = Path{
			PathElement{
				Key: &root,
			},
		}
		obj = &cp
	}

	makeReference(result.Types, obj)
	return result
}
