package definition

import (
	"encoding/json"
	"fmt"
	"strings"

	cuelang "cuelang.org/go/cue"
	cue_mod "github.com/acorn-io/aml/cue.mod"
	"github.com/acorn-io/aml/pkg/amlparser"
	"github.com/acorn-io/aml/pkg/cue"
	"github.com/acorn-io/aml/schema"
)

const (
	AcornCueFile = "Acornfile"
	Schema       = "github.com/acorn-io/aml/schema/v1"
	AppType      = "#App"
)

var (
	TopFields = []string{
		"labels",
		"annotations",
		"name",
		"description",
		"readme",
		"info",
		"icon",
		"containers",
		"jobs",
		"acorns",
		"secrets",
		"volumes",
		"images",
		"routers",
		"services",
	}
	Defaults = []byte(`

args: {
	dev: bool | *false
	profiles: [...string]
	autoUpgrade: bool | *false
}
profiles: {
	devMode: dev: bool | *true
	autoUpgrade: autoUpgrade: bool | *true
}
`)
)

type Definition struct {
	ctx  *cue.Context
	data bool
}

func NewAcornfile(data []byte) []cue.File {
	return []cue.File{{
		Filename:    AcornCueFile + ".cue",
		DisplayName: AcornCueFile,
		Data:        append(data, Defaults...),
		Parser:      amlparser.ParseFile,
	}}
}

func NewData(files []cue.File) (*Definition, error) {
	ctx := cue.NewContext()
	ctx = ctx.WithFiles(files...)
	_, err := ctx.Value()
	if err != nil {
		return nil, err
	}
	return &Definition{
		ctx:  ctx,
		data: true,
	}, nil
}

func NewDefinition(files []cue.File) (*Definition, error) {
	ctx := cue.NewContext().
		WithNestedFS("schema", schema.Files).
		WithNestedFS("cue.mod", cue_mod.Files)
	ctx = ctx.WithFiles(files...)
	ctx = ctx.WithSchema(Schema, AppType)
	_, err := ctx.Value()
	if err != nil {
		return nil, err
	}
	return &Definition{
		ctx: ctx,
	}, nil
}

func (a *Definition) getArgsForProfile(args map[string]any, profiles []string) (map[string]any, error) {
	val, err := a.ctx.Value()
	if err != nil {
		return nil, err
	}
	var profileList []any
	for _, profile := range profiles {
		optional := false
		if strings.HasSuffix(profile, "?") {
			optional = true
			profile = profile[:len(profile)-1]
		}
		path := cuelang.ParsePath(fmt.Sprintf("profiles[\"%s\"]", profile))
		pValue := val.LookupPath(path)
		if !pValue.Exists() {
			if !optional {
				return nil, fmt.Errorf("failed to find profile %s", profile)
			}
			continue
		}

		if args == nil {
			args = map[string]any{}
		}

		profileList = append(profileList, profile)

		inValue, err := a.ctx.Encode(args)
		if err != nil {
			return nil, err
		}

		newArgs := map[string]any{}
		err = pValue.Unify(*inValue).Decode(&newArgs)
		if err != nil {
			return nil, cue.WrapErr(err)
		}
		args = newArgs
	}

	existingProfiles, _ := args["profiles"].([]string)
	if len(existingProfiles) == 0 && len(profileList) > 0 {
		if args == nil {
			args = map[string]any{}
		}
		args["profiles"] = profileList
	}

	return args, nil
}

func (a *Definition) WithArgs(args map[string]any, profiles []string) (*Definition, map[string]any, error) {
	args, err := a.getArgsForProfile(args, profiles)
	if err != nil {
		return nil, nil, err
	}
	if len(args) == 0 {
		return a, args, nil
	}
	data, err := json.Marshal(map[string]any{
		"args": args,
	})
	if err != nil {
		return nil, nil, err
	}
	return &Definition{
		ctx: a.ctx.WithFile("args.cue", data),
	}, args, nil
}

func (a *Definition) Decode(out interface{}) error {
	app, err := a.ctx.Value()
	if err != nil {
		return err
	}

	if a.data {
		return a.ctx.Decode(app, out)
	}

	objs := map[string]any{}
	for _, key := range TopFields {
		v := app.LookupPath(cuelang.ParsePath(key))
		if v.Exists() {
			objs[key] = v
		}
	}

	newApp, err := a.ctx.Encode(objs)
	if err != nil {
		return err
	}

	return a.ctx.Decode(newApp, out)
}
