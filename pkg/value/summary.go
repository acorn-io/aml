package value

type Summary struct {
	Types map[string]*TypeSchema `json:"types,omitempty"`
}

func makeReference(types map[string]*TypeSchema, ts *TypeSchema) *TypeSchema {
	if ts.Reference {
		return ts
	}

	result := &TypeSchema{
		Path:      ts.Path,
		Reference: true,
	}

	s := ts.Path.String()
	if s == "" {
		return ts
	}
	if _, ok := types[s]; ok {
		return result
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
		var valids []*TypeSchema
		for _, valid := range cp.Array.Valid {
			valids = append(valids, makeReference(types, valid))
		}
		cp.Array.Valid = valids
	}

	types[s] = &cp

	return result
}

func Summarize(obj *TypeSchema) *Summary {
	result := &Summary{
		Types: map[string]*TypeSchema{},
	}

	if len(obj.Path) == 0 {
		root := "$"
		cp := *obj
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
