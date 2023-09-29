package value

import (
	"fmt"

	"github.com/acorn-io/aml/pkg/schema"
)

type DescribeFieldTyper interface {
	DescribeFieldType(ctx SchemaContext) (schema.FieldType, error)
}

func DescribeFieldType(ctx SchemaContext, v Value) (result schema.FieldType, _ error) {
	if ft, ok := v.(DescribeFieldTyper); ok {
		return ft.DescribeFieldType(ctx)
	}

	return result, fmt.Errorf("failed to determine field type for kind %s", TargetKind(v))
}
