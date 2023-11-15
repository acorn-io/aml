package flagargs

import (
	"bytes"
	"os"
	"testing"

	"github.com/acorn-io/aml"
	"github.com/acorn-io/aml/pkg/value"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	argsData, profiles, args, err := ParseArgs(
		"testdata/TestParseArgs/input-args.acorn",
		"testdata/TestParseArgs/input.acorn",
		[]string{"--foo", "from-cli", "--profile=one", "--profile", "two", "arg1", "arg2"})
	require.NoError(t, err)

	autogold.Expect([]string{
		"one",
		"two",
	}).Equal(t, profiles)
	autogold.Expect(map[string]interface{}{"anObject": map[string]interface{}{"aFour": "six", "aThree": 5}, "foo": "from-cli", "foo2": "from-arg-file"}).Equal(t, argsData)
	autogold.Expect([]string{"arg1", "arg2"}).Equal(t, args)
}

func TestParseStringObject(t *testing.T) {
	argsData, _, _, err := ParseArgs(
		"",
		"testdata/TestParseArgs/input.acorn",
		[]string{
			"--foo3", "@testdata/TestParseArgs/input.yaml",
			"--anObject", "@testdata/TestParseArgs/input.yaml",
		})

	require.NoError(t, err)
	autogold.Expect(map[string]any{
		"anObject": map[string]any{
			"aKey": 3,
		}, "foo3": "aKey: 3",
	}).Equal(t, argsData)
}

func TestParseInvalidFlag(t *testing.T) {
	_, _, _, err := ParseArgs(
		"testdata/TestParseArgs/input-args.acorn",
		"testdata/TestParseArgs/input.acorn",
		[]string{"--foo4", "bar"})

	autogold.Expect("unknown flag: --foo4").Equal(t, err.Error())
}

func TestHelp(t *testing.T) {
	var file value.FuncSchema

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
