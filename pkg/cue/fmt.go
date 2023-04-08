package cue

import (
	"bytes"
	"os"

	"cuelang.org/go/cue/format"
)

func FmtBytes(data []byte) ([]byte, error) {
	return format.Source(data, format.Simplify(), format.TabIndent(true))
}

func Fmt(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	newData, err := format.Source(data, format.Simplify(), format.TabIndent(true))
	if err != nil {
		return err
	}

	if !bytes.Equal(data, newData) {
		return os.WriteFile(file, newData, 0600)
	}

	return nil
}
