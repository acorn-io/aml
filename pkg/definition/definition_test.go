package definition

import (
	"os"
	"testing"
)

func TestStd(t *testing.T) {
	data, err := os.ReadFile("../std/std_test.cue")
	if err != nil {
		t.Fatal(err)
	}

	def, err := NewDefinition(NewAcornfile(data))
	if err != nil {
		t.Fatal(err)
	}

	d := map[string]any{}
	err = def.Decode(&d)
	if err != nil {
		t.Fatal(err)
	}
}
