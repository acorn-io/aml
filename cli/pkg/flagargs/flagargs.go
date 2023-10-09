package flagargs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/acorn-io/aml"
	"github.com/acorn-io/aml/cli/pkg/amlreadhelper"
	"github.com/acorn-io/aml/pkg/schema"
	"github.com/acorn-io/aml/pkg/value"
	"github.com/spf13/pflag"
)

type Flags struct {
	FlagSet    *pflag.FlagSet
	fieldFlags map[string]fieldFlag
	profile    *[]string
	Usage      func()
	argsFile   string
}

type fieldFlag struct {
	Field       schema.Field
	String      *string
	StringSlice *[]string
	Bool        *bool
}

func ParseArgs(argsFile, acornFile string, args []string) (map[string]any, []string, error) {
	f, err := amlreadhelper.Open(acornFile)
	if err != nil {
		return nil, nil, err
	}

	var file schema.File
	if err := aml.NewDecoder(f).Decode(&file); err != nil {
		return nil, nil, err
	}

	flags := New(argsFile, filepath.Base(acornFile), file.ProfileNames, file.Args.Fields)
	return flags.Parse(args)
}

func New(argsFile, filename string, profiles schema.Names, args []schema.Field) *Flags {
	var (
		flagSet    = pflag.NewFlagSet(filename, pflag.ContinueOnError)
		fieldFlags = map[string]fieldFlag{}
		profile    *[]string
	)

	desc := strings.Builder{}
	desc.WriteString("Available profiles (")
	startLen := desc.Len()
	for _, name := range profiles {
		val := name.Name
		if name.Description != "" {
			val += ": " + name.Description
		}
		if desc.Len() > startLen {
			desc.WriteString(", ")
		}
		desc.WriteString(val)
	}
	desc.WriteString(")")
	profile = flagSet.StringSlice("profile", nil, desc.String())

	for _, field := range args {
		flag := fieldFlag{
			Field: field,
		}
		if profile != nil && field.Name == "profile" {
			continue
		}
		if field.Type.Kind == schema.BoolKind {
			flag.Bool = flagSet.Bool(field.Name, false, field.Description)
		} else if field.Type.Kind == schema.ArrayKind {
			flag.StringSlice = flagSet.StringSlice(field.Name, nil, field.Description)
		} else {
			flag.String = flagSet.String(field.Name, "", field.Description)
		}
		fieldFlags[field.Name] = flag
	}

	return &Flags{
		fieldFlags: fieldFlags,
		profile:    profile,
		FlagSet:    flagSet,
		argsFile:   argsFile,
	}
}

func parseValue(v string, isNumber bool) (any, error) {
	if !strings.HasPrefix(v, "@") {
		if isNumber {
			return value.Number(v), nil
		}
		return v, nil
	}

	v = v[1:]
	data := map[string]any{}
	if strings.HasPrefix(v, "{") {
		if err := aml.Unmarshal([]byte(v), &data); err != nil {
			return nil, err
		}
		return data, nil
	}

	return data, amlreadhelper.UnmarshalFile(v, &data)
}

func (f *Flags) readArgsFile() (map[string]any, error) {
	result := map[string]any{}

	if f.argsFile == "" {
		return result, nil
	}

	input, err := os.Open(f.argsFile)
	if os.IsNotExist(err) {
		return result, nil
	}

	if err := aml.NewDecoder(input).Decode(result); err != nil {
		return nil, err
	}

	return result, nil
}

func (f *Flags) Parse(args []string) (map[string]any, []string, error) {
	result, err := f.readArgsFile()
	if err != nil {
		return nil, nil, err
	}

	if f.Usage != nil {
		f.FlagSet.Usage = func() {
			f.Usage()
			f.FlagSet.PrintDefaults()
		}
	}

	if err := f.FlagSet.Parse(args); err != nil {
		return nil, nil, err
	}

	if args := f.FlagSet.Args(); len(args) > 0 {
		return nil, nil, fmt.Errorf("accepts no args, received %d %v", len(args), args)
	}

	for name, field := range f.fieldFlags {
		flag := f.FlagSet.Lookup(name)

		switch {
		case !flag.Changed:
		case field.Bool != nil:
			result[name] = *field.Bool
		case field.StringSlice != nil:
			vals := []any{}
			for _, str := range *field.StringSlice {
				isNum := len(field.Field.Type.Array.Types) > 0 &&
					field.Field.Type.Array.Types[0].Kind == schema.NumberKind
				val, err := parseValue(str, isNum)
				if err != nil {
					return nil, nil, err
				}
				vals = append(vals, val)
			}
			result[name] = vals
		default:
			result[name], err = parseValue(*field.String, field.Field.Type.Kind == schema.NumberKind)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return result, *f.profile, nil
}

func (f *Flags) flagChanged(name string) bool {
	return f.FlagSet.Lookup(name).Changed
}
