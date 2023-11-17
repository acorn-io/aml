package flagargs

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/acorn-io/aml"
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
	Field       value.ObjectSchemaField
	String      *string
	StringSlice *[]string
	Bool        *bool
}

func ParseArgs(argsFile, acornFile string, args []string) (map[string]any, []string, []string, error) {
	f, err := aml.Open(acornFile)
	if err != nil {
		return nil, nil, nil, err
	}

	var file value.FuncSchema
	if err := aml.NewDecoder(f, aml.DecoderOption{SourceName: acornFile}).Decode(&file); err != nil {
		return nil, nil, nil, err
	}

	flags := New(argsFile, filepath.Base(acornFile), file.ProfileNames, file.Args)
	argsData, profiles, err := flags.Parse(args)
	if err != nil {
		return nil, nil, nil, err
	}
	return argsData, profiles, flags.FlagSet.Args(), nil
}

func New(argsFile, filename string, profiles value.Names, args []value.ObjectSchemaField) *Flags {
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
		if profile != nil && field.Key == "profile" {
			continue
		}
		if field.Schema.TargetKind() == value.BoolKind {
			flag.Bool = flagSet.Bool(field.Key, false, field.Description)
		} else if field.Schema.TargetKind() == value.ArrayKind {
			flag.StringSlice = flagSet.StringSlice(field.Key, nil, field.Description)
		} else {
			flag.String = flagSet.String(field.Key, "", field.Description)
		}
		fieldFlags[field.Key] = flag
	}

	return &Flags{
		fieldFlags: fieldFlags,
		profile:    profile,
		FlagSet:    flagSet,
		argsFile:   argsFile,
	}
}

func parseValue(v string, kind value.Kind) (any, error) {
	if !strings.HasPrefix(v, "@") {
		if kind == value.NumberKind {
			return value.Number(v), nil
		}
		return v, nil
	}

	v = v[1:]
	if kind == value.StringKind {
		data, err := os.ReadFile(v)
		return string(data), err
	}

	data := map[string]any{}
	if strings.HasPrefix(v, "{") {
		if err := aml.Unmarshal([]byte(v), &data); err != nil {
			return nil, err
		}
		return data, nil
	}

	return data, aml.UnmarshalFile(v, &data)
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

	if err := aml.NewDecoder(input).Decode(&result); err != nil {
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

	for name, field := range f.fieldFlags {
		flag := f.FlagSet.Lookup(name)

		switch {
		case !flag.Changed:
		case field.Bool != nil:
			result[name] = *field.Bool
		case field.StringSlice != nil:
			vals := []any{}
			for _, str := range *field.StringSlice {
				kind := value.StringKind
				if len(field.Field.Schema.ValidArrayItems()) > 0 {
					kind = field.Field.Schema.ValidArrayItems()[0].TargetKind()
				}
				val, err := parseValue(str, kind)
				if err != nil {
					return nil, nil, err
				}
				vals = append(vals, val)
			}
			result[name] = vals
		default:
			result[name], err = parseValue(*field.String, field.Field.Schema.TargetKind())
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
