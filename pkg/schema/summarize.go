package schema

type Summary struct {
	Types  map[string]FieldType `json:"types,omitempty"`
	Fields []Field              `json:"fields,omitempty"`
}

func Summarize(obj Object) Summary {
	types := map[string]FieldType{}
	return Summary{
		Types:  types,
		Fields: addFields(types, obj.Fields),
	}
}

func addType(types map[string]FieldType, fieldType FieldType) FieldType {
	if fieldType.Alternate != nil {
		newAlt := addType(types, *fieldType.Alternate)
		fieldType.Alternate = &newAlt
	}

	if fieldType.Object != nil && !fieldType.Object.Reference && fieldType.Object.Path != "" {
		cp := fieldType.Object
		cp.Fields = addFields(types, cp.Fields)
		types[fieldType.Object.Path] = fieldType

		fieldType.Object = &Object{
			Path:      fieldType.Object.Path,
			Reference: true,
		}
	}

	return fieldType
}

func addFields(types map[string]FieldType, fields []Field) (result []Field) {
	for _, field := range fields {
		field.Type = addType(types, field.Type)
		result = append(result, field)
	}
	return
}
