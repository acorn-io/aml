package flagargs

import (
	"bytes"
	"os"
	"testing"

	"github.com/acorn-io/aml"
	"github.com/acorn-io/aml/pkg/schema"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	args, profiles, err := ParseArgs(
		"testdata/TestParseArgs/input-args.acorn",
		"testdata/TestParseArgs/input.acorn",
		[]string{"--foo", "bar", "--profile=one", "--profile", "two"})
	require.NoError(t, err)

	autogold.Expect([]string{
		"one",
		"two",
	}).Equal(t, profiles)
	autogold.Expect(map[string]interface{}{
		"foo": "bar",
	}).Equal(t, args)
}

func TestParseInvalidFlag(t *testing.T) {
	_, _, err := ParseArgs(
		"testdata/TestParseArgs/input-args.acorn",
		"testdata/TestParseArgs/input.acorn",
		[]string{"--foo2", "bar"})

	autogold.Expect("unknown flag: --foo2").Equal(t, err.Error())
}

func TestHelp(t *testing.T) {
	var file schema.File

	data, err := os.ReadFile("testdata/TestParseArgs/input.acorn")
	require.NoError(t, err)

	err = aml.Unmarshal(data, &file)
	require.NoError(t, err)

	buffer := &bytes.Buffer{}

	flags := New(
		"testdata/TestParseArgs/input-args.acorn",
		"testdata/TestParseArgs/input.acorn",
		file.ProfileNames,
		file.Args)
	flags.FlagSet.SetOutput(buffer)

	_, _, err = flags.Parse([]string{"--help"})
	autogold.Expect("pflag: help requested").Equal(t, err.Error())
	autogold.ExpectFile(t, autogold.Raw(buffer.String()))
}
