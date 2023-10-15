package eval

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/acorn-io/aml/pkg/errors"
	"github.com/acorn-io/aml/pkg/schema"
	"github.com/acorn-io/aml/pkg/value"
)

type FunctionDefinition struct {
	Comments         Comments
	Pos              Position
	Body             *Struct
	ReturnBody       bool
	AllowUnknownArgs bool
	AssignRoot       bool
	SchemaScope      bool
}

func (f *FunctionDefinition) ToValue(scope Scope) (value.Value, bool, error) {
	argsFields, bodyFields := f.splitFields()
	argNames, argsSchema, err := f.toSchema(scope, argsFields, "args", f.AllowUnknownArgs)
	if err != nil {
		return nil, false, err
	}
	profileNames, profileSchema, err := f.toSchema(scope, argsFields, "profiles", true)
	if err != nil {
		return nil, false, err
	}
	body := &Struct{
		Position: f.Pos,
		Fields:   bodyFields,
	}
	return &Function{
		Pos:            f.Pos,
		Scope:          scope,
		Body:           body,
		ArgsSchema:     argsSchema,
		ArgNames:       argNames,
		ProfileNames:   profileNames,
		ProfilesSchema: profileSchema,
		ReturnBody:     f.ReturnBody,
		AssignRoot:     f.AssignRoot,
		CallScope:      !f.SchemaScope,
	}, true, nil
}

func (f *FunctionDefinition) toSchema(scope Scope, argDefs []Field, fieldName string, allowNewFields bool) (Names, value.Value, error) {
	s := Schema{
		AllowNewFields: allowNewFields,
		Struct: &Struct{
			Position: f.Pos,
			Fields:   argDefs,
		},
	}
	v, _, err := s.ToValue(scope)
	if err != nil {
		return nil, nil, err
	}

	args, ok, err := value.Lookup(v, value.NewValue(fieldName))
	if err != nil {
		return nil, nil, err
	} else if !ok {
		return nil, value.NewClosedObject(), nil
	}

	obj, err := value.DescribeObject(value.SchemaContext{}, args)
	if err != nil {
		// for various reasons during partial evaluation this call could fail, in that situation
		// we don't care because we are just looking for the descriptions
		obj = &schema.Object{}
	}

	var names Names
	keys, err := value.Keys(args)
	for _, key := range keys {
		name := Name{
			Name: key,
		}
		for _, field := range obj.Fields {
			if field.Name == key {
				name.Description = field.Description
				break
			}
		}
		names = append(names, name)
	}

	return names, args, err
}

func (f *FunctionDefinition) splitFields() (argFields []Field, bodyFields []Field) {
	for _, field := range f.Body.Fields {
		arg, ok := field.(IsArgumentDefinition)
		if ok && arg.IsArgumentDefinition() {
			argFields = append(argFields, field)
			continue
		}
		bodyFields = append(bodyFields, field)
	}
	return
}

type IsArgumentDefinition interface {
	IsArgumentDefinition() bool
}

type Function struct {
	Pos            Position
	Scope          Scope
	Body           Expression
	ArgsSchema     value.Value
	ArgNames       Names
	ProfilesSchema value.Value
	ProfileNames   Names
	ReturnBody     bool
	AssignRoot     bool
	UnscopedArgs   bool
	CallScope      bool

	depth atomic.Int32
}

type Names []Name

type Name struct {
	Name        string
	Description string
}

func (n Names) Describe() (result schema.Names) {
	for _, name := range n {
		result = append(result, schema.Name(name))
	}
	return
}

func (c *Function) DescribeFieldType(ctx value.SchemaContext) (result schema.FieldType, _ error) {
	argsSchema, err := value.DescribeObject(ctx, c.ArgsSchema)
	if err != nil {
		return result, err
	}
	return schema.FieldType{
		Func: &schema.Func{
			Args: *argsSchema,
		},
		Kind: schema.FuncKind,
	}, nil
}

func (c *Function) Kind() value.Kind {
	return value.FuncKind
}

