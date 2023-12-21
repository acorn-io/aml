package aml

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/acorn-io/aml/pkg/ast"
	"github.com/acorn-io/aml/pkg/eval"
	"github.com/acorn-io/aml/pkg/parser"
	"github.com/acorn-io/aml/pkg/value"
)

var ErrNoOutput = errors.New("value did not produce any output")

type DecoderOption struct {
	PositionalArgs   []any
	Args             map[string]any
	Profiles         []string
	SourceName       string
	SchemaSourceName string
	Schema           io.Reader
	SchemaValue      value.Value
	Globals          map[string]any
	GlobalsLookup    eval.ScopeFunc
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
		if opt.SchemaSourceName != "" {
			result.SchemaSourceName = opt.SchemaSourceName
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
		if opt.SchemaValue != nil {
			result.SchemaValue = opt.SchemaValue
		}
		if len(opt.Globals) > 0 && result.Globals == nil {
			result.Globals = map[string]any{}
		}
		for k, v := range opt.Globals {
			result.Globals[k] = v
		}
		if opt.GlobalsLookup != nil {
			result.GlobalsLookup = opt.GlobalsLookup
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

func (d *Decoder) processSchema(ctx context.Context, data value.Value) (value.Value, error) {
	_, _, err := value.NativeValue(data)
	if err != nil {
		return nil, err
	}

	if d.opts.SchemaValue != nil {
		return value.Validate(ctx, d.opts.SchemaValue, data)
	}

	f := &eval.File{}

	err = NewDecoder(d.opts.Schema, DecoderOption{
		Context:    d.opts.Context,
		SourceName: d.opts.SchemaSourceName,
	}).Decode(f)
	if err != nil {
		return nil, err
	}

	schema, ok, err := eval.EvalSchema(ctx, f)
	if err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("invalid schema %s yield no schema value", d.opts.SchemaSourceName)
	}

	return value.Validate(ctx, schema, data)
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

	ctx := eval.WithScope(d.opts.Context, eval.Builtin)

	switch n := out.(type) {
	case *value.FuncSchema:
		fileSchema, err := file.Describe(ctx)
		if err != nil {
			return err
		}
		*n = *fileSchema
		return nil
	case *value.Schema:
		val, ok, err := eval.EvalSchema(ctx, file)
		if err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("source <%s>: %w", d.opts.SourceName, ErrNoOutput)
		}
		*n = val
		return nil
	case *value.Summary:
		val, ok, err := eval.EvalSchema(ctx, file)
		if err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("source <%s>: %w", d.opts.SourceName, ErrNoOutput)
		}
		*n = *value.Summarize(val.(*value.TypeSchema))
		return nil
	}

	val, ok, err := eval.EvalExpr(ctx, file, eval.EvalOption{
		Globals:       d.opts.Globals,
		GlobalsLookup: d.opts.GlobalsLookup,
	})
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("source <%s>: %w", d.opts.SourceName, ErrNoOutput)
	}

	if d.opts.Schema != nil || d.opts.SchemaValue != nil {
		val, err = d.processSchema(ctx, val)
		if err != nil {
			return err
		}
	}

	switch n := out.(type) {
	case *value.Value:
		*n = val
		return nil
	}

	if val.Kind() == value.FuncKind {
		ret, ok, err := value.Call(d.opts.Context, val)
		if err != nil {
			return err
		}
		if ok {
			val = ret
		}
	}

	nv, ok, err := value.NativeValue(val)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("value kind %s from source <%s> did not produce a native value: %w", val.Kind(), d.opts.SourceName, ErrNoOutput)
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

func NewValueReader(value value.Value) io.Reader {
	data, err := Marshal(value)
	if err != nil {
		return errReader{err: err}
	}
	return bytes.NewReader(data)
}

type errReader struct {
	err error
}

func (e errReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
