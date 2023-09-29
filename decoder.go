package aml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/acorn-io/aml/pkg/ast"
	"github.com/acorn-io/aml/pkg/eval"
	"github.com/acorn-io/aml/pkg/parser"
	"github.com/acorn-io/aml/pkg/schema"
	"github.com/acorn-io/aml/pkg/value"
)

type DecoderOption struct {
	PositionalArgs   []any
	Args             map[string]any
	Profiles         []string
	SourceName       string
	SchemaSourceName string
	Schema           io.Reader
	Context          context.Context
}

func (o DecoderOption) Complete() DecoderOption {
	if o.SourceName == "" {
		o.SourceName = "<inline>"
	}
	if o.SchemaSourceName == "" {
		o.SchemaSourceName = "<inline>"
	}
	if o.Context == nil {
		o.Context = context.Background()
	}
	return o
}

type DecoderOptions []DecoderOption

func (o DecoderOptions) Merge() (result DecoderOption) {
	for _, opt := range o {
		result.PositionalArgs = append(result.PositionalArgs, opt.PositionalArgs...)
		result.Profiles = append(result.Profiles, opt.Profiles...)
		if opt.SourceName != "" {
			result.SourceName = opt.SourceName
		}
		if opt.Context != nil {
			result.Context = opt.Context
		}
		if len(opt.Args) > 0 && result.Args == nil {
			result.Args = map[string]any{}
		}
		for k, v := range opt.Args {
			result.Args[k] = v
		}
		if opt.Schema != nil {
			result.Schema = opt.Schema
		}
	}
	return
}

type Decoder struct {
	opts  DecoderOption
	input io.Reader
}

func NewDecoder(input io.Reader, opts ...DecoderOption) *Decoder {
	return &Decoder{
		opts:  DecoderOptions(opts).Merge().Complete(),
		input: input,
	}
}

func (d *Decoder) processSchema(data value.Value) (value.Value, error) {
	f := &eval.File{}

	err := NewDecoder(d.opts.Schema, DecoderOption{
		Context:    d.opts.Context,
		SourceName: d.opts.SchemaSourceName,
	}).Decode(f)
	if err != nil {
		return nil, err
	}

	schema, ok, err := eval.EvalSchema(d.opts.Context, f)
	if err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("invalid schema %s yield no schema value", d.opts.SchemaSourceName)
	}

	return value.Merge(schema, data)
}

func (d *Decoder) Decode(out any) error {
	parsed, err := parser.ParseFile(d.opts.SourceName, d.input)
	if err != nil {
		return err
	}

	switch n := out.(type) {
	case *ast.File:
		*n = *parsed
		return nil
	}

	file, err := eval.Build(parsed, eval.BuildOption{
		PositionalArgs: d.opts.PositionalArgs,
		Args:           d.opts.Args,
		Profiles:       d.opts.Profiles,
	})
	if err != nil {
		return err
	}

	switch n := out.(type) {
	case *eval.File:
		*n = *file
		return nil
	}

	switch n := out.(type) {
	case *schema.File:
		fileSchema, err := file.DescribeFile()
		if err != nil {
			return err
		}
		*n = *fileSchema
		return nil
	case *schema.Summary:
		val, ok, err := eval.EvalSchema(d.opts.Context, file)
		if err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("source <%s> did not produce a value", d.opts.SourceName)
		}
		objSchema, err := value.DescribeObject(value.SchemaContext{}, val)
		if err != nil {
			return err
		}

		*n = schema.Summarize(*objSchema)
		return nil
	}

	val, ok, err := eval.EvalExpr(d.opts.Context, file)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("source <%s> did not produce a value", d.opts.SourceName)
	}

	if d.opts.Schema != nil {
		val, err = d.processSchema(val)
		if err != nil {
			return err
		}
	}

	switch n := out.(type) {
	case *value.Value:
		*n = val
		return nil
	}

	nv, ok, err := value.NativeValue(val)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("value kind %s from source %s did not produce a native value", val.Kind(), d.opts.SourceName)
	}

	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(nv); err != nil {
		return err
	}

	return json.NewDecoder(buf).Decode(out)
}

func Unmarshal(data []byte, v any, opts ...DecoderOption) error {
	return NewDecoder(bytes.NewReader(data), opts...).Decode(v)
}
