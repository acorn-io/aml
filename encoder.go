package aml

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/acorn-io/aml/pkg/format"
	"github.com/acorn-io/aml/pkg/parser"
	"github.com/acorn-io/aml/pkg/value"
)

type EncoderOption struct {
}

func (o EncoderOption) Complete() EncoderOption {
	return EncoderOption{}
}

type EncoderOptions []EncoderOption

func (o EncoderOptions) Merge() (result EncoderOption) {
	return
}

type Encoder struct {
	opts   EncoderOption
	output io.Writer
}

func NewEncoder(output io.Writer, opts ...EncoderOption) *Encoder {
	return &Encoder{
		opts:   EncoderOptions(opts).Merge().Complete(),
		output: output,
	}
}

func (d *Encoder) Encode(out any) error {
	if v, ok := out.(value.Value); ok {
		nv, ok, err := value.NativeValue(v)
		if err != nil {
			return err
		}
		if ok {
			out = nv
		} else {
			out = nil
		}
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}

	if len(data) > 2 && data[0] == '{' && data[len(data)-1] == '}' {
		data = data[1 : len(data)-1]
	}

	parsed, err := parser.ParseFile("", bytes.NewReader(data))
	if err != nil {
		return err
	}

	data, err = format.Node(parsed)
	if err != nil {
		return err
	}

	_, err = d.output.Write(data)
	return err
}

func Marshal(v any, opts ...EncoderOption) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := NewEncoder(buf, opts...).Encode(v)
	return buf.Bytes(), err
}
