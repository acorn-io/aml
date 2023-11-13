package cmds

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/acorn-io/aml"
	"github.com/acorn-io/cmd"
	"github.com/spf13/cobra"
)

type Fmt struct {
	aml *AML
}

func NewFmt(aml *AML) *cobra.Command {
	return cmd.Command(&Fmt{aml: aml}, cobra.Command{
		Use:           "fmt [flags] [FILE]",
		Short:         "Formats a single file, writing the output to the source file if changed",
		SilenceErrors: true,
	})
}

func (e *Fmt) Run(cmd *cobra.Command, args []string) error {
	var errs []error
	for _, arg := range args {
		var (
			data []byte
			err  error
		)
		if arg == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(arg)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("reading %s: %w", arg, err))
			continue
		}

		newData, err := aml.Format(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("formatting %s: %w", arg, err))
			continue
		}

		if arg == "-" {
			_, err := os.Stdout.Write(newData)
			if err != nil {
				return err
			}
		} else if !bytes.Equal(data, newData) {
			err := os.WriteFile(arg, newData, 0644)
			if err != nil {
				errs = append(errs, fmt.Errorf("writing file %s: %w", args, err))
			}
			continue
		}
	}

	return errors.Join(errs...)
}
