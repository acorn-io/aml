package definition

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParamTypes(t *testing.T) {
	acornCue := `
args: {
	s: "string"
	b: true
	i: 4
	f: 5.0
	e: "hi" | "bye"
	a: ["hi"]
	o: {}
}
`
	def, err := NewDefinition(NewAcornfile([]byte(acornCue)))
	if err != nil {
		t.Fatal(err)
	}

	spec, err := def.Args()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "string", spec.Params[0].Type)
	assert.Equal(t, "bool", spec.Params[1].Type)
	assert.Equal(t, "int", spec.Params[2].Type)
	assert.Equal(t, "float", spec.Params[3].Type)
	assert.Equal(t, "enum", spec.Params[4].Type)
	assert.Equal(t, "array", spec.Params[5].Type)
	assert.Equal(t, "object", spec.Params[6].Type)
}

func TestParamProfiles(t *testing.T) {
	acornCue := `
args: {}
`
	def, err := NewDefinition(NewAcornfile([]byte(acornCue)))
	if err != nil {
		t.Fatal(err)
	}

	spec, err := def.Args()
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, spec.Profiles, 0)
}

func TestParamSpec(t *testing.T) {
	acornCue := `
args: {
  // Description of a string param
  foo: string

  // Two line Description of an int
  // Description of an int with default
//
  bar: int | *4
// This is dropped

// Complex  value 
  complex: {
    foo: string
  }
}
`
	def, err := NewDefinition(NewAcornfile([]byte(acornCue)))
	if err != nil {
		t.Fatal(err)
	}

	spec, err := def.Args()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "foo", spec.Params[0].Name)
	assert.Equal(t, "string", spec.Params[0].Schema)
	assert.Equal(t, "Description of a string param", spec.Params[0].Description)

	assert.Equal(t, "bar", spec.Params[1].Name)
	assert.Equal(t, "*4 | int", spec.Params[1].Schema)
	assert.Equal(t, "Two line Description of an int\nDescription of an int with default", spec.Params[1].Description)

	assert.Equal(t, "complex", spec.Params[2].Name)
	assert.Equal(t, "{\n\tfoo: string\n}", spec.Params[2].Schema)
	assert.Equal(t, "Complex  value", spec.Params[2].Description)
}

func TestJSONFloatParsing(t *testing.T) {
	data := []byte(`
args: {
	replicas: 1
}

profiles: {
	prod: {
		replicas: 2
	}
}

containers: {
	web: {
		image: "public.ecr.aws/docker/library/nginx:latest"
		scale: args.replicas
	}
}`)

	appDef, err := NewDefinition(NewAcornfile(data))
	if err != nil {
		t.Fatal(err)
	}

	params := map[string]interface{}{
		"replicas": 3,
	}

	appDef, args, err := appDef.WithArgs(params, []string{"prod"})
	if err != nil {
		t.Fatal(err)
	}

	result := map[string]any{}
	err = appDef.Decode(&result)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 3, args["replicas"])
	assert.Equal(t, float64(3), result["containers"].(map[string]any)["web"].(map[string]any)["scale"])
}
