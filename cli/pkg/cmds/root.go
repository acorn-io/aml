package cmds

import (
	"encoding/json"
	"os"

	"github.com/acorn-io/cmd"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	return cmd.Command(&AML{}, cobra.Command{
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		SilenceUsage: true,
	})
}

type AML struct {
}

func (a *AML) Customize(cmd *cobra.Command) {
	cmd.AddCommand(NewEval(a))
	cmd.AddCommand(NewFmt(a))
}

func (a *AML) Run(cmd *cobra.Command, args []string) error {
	return cmd.Usage()
}

func (a *AML) Output(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
