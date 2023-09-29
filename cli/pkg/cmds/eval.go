package cmds

import (
	"errors"
	"io"
	"os"

	"github.com/acorn-io/aml"
	"github.com/acorn-io/aml/cli/pkg/flagargs"
	"github.com/acorn-io/aml/pkg/schema"
	"github.com/acorn-io/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Eval struct {
	aml *AML

	ArgsFile    string `usage:"Default arguments to pass" default:".args.acorn"`
	PrintArgs   bool   `usage:"Evaluate the file and print args description"`
	PrintSchema bool   `usage:"Evaluate the file as schema and print schema description"`
	SchemaFile  string `usage:"Validate result against schema file"`
}

func NewEval(aml *AML) *cobra.Command {
	return cmd.Command(&Eval{aml: aml}, cobra.Command{
		Use:           "eval [flags] FILE",
		Short:         "Evaluate a file and output the result",
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: true,
	})
}

func (e *Eval) Customize(cmd *cobra.Command) {
	cmd.Flags().SetInterspersed(false)
}

func (e *Eval) Run(cmd *cobra.Command, args []string) error {
	filename := args[0]
	args = args[1:]

	argsData, profiles, err := flagargs.ParseArgs(e.ArgsFile, filename, args)
	if errors.Is(err, pflag.ErrHelp) {
		return nil
	} else if err != nil {
		return err
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var (
		out         any = &map[string]any{}
		schemaInput io.Reader
	)
	if e.PrintArgs {
		out = &schema.File{}
	} else if e.PrintSchema {
		out = &schema.Summary{}
	}

	if e.SchemaFile != "" {
		schemaInput, err = os.Open(e.SchemaFile)
		if err != nil {
			return err
		}
	}

	err = aml.Unmarshal(data, out, aml.DecoderOption{
		Schema:           schemaInput,
		SchemaSourceName: e.SchemaFile,
		SourceName:       filename,
		Args:             argsData,
		Profiles:         profiles,
		Context:          cmd.Context(),
	})
	if err != nil {
		return err
	}

	return e.aml.Output(out)
}
