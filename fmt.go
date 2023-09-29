package aml

import (
	"bytes"

	"github.com/acorn-io/aml/pkg/format"
)

func Format(data []byte) ([]byte, error) {
	return format.Format(bytes.NewBuffer(data))
}
