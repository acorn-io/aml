package aml

import (
	"fmt"

	cuelang "cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/acorn-io/aml/pkg/cue"
	"github.com/acorn-io/aml/pkg/replace"
)

func Interpolate(data any, s string) (string, error) {
	ctx := cuecontext.New()
	model := ctx.Encode(data)
	if model.Err() != nil {
		return "", cue.WrapErr(model.Err())
	}

	return replace.Replace(s, "@{", "}", func(s string) (string, error) {
		path := cuelang.ParsePath(s)
		if err := cue.CheckErr(path); err != nil {
			return "", err
		}

		v := model.LookupPath(path)
		if err := cue.CheckErr(v); err != nil {
			return "", err
		}
		s, err := v.String()
		if err == nil {
			return s, nil
		}
		return fmt.Sprint(v), nil
	})
}
