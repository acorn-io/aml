package value

import (
	"fmt"

	"github.com/acorn-io/aml/pkg/jsonschema"
)

func jsonSchemaConvert(rootName string, summary *Summary) (*jsonschema.Schema, error) {
	if rootName == "" {
		rootName = "$"
	}

	defs := map[string]jsonschema.Schema{}
	root, ok := summary.Types[rootName]
	if !ok {
		return nil, fmt.Errorf("failed to find root schema")
	}

	result, err := toSchema(defs, summary, root.(*TypeSchema))
	if err != nil || result == nil {
		return nil, err
	}
	result.Defs = defs
	return result, nil
}

func toRef(defs map[string]jsonschema.Schema, summary *Summary, amlSchema *TypeSchema) (*jsonschema.Schema, error) {
	schema, err := toSchema(defs, summary, amlSchema)
	if err != nil || schema == nil {
		return nil, err
	}
	key := amlSchema.Path.String()
	if key != "" {
		if !amlSchema.Reference {
			defs[amlSchema.Path.String()] = *schema
		}
		return &jsonschema.Schema{
			Property: jsonschema.Property{
				Ref: "#/$defs/" + amlSchema.Path.String(),
			},
		}, nil
	}
	return schema, nil
}

func toSchema(defs map[string]jsonschema.Schema, summary *Summary, amlSchema *TypeSchema) (*jsonschema.Schema, error) {
	if amlSchema.Reference {
		schema, ok := defs[amlSchema.Path.String()]
		if ok {
			return &schema, nil
		}
		target := summary.Types[amlSchema.Path.String()]
		newSchema, err := toSchema(defs, summary, target.(*TypeSchema))
		if err != nil || newSchema == nil {
			return newSchema, err
		}

		defs[amlSchema.Path.String()] = *newSchema
		return newSchema, nil
	}

	switch amlSchema.KindValue {
	case StringKind:
		return &jsonschema.Schema{
			Property: jsonschema.Property{
				Type: "string",
			},
		}, nil
	case BoolKind:
		return &jsonschema.Schema{
			Property: jsonschema.Property{
				Type: "boolean",
			},
		}, nil
	case NumberKind:
		return &jsonschema.Schema{
			Property: jsonschema.Property{
				Type: "number",
			},
		}, nil
	case ArrayKind:
		array := &jsonschema.Schema{
			Property: jsonschema.Property{
				Type: "array",
			},
		}
		if amlSchema.Array == nil {
			return array, nil
		}
		array.Property.Description = amlSchema.Array.Description
		var items []jsonschema.Schema
		for _, valid := range amlSchema.Array.Valid {
			if ts, ok := valid.(*TypeSchema); ok {
				ref, err := toRef(defs, summary, ts)
				if err != nil || ref == nil {
					return nil, err
				}
				items = append(items, *ref)
			}
		}
		array.Description = amlSchema.Array.Description
		array.Items = items
		return array, nil
	case ObjectKind:
		obj := &jsonschema.Schema{
			Property: jsonschema.Property{
				Type: "object",
			},
			Properties: map[string]jsonschema.Property{},
		}
		if amlSchema.Object == nil {
			return obj, nil
		}

		for _, prop := range amlSchema.Object.Fields {
			if prop.Key == "" {
				continue
			}
			if ts, ok := prop.Schema.(*TypeSchema); ok {
				property, err := toRef(defs, summary, ts)
				if err != nil || property == nil {
					return nil, err
				}
				property.Description = prop.Description
				obj.Properties[prop.Key] = property.Property
			}
		}

		obj.Description = amlSchema.Object.Description
		return obj, nil
	}

	return nil, nil
}
