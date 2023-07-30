package definition

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"github.com/acorn-io/aml/pkg/amlparser"
)

var (
	implicitArgs = map[string]struct{}{
		"autoUpgrade": {},
		"dev":         {},
		"profiles":    {},
	}
	implicitProfiles = map[string]struct{}{
		"devMode":     {},
		"autoUpgrade": {},
	}
)

type ParamSpec struct {
	Params   []Param   `json:"params,omitempty"`
	Profiles []Profile `json:"profiles,omitempty"`
}

type Param struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty" wrangler:"options=string|int|float|bool|object|array"`
	Schema      string `json:"schema,omitempty"`
}

type Profile struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

func (a *Definition) Args() (*ParamSpec, error) {
	return a.addProfiles(a.args("args", implicitArgs))
}

func (a *Definition) addProfiles(paramSpec *ParamSpec, err error) (*ParamSpec, error) {
	if err != nil {
		return nil, err
	}

	profiles, err := a.args("profiles", implicitProfiles)
	if err != nil {
		return nil, err
	}

	for _, profile := range profiles.Params {
		paramSpec.Profiles = append(paramSpec.Profiles, Profile{
			Name:        profile.Name,
			Description: profile.Description,
		})
	}

	return paramSpec, nil
}

func (a *Definition) args(section string, ignore map[string]struct{}) (*ParamSpec, error) {
	app, err := a.ctx.ValueNoSchema()
	if err != nil {
		return nil, err
	}

	v := app.LookupPath(cue.ParsePath(section))
	sv, err := v.Struct()
	if err != nil {
		return nil, err
	}

	// I have no clue what I'm doing here, just poked around
	// until something worked

	result := &ParamSpec{}
	node := v.Syntax(cue.Docs(true))
	s, ok := node.(*ast.StructLit)
	if !ok {
		return result, nil
	}

	for i, o := range s.Elts {
		f := o.(*ast.Field)
		if _, ok := ignore[fmt.Sprint(f.Label)]; ok {
			continue
		}
		com := strings.Builder{}
		for _, c := range ast.Comments(o) {
			for _, d := range c.List {
				s := strings.TrimSpace(d.Text)
				s = strings.TrimPrefix(s, "//")
				s = strings.TrimSpace(s)
				com.WriteString(s)
				com.WriteString("\n")
			}
		}
		result.Params = append(result.Params, Param{
			Name:        fmt.Sprint(f.Label),
			Description: strings.TrimSpace(com.String()),
			Schema:      fmt.Sprint(sv.Field(i).Value),
			Type:        getType(sv.Field(i).Value, f.Value),
		})
	}

	return result, nil
}

func getType(v cue.Value, expr ast.Expr) string {
	if _, err := v.String(); err == nil {
		if amlparser.AllLitStrings(expr, true) {
			return "enum"
		}
		return "string"
	}
	if _, err := v.Bool(); err == nil {
		return "bool"
	}
	if _, err := v.Int(nil); err == nil {
		return "int"
	}
	if _, err := v.Float64(); err == nil {
		return "float"
	}
	if _, err := v.List(); err == nil {
		return "array"
	}
	return "object"
}