func (c *Function) getProfiles(v value.Value) (profiles []value.Value, profileStringNames []string, _ bool, _ error) {
	v, ok, err := value.Lookup(v, value.NewValue("profiles"))
	if err != nil || !ok {
		return nil, nil, ok, err
	} else if v.Kind() == value.UndefinedKind {
		return []value.Value{v}, nil, true, nil
	}

	if v.Kind() != value.ArrayKind {
		return nil, nil, false, fmt.Errorf("profiles type should be an array")
	}

	profileNames, err := value.ToValueArray(v)
	if err != nil {
		return nil, nil, false, err
	}

	for _, profileName := range profileNames {
		profileNameString, err := value.ToString(profileName)
		if err != nil {
			return nil, nil, false, err
		}
		optional := strings.HasSuffix(profileNameString, "?")
		if optional {
			profileNameString = strings.TrimSuffix(profileNameString, "?")
		}
		profile, ok, err := value.Lookup(c.ProfilesSchema, value.NewValue(profileNameString))
		if err != nil {
			return nil, nil, false, err
		} else if !ok {
			if optional {
				continue
			}
			return nil, nil, false, fmt.Errorf("failed to find profile %s", profileName)
		} else {
			profileStringNames = append(profileStringNames, profileNameString)
			profiles = append(profiles, profile)
		}
	}

	return profiles, profileStringNames, true, nil
}

func (c *Function) callArgumentToValue(args []value.CallArgument) (value.Value, error) {
	var (
		argValues      []value.Value
		profiles       []value.Value
		profilesActive []string
	)

	for i, arg := range args {
		if arg.Self {
			continue
		}
		if arg.Positional {
			if i >= len(c.ArgNames) {
				return nil, fmt.Errorf("invalid arg index %d, args len %d", i, len(c.ArgNames))
			}
			argValues = append(argValues, value.NewObject(map[string]any{
				c.ArgNames[i].Name: arg.Value,
			}))
		} else if arg.Value.Kind() != value.ObjectKind {
			return nil, fmt.Errorf("invalid argument kind %s (index %d)", arg.Value.Kind(), i)
		} else if profile, profileNames, profilesSet, err := c.getProfiles(arg.Value); err != nil {
			return nil, err
		} else if profilesSet {
			profilesActive = append(profilesActive, profileNames...)
			profiles = append(profiles, profile...)
		} else {
			argValues = append(argValues, arg.Value)
		}
	}

	argValue, err := value.Merge(argValues...)
	if err != nil {
		return nil, err
	}

	if argValue == nil {
		argValue = value.NewObject(nil)
	}

	for i := len(profiles) - 1; i >= 0; i-- {
		argValue, err = value.Merge(profiles[i], argValue)
		if err != nil {
			return nil, err
		}
	}

	validated, err := value.Merge(c.ArgsSchema, argValue)
	if err != nil {
		return validated, errors.NewErrEval(value.Position(c.Pos), &ErrInvalidArgument{
			Err: err,
		})
	}

	if c.UnscopedArgs {
		return validated, nil
	}

	return value.Merge(validated, value.NewObject(map[string]any{
		"profiles": profilesActive,
	}))
}

type ErrInvalidArgument struct {
	Err error
}

func (e *ErrInvalidArgument) Error() string {
	return fmt.Sprintf("invalid arguments: %v", e.Err)
}

const MaxCallDepth = 100

func (c *Function) Call(ctx context.Context, args []value.CallArgument) (value.Value, bool, error) {
	defer c.depth.Add(-1)
	if depth := c.depth.Add(1); depth > MaxCallDepth {
		return nil, false, fmt.Errorf("exceeded max call depth %d > %d", depth, MaxCallDepth)
	}

	argsValue, err := c.callArgumentToValue(args)
	if err != nil {
		return nil, false, err
	}

	select {
	case <-ctx.Done():
		return nil, false, fmt.Errorf("context is closed: %w", ctx.Err())
	default:
	}

	var path string
	if c.Scope.Path() != "" {
		path = "()"
	}

	scope := c.Scope
	for _, arg := range args {
		if arg.Self {
			scope = scope.Push(ValueScopeLookup{
				Value: arg.Value,
			})
			break
		}
	}

	if c.UnscopedArgs {
		scope = scope.Push(ValueScopeLookup{
			Value: argsValue,
		}, ScopeOption{
			Path:    path,
			Context: ctx,
			Call:    c.CallScope,
		})
	} else {
		rootData := map[string]any{
			"args": argsValue,
		}

		scope = scope.Push(ScopeData(rootData), ScopeOption{
			Path:    path,
			Context: ctx,
			Call:    c.CallScope,
		})

		if c.AssignRoot {
			scope = NewRootScope(c.Pos, scope, rootData)
		}
	}

	ret, ok, err := c.Body.ToValue(scope)
	if err != nil || !ok {
		return nil, ok, err
	}
	if c.ReturnBody {
		return ret, true, nil
	}
	return value.Lookup(ret, value.NewValue("return"))
}
