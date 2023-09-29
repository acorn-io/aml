package aml

import (
	cuelang "cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/acorn-io/aml/legacy/pkg/cue"
	"github.com/acorn-io/aml/legacy/pkg/replace"
)

func Interpolate(data any, s string) (string, error) {
	ctx := cuecontext.New()
	model := ctx.Encode(data)
	if model.Err() != nil {
		return "", cue.WrapErr(model.Err())
	}

	return replace.Replace(s, "@{", "}", func(s string) (string, bool, error) {
		path := cuelang.ParsePath(s)
		if err := cue.CheckErr(path); err != nil {
			return "", true, err
		}

		v := model.LookupPath(path)
		if err := cue.CheckErr(v); err != nil {
			return "", true, err
		}
		s, err := v.String()
		if err == nil {
			return s, true, nil
		}
		data, err := v.MarshalJSON()
		if err != nil {
			return "", false, err
		}
		return string(data), true, nil
	})
}
