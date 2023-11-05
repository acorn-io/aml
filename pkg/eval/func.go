package eval

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/acorn-io/aml/pkg/value"
)

type FunctionDefinition struct {
	Comments   Comments
	Pos        value.Position
	Body       *Struct
	ReturnType Expression
	ReturnBody bool
	AssignRoot bool
}

func (f *FunctionDefinition) ToValue(ctx context.Context) (value.Value, bool, error) {
	argsFields, bodyFields := f.splitFields()
	argNames, argsSchema, err := f.toSchema(ctx, argsFields, "args", true)
	if err != nil {
		return nil, false, err
	}
	profileNames, profileSchema, err := f.toSchema(ctx, argsFields, "profiles", true)
	if err != nil {
		return nil, false, err
	}
	body := &Struct{
		Position: f.Pos,
		Fields:   bodyFields,
	}
	scope := GetScope(ctx)

	var args []value.ObjectSchemaField
	if argsSchema.Object != nil {
		args = argsSchema.Object.Fields
	}

	funcSchema := &value.FuncSchema{
		Args:         args,
		ProfileNames: profileNames,
		Returns: func(ctx context.Context) (value.Schema, bool, error) {
			if f.ReturnType == nil {
				return nil, false, err
			}
			v, ok, err := f.ReturnType.ToValue(WithScope(ctx, scope))
			if err != nil || !ok {
				return nil, ok, err
			}
			if s, ok := v.(value.Schema); ok {
				return s, true, nil
			}
			return nil, false, value.NewErrPosition(f.Pos,
				fmt.Errorf("return value is does not a schema type, got kind: %s", v.Kind()))
		},
	}

	returnFunc := &Function{
		Pos:            f.Pos,
		Scope:          scope,
		Body:           body,
		ReturnType:     f.ReturnType,
		ArgsSchema:     argsSchema,
		ArgNames:       argNames,
		ProfileNames:   profileNames,
		ProfilesSchema: profileSchema,
		ReturnBody:     f.ReturnBody,
		AssignRoot:     f.AssignRoot,
		FuncSchema:     funcSchema,
	}

	if IsSchema(ctx) && !f.AssignRoot {
		return &value.TypeSchema{
			Positions:    []value.Position{f.Pos},
			KindValue:    value.FuncKind,
			FuncSchema:   funcSchema,
			DefaultValue: returnFunc,
		}, true, nil
	}
	return returnFunc, true, nil
}

func (f *FunctionDefinition) toSchema(ctx context.Context, argDefs []Field, fieldName string, allowNewFields bool) (value.Names, *value.TypeSchema, error) {
	s := Schema{
		AllowNewFields: allowNewFields,
		Expression: &Struct{
			Position: f.Pos,
			Fields:   argDefs,
		},
	}
	v, _, err := s.ToValue(ctx)
	if err != nil {
		return nil, nil, err
	}

	v, ok, err := value.Lookup(v, value.NewValue(fieldName))
	if err != nil {
		return nil, nil, err
	} else if !ok {
		v = value.NewObject(nil)
	}

	if undef := value.IsUndefined(v); undef != nil {
		return nil, nil, value.NewErrPosition(f.Pos, fmt.Errorf("errors reading args: %s", undef))
	}

	if _, ok := v.(*value.TypeSchema); !ok {
		if allowNewFields {
			v = value.NewOpenObject()
		} else {
			v = value.NewClosedObject()
		}
	}

	var (
		names value.Names
		ts    = v.(*value.TypeSchema)
	)

	if ts.Object == nil {
		return names, ts, nil
	}

	for _, field := range ts.Object.Fields {
		names = append(names, value.Name{
			Name:        field.Key,
			Description: field.Description,
		})
	}

	return names, ts, err
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
	Pos            value.Position
	Scope          Scope
	Body           Expression
	ReturnType     Expression
	ArgsSchema     value.Schema
	ArgNames       value.Names
	ProfilesSchema value.Schema
	ProfileNames   value.Names
	FuncSchema     *value.FuncSchema
	ReturnBody     bool
	AssignRoot     bool
	UnscopedArgs   bool

	depth atomic.Int32
}

func (c *Function) Returns(ctx context.Context) (value.Value, bool, error) {
	if c.ReturnType == nil {
		return nil, false, nil
	}
	return c.ReturnType.ToValue(WithScope(ctx, c.Scope))
}

func (c *Function) Eq(right value.Value) (value.Value, error) {
	if rf, ok := right.(*Function); ok {
		return value.NewValue(c.Pos.String() == rf.Pos.String()), nil
	}
	return value.False, nil
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

func (c *Function) callArgumentToValue(ctx context.Context, args []value.CallArgument) (value.Value, error) {
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
		argValue, err = value.Validate(ctx, profiles[i], argValue)
		if err != nil {
			return nil, err
		}
	}

	validated, err := value.Validate(ctx, c.ArgsSchema, argValue)
	if err != nil {
		return validated, value.NewErrPosition(c.Pos, &ErrInvalidArgument{
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

func (c *Function) validateReturn(ctx context.Context, ret value.Value, ok bool, err error) (value.Value, bool, error) {
	if err != nil || !ok {
		return ret, ok, err
	}

	check, hasCheck, checkErr := c.Returns(ctx)
	if checkErr != nil {
		return nil, false, checkErr
	}
	if !hasCheck {
		return ret, ok, err
	}
	ret, err = value.Validate(ctx, check, ret)
	return ret, true, err
}

func (c *Function) Call(ctx context.Context, args []value.CallArgument) (ret value.Value, ok bool, err error) {
	defer c.depth.Add(-1)
	if depth := c.depth.Add(1); depth > MaxCallDepth {
		return nil, false, fmt.Errorf("exceeded max call depth %d > %d", depth, MaxCallDepth)
	}

	defer func() {
		ret, ok, err = c.validateReturn(ctx, ret, ok, err)
	}()

	argsValue, err := c.callArgumentToValue(ctx, args)
	if err != nil {
		return nil, false, err
	}

	select {
	case <-ctx.Done():
		return nil, false, fmt.Errorf("context is closed: %w", ctx.Err())
	default:
	}

	if !c.AssignRoot {
		ctx = value.WithCallPath(ctx)
	}
	scope := c.Scope

	for _, arg := range args {
		if arg.Self {
			scope, _ = scope.NewScope(ctx, ScopeData{
				"self": arg.Value,
			})
			break
		}
	}

	if c.UnscopedArgs {
		scope, _ = scope.NewScope(WithSchema(ctx, false), ValueScopeLookup{
			Value: argsValue,
		})
	} else {
		rootData := map[string]any{
			"args": argsValue,
		}

		if c.AssignRoot {
			rootData["__root"] = true
		} else {
			ctx = WithSchema(ctx, false)
		}

		scope, _ = scope.NewScope(ctx, ScopeData(rootData))
	}

	ctx = WithScope(ctx, scope)

	ret, ok, err = c.Body.ToValue(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}
	if c.ReturnBody {
		return ret, true, nil
	}
	return value.Lookup(ret, value.NewValue("return"))
}
