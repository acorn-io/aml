package aml

import (
	"io"

	"github.com/acorn-io/aml/pkg/definition"
	"github.com/acorn-io/aml/pkg/loader"
)

type Options struct {
	Args     map[string]any
	Profiles []string
}

func (d Options) ApplyTo(opts *Options) {
	if len(d.Args) > 0 {
		if opts.Args == nil {
			opts.Args = map[string]any{}
		}
		for k, v := range d.Args {
			opts.Args[k] = v
		}
	}

	opts.Profiles = append(opts.Profiles, d.Profiles...)
}

type Option interface {
	ApplyTo(d *Options)
}

type Decoder struct {
	opts  *Options
	input io.Reader
}

func NewDecoder(input io.Reader, options ...Option) *Decoder {
	opts := &Options{}
	for _, opt := range options {
		opt.ApplyTo(opts)
	}
	return &Decoder{
		opts:  opts,
		input: input,
	}
}

func (d *Decoder) Args() (*definition.ParamSpec, error) {
	files, err := loader.ToFiles(d.input)
	if err != nil {
		return nil, err
	}
	def, err := definition.NewDefinition(files)
	if err != nil {
		return nil, err
	}
	return def.Args()
}

func (d *Decoder) ComputedArgs() (map[string]any, error) {
	files, err := loader.ToFiles(d.input)
	if err != nil {
		return nil, err
	}
	def, err := definition.NewDefinition(files)
	if err != nil {
		return nil, err
	}

	_, computed, err := def.WithArgs(d.opts.Args, d.opts.Profiles)
	return computed, err
}

func (d *Decoder) Decode(v any) error {
	files, err := loader.ToFiles(d.input)
	if err != nil {
		return err
	}
	def, err := definition.NewDefinition(files)
	if err != nil {
		return err
	}

	def, _, err = def.WithArgs(d.opts.Args, d.opts.Profiles)
	if err != nil {
		return err
	}

	return def.Decode(v)
}
