package aml

import (
	"strings"
	"testing"

	"github.com/acorn-io/aml/pkg/value"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
)

const testDocument = `
args: {
	// Foo
	foo: 1
}
args: two: 10
args: bar: number < 10 || 1
x: args.foo + args.bar + args.two
profiles: baz: two: 2
`

func TestUnmarshal(t *testing.T) {
	data := map[string]any{}

	err := Unmarshal([]byte(testDocument), &data, DecoderOption{
		PositionalArgs: []any{3},
		Args: map[string]any{
			"bar": 2,
		},
		Profiles: []string{"baz", "missing?"},
	})
	require.NoError(t, err)

	autogold.Expect(map[string]interface{}{
		"x": 7,
	}).Equal(t, data)
}

func TestSchemaValidate(t *testing.T) {
	out := map[string]any{}
	err := NewDecoder(strings.NewReader(`
b: "test"
`), DecoderOption{
		Schema: strings.NewReader(`
a: 1
b: string
`),
	}).Decode(&out)

	require.NoError(t, err)
	autogold.Expect(map[string]interface{}{"a": 1, "b": "test"}).Equal(t, out)
}

func TestSchemaUnmarshal(t *testing.T) {
	out := &value.FuncSchema{}
	err := Unmarshal([]byte(testDocument), out)
	require.NoError(t, err)

	// autogold can't handle this
	out.Returns = nil

	autogold.Expect(&value.FuncSchema{
		Returns: nil,
		Args: []value.ObjectSchemaField{
			{
				Key:         "foo",
				Description: "Foo",
				Schema: &value.TypeSchema{
					Positions: []value.Position{{
						Filename: "<inline>",
						Offset:   18,
						Line:     4,
						Column:   2,
					}},
					KindValue:    value.Kind("number"),
					DefaultValue: value.Number("1"),
				},
			},
			{
				Key: "two",
				Schema: &value.TypeSchema{
					Positions: []value.Position{{
						Filename: "<inline>",
						Offset:   33,
						Line:     6,
						Column:   7,
					}},
					KindValue:    value.Kind("number"),
					DefaultValue: value.Number("10"),
				},
			},
			{
				Key: "bar",
				Schema: &value.TypeSchema{
					Positions:   []value.Position{{}},
					KindValue:   value.Kind("number"),
					Constraints: []value.Constraint{{Op: "mustMatchAlternate"}},
					Alternates: []*value.TypeSchema{
						{
							KindValue: value.Kind("number"),
							Constraints: []value.Constraint{{
								Op:    "<",
								Right: value.Number("10"),
							}},
						},
						{
							Positions: []value.Position{{}},
							KindValue: value.Kind("number"),
							Constraints: []value.Constraint{{
								Op:    "==",
								Right: value.Number("1"),
							}},
							DefaultValue: value.Number("1"),
						},
					},
				},
			},
		},
		ProfileNames: value.Names{value.Name{Name: "baz"}},
	}).Equal(t, out)
}
