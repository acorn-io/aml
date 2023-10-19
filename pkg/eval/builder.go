package eval

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/acorn-io/aml/pkg/ast"
	"github.com/acorn-io/aml/pkg/errors"
	"github.com/acorn-io/aml/pkg/token"
	"github.com/acorn-io/aml/pkg/value"
)

type BuildOption struct {
	PositionalArgs []any
	Args           map[string]any
	Profiles       []string
}

type BuildOptions []BuildOption

func (b BuildOptions) Merge() (merged BuildOption) {
	for _, opt := range b {
		merged.PositionalArgs = append(merged.PositionalArgs, opt.PositionalArgs...)
		if merged.Args == nil {
			merged.Args = map[string]any{}
		}
		for k, v := range opt.Args {
			merged.Args[k] = v
		}
		merged.Profiles = append(merged.Profiles, opt.Profiles...)
	}
	return
}

func Build(file *ast.File, opts ...BuildOption) (*File, error) {
	s, err := fileToObject(file)
	if err != nil {
		return nil, err
	}
	opt := BuildOptions(opts).Merge()
	return &File{
		PositionalArgs: opt.PositionalArgs,
		Args:           opt.Args,
		Profiles:       opt.Profiles,
		Body:           s,
	}, nil
}

func fileToObject(file *ast.File) (*Struct, error) {
	fields, err := declsToFields(file.Decls)
	if err != nil {
		return nil, err
	}

	return &Struct{
		Position: pos(file.Pos()),
		Comments: getComments(file),
		Fields:   fields,
	}, err
}

func declsToFields(decls []ast.Decl) (result []Field, err error) {
	var (
		errs   []error
		fields []Field
	)

	for _, decl := range decls {
		field, err := declToField(decl)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		fields = append(fields, field)
	}

	return fields, errors.Join(errs...)
}

func declToField(decl ast.Decl) (ref Field, err error) {
	switch v := decl.(type) {
	case *ast.Field:
		var result KeyValue
		result.Comments = getComments(decl)
		result.Optional = v.Constraint == token.OPTION
		result.Key, err = labelToKey(v.Label, v.Match != token.NoPos)
		if err != nil {
			return &result, err
		}
		result.Pos = pos(decl.Pos())
		result.Value, err = exprToExpression(v.Value)
		return &result, err
	case *ast.EmbedDecl:
		var result Embedded
		result.Pos = pos(decl.Pos())
		result.Comments = getComments(decl)
		result.Expression, err = exprToExpression(v.Expr)
		return &result, err
	case *ast.LetClause:
		var result KeyValue
		result.Comments = getComments(decl)
		result.Local = true
		result.Key, err = labelToKey(v.Ident, false)
		if err != nil {
			return nil, err
		}

		result.Value, err = exprToExpression(v.Expr)
		return &result, err
	default:
		return nil, NewErrUnknownError(decl)
	}
}

func defaultToExpression(comp *ast.DefaultExpr) (Expression, error) {
	expr, err := exprToExpression(comp.X)
	if err != nil {
		return nil, err
	}

	return &Default{
		Comments: getComments(comp),
		Expr:     expr,
		Pos:      pos(comp.Default),
	}, nil
}

func interpolationToExpression(comp *ast.Interpolation) (*Interpolation, error) {
	result := &Interpolation{}

	for i := range comp.Elts {
		switch {
		case i == 0:
			lit := *comp.Elts[i].(*ast.BasicLit)
			lit.Value = strings.TrimSuffix(lit.Value, "\\(")
			result.Parts = append(result.Parts, lit.Value)
		case i == len(comp.Elts)-1:
			lit := *comp.Elts[i].(*ast.BasicLit)
			lit.Value = strings.TrimPrefix(lit.Value, ")")
			result.Parts = append(result.Parts, lit.Value)
		case i%2 == 0:
			lit := *comp.Elts[i].(*ast.BasicLit)
			lit.Value = strings.TrimPrefix(lit.Value, ")")
			lit.Value = strings.TrimSuffix(lit.Value, "\\(")
			result.Parts = append(result.Parts, lit.Value)
		case i%2 == 1:
			expr, err := exprToExpression(comp.Elts[i])
			if err != nil {
				return nil, err
			}
			result.Parts = append(result.Parts, expr)
		}
	}

	return result, nil
}

func elseToExpression(c *ast.Else) (Expression, error) {
	if c.If != nil {
		return ifToExpression(c.If)
	}
	return structToExpression(c.Struct)
}

func ifToExpression(c *ast.If) (Expression, error) {
	value, err := exprToExpression(c.Struct)
	if err != nil {
		return nil, err
	}

	var elseExpr Expression
	if c.Else != nil {
		elseExpr, err = exprToExpression(c.Else)
		if err != nil {
			return nil, err
		}
	}

	condition, err := exprToExpression(c.Condition.Condition)
	if err != nil {
		return nil, err
	}

	return &If{
		Pos:       pos(c.If),
		Comments:  getComments(c),
		Condition: condition,
		Value:     value,
		Else:      elseExpr,
	}, nil
}

func listComprehensionToExpression(c *ast.ListComprehension) (Expression, error) {
	value, err := exprToExpression(c.Value)
	if err != nil {
		return nil, err
	}

	return forClauseToFor(c.Clause, value, false)
}

func forToExpression(c *ast.For) (Expression, error) {
	value, err := exprToExpression(c.Struct)
	if err != nil {
		return nil, err
	}

	e, err := forClauseToFor(c.Clause, value, true)
	if err != nil {
		return nil, err
	}

	if c.Else != nil {
		e.Else, err = exprToExpression(c.Else)
		if err != nil {
			return nil, err
		}
	}

	e.Merge = true
	return e, nil
}

func forClauseToFor(comp *ast.ForClause, expr Expression, merge bool) (*For, error) {
	var (
		result = &For{
			Comments: getComments(comp),
			Body:     expr,
			Merge:    merge,
			Position: pos(comp.Pos()),
		}
		err error
	)

	if comp.Key != nil {
		result.Key, err = value.Unquote(comp.Key.Name)
		if err != nil {
			return nil, value.NewErrPosition(posValue(comp.Key.Pos()), err)
		}
	}

	result.Value, err = value.Unquote(comp.Value.Name)
	if err != nil {
		return nil, value.NewErrPosition(posValue(comp.Value.Pos()), err)
	}

	result.Collection, err = exprToExpression(comp.Source)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func basicListToValue(lit *ast.BasicLit) (Expression, error) {
	switch lit.Kind {
	case token.NUMBER:
		return Value{
			Value: value.Number(lit.Value),
		}, nil
	case token.STRING:
		s, err := value.Unquote(lit.Value)
		if err != nil {
			return nil, value.NewErrPosition(posValue(lit.Pos()), err)
		}
		return Value{
			Value: value.NewValue(s),
		}, nil
	case token.TRUE:
		return Value{
			Value: value.True,
		}, nil
	case token.FALSE:
		return Value{
			Value: value.False,
		}, nil
	case token.NULL:
		return Value{
			Value: &value.Null{},
		}, nil
	default:
		return nil, fmt.Errorf("unknown literal kind %s, value %s at %s", lit.Kind.String(), lit.Value, lit.Pos())
	}
}

func schemaToExpression(s *ast.SchemaLit) (Expression, error) {
	decl, err := declToField(s.Decl)
	if err != nil {
		return nil, err
	}
	if field, ok := decl.(*KeyValue); ok {
		field.Value = &Schema{
			Comments:   field.Comments,
			Expression: field.Value,
		}
		return &Struct{
			Position: pos(s.Pos()),
			Comments: getComments(s),
			Fields:   []Field{field},
		}, nil
	}
	return &Schema{
		Comments: getComments(s),
		Expression: &Struct{
			Position: pos(s.Pos()),
			Comments: getComments(s),
			Fields:   []Field{decl},
		},
	}, nil
}

func structToExpression(s *ast.StructLit) (*Struct, error) {
	fields, err := declsToFields(s.Elts)
	if err != nil {
		return nil, err
	}
	return &Struct{
		Position: pos(s.Pos()),
		Comments: getComments(s),
		Fields:   fields,
	}, err
}

func listToExpression(list *ast.ListLit) (Expression, error) {
	exprs, err := exprsToExpressions(list.Elts)
	if err != nil {
		return nil, err
	}
	return &Array{
		Pos:      pos(list.Lbrack),
		Comments: getComments(list),
		Items:    exprs,
	}, nil
}

func exprsToExpressions(exprs []ast.Expr) (result []Expression, _ error) {
	var errs []error
	for _, expr := range exprs {
		newExpr, err := exprToExpression(expr)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		result = append(result, newExpr)
	}
	return result, errors.Join(errs...)
}

func unaryToExpression(bin *ast.UnaryExpr) (Expression, error) {
	left, err := exprToExpression(bin.X)
	if err != nil {
		return nil, err
	}

	return &Op{
		Comments: getComments(bin),
		Unary:    true,
		Operator: value.Operator(bin.Op.String()),
		Left:     left,
		Pos:      pos(bin.OpPos),
	}, nil
}

func pos(t token.Pos) value.Position {
	return value.Position(t.Position())
}

func posValue(t token.Pos) value.Position {
	return value.Position(t.Position())
}

func binaryToExpression(bin *ast.BinaryExpr) (Expression, error) {
	left, err := exprToExpression(bin.X)
	if err != nil {
		return nil, err
	}

	right, err := exprToExpression(bin.Y)
	if err != nil {
		return nil, err
	}

	return &Op{
		Comments: getComments(bin),
		Operator: value.Operator(bin.Op.String()),
		Left:     left,
		Right:    right,
		Pos:      pos(bin.OpPos),
	}, nil
}

func parensToExpression(parens *ast.ParenExpr) (Expression, error) {
	expr, err := exprToExpression(parens.X)
	return &Parens{
		Comments: getComments(parens),
		Expr:     expr,
	}, err
}

func identToExpression(ident *ast.Ident) (Expression, error) {
	key, err := value.Unquote(ident.Name)
	if err != nil {
		return nil, value.NewErrPosition(posValue(ident.Pos()), err)
	}
	return &Lookup{
		Comments: getComments(ident),
		Pos:      pos(ident.NamePos),
		Key:      key,
	}, nil
}

func selectorToExpression(sel *ast.SelectorExpr) (Expression, error) {
	str, key, _, err := labelToExpression(sel.Sel)
	if err != nil {
		return nil, err
	}

	if key == nil {
		key = Value{
			Value: value.NewValue(str),
		}
	}

	selExpr, err := exprToExpression(sel.X)
	if err != nil {
		return nil, err
	}

	return &Selector{
		Comments: getComments(sel),
		Pos:      pos(sel.Sel.Pos()),
		Base:     selExpr,
		Key:      key,
	}, nil
}

func indexToExpression(indexExpr *ast.IndexExpr) (Expression, error) {
	base, err := exprToExpression(indexExpr.X)
	if err != nil {
		return nil, err
	}

	index, err := exprToExpression(indexExpr.Index)
	if err != nil {
		return nil, err
	}

	return &Index{
		Comments: getComments(indexExpr),
		Pos:      pos(indexExpr.Pos()),
		Base:     base,
		Index:    index,
	}, nil
}

func sliceToExpression(sliceExpr *ast.SliceExpr) (Expression, error) {
	base, err := exprToExpression(sliceExpr.X)
	if err != nil {
		return nil, err
	}

	low, err := exprToExpression(sliceExpr.Low)
	if err != nil {
		return nil, err
	}

	high, err := exprToExpression(sliceExpr.High)
	if err != nil {
		return nil, err
	}

	return &Slice{
		Comments: getComments(sliceExpr),
		Pos:      pos(sliceExpr.Lbrack),
		Base:     base,
		Start:    low,
		End:      high,
	}, nil
}

func lambdaToExpression(def *ast.Lambda) (Expression, error) {
	expr, err := exprToExpression(def.Expr)
	if err != nil {
		return nil, err
	}

	lambda := &LambdaDefinition{
		Comments: getComments(def),
		Pos:      pos(def.Lambda),
		Body:     expr,
	}

	for _, ident := range def.Idents {
		s, err := value.Unquote(ident.Name)
		if err != nil {
			return nil, err
		}
		lambda.Vars = append(lambda.Vars, s)
	}

	return lambda, nil
}

func funcToExpression(def *ast.Func) (Expression, error) {
	body, err := structToExpression(def.Body)
	if err != nil {
		return nil, err
	}
	returnType, err := exprToExpression(def.ReturnType)
	if err != nil {
		return nil, err
	}
	return &FunctionDefinition{
		Comments:   getComments(def),
		Pos:        pos(def.Func),
		Body:       body,
		ReturnType: returnType,
	}, nil
}

func callToExpression(callExpr *ast.CallExpr) (Expression, error) {
	f, err := exprToExpression(callExpr.Fun)
	if err != nil {
		return nil, err
	}

	args, err := declsToFields(callExpr.Args)
	if err != nil {
		return nil, err
	}

	return &Call{
		Comments: getComments(callExpr),
		Pos:      pos(callExpr.Lparen),
		Func:     f,
		Args:     args,
	}, nil
}

func exprToExpression(expr ast.Expr) (Expression, error) {
	if expr == nil {
		return nil, nil
	}

	switch n := expr.(type) {
	case *ast.BasicLit:
		return basicListToValue(n)
	case *ast.StructLit:
		return structToExpression(n)
	case *ast.SchemaLit:
		return schemaToExpression(n)
	case *ast.ListLit:
		return listToExpression(n)
	case *ast.BinaryExpr:
		return binaryToExpression(n)
	case *ast.UnaryExpr:
		return unaryToExpression(n)
	case *ast.ParenExpr:
		return parensToExpression(n)
	case *ast.Ident:
		return identToExpression(n)
	case *ast.SelectorExpr:
		return selectorToExpression(n)
	case *ast.IndexExpr:
		return indexToExpression(n)
	case *ast.SliceExpr:
		return sliceToExpression(n)
	case *ast.CallExpr:
		return callToExpression(n)
	case *ast.If:
		return ifToExpression(n)
	case *ast.Else:
		return elseToExpression(n)
	case *ast.For:
		return forToExpression(n)
	case *ast.ListComprehension:
		return listComprehensionToExpression(n)
	case *ast.Interpolation:
		return interpolationToExpression(n)
	case *ast.DefaultExpr:
		return defaultToExpression(n)
	case *ast.Func:
		return funcToExpression(n)
	case *ast.Lambda:
		return lambdaToExpression(n)
	default:
		return nil, NewErrUnknownError(n)
	}
}

func labelToKey(label ast.Label, match bool) (FieldKey, error) {
	str, expr, stringKey, err := labelToExpression(label)
	if err != nil {
		return FieldKey{}, err
	}
	if stringKey && !match {
		return FieldKey{
			Match: &Value{
				Value: value.NewValue(".*"),
			},
			Pos: pos(label.Pos()),
		}, nil
	} else if match {
		if expr == nil {
			expr = Value{
				Value: value.NewValue(str),
			}
		}
		return FieldKey{
			Match: expr,
			Pos:   pos(label.Pos()),
		}, nil
	} else if expr != nil {
		return FieldKey{
			Interpolation: expr,
			Pos:           pos(label.Pos()),
		}, nil
	}
	return FieldKey{
		Key: str,
		Pos: pos(label.Pos()),
	}, nil
}

func labelToExpression(expr ast.Label) (s string, _ Expression, isStringIdent bool, _ error) {
	if expr == nil {
		return "", nil, false, nil
	}

	switch n := expr.(type) {
	case *ast.BasicLit:
		s, err := value.Unquote(n.Value)
		if err != nil {
			return "", nil, false, value.NewErrPosition(posValue(n.Pos()), err)
		}
		return s, nil, false, nil
	case *ast.Ident:
		s, err := value.Unquote(n.Name)
		if err != nil {
			return s, nil, false, value.NewErrPosition(posValue(n.Pos()), err)
		}
		return s, nil, n.Name == "string", nil
	case *ast.Interpolation:
		i, err := interpolationToExpression(n)
		return "", i, false, err
	default:
		return "", nil, false, NewErrUnknownError(n)
	}
}

func getComments(node ast.Node) (result Comments) {
	for _, cg := range ast.Comments(node) {
		var group []string

		if cg != nil {
			for _, c := range cg.List {
				l := strings.TrimLeftFunc(strings.TrimPrefix(c.Text, "//"), unicode.IsSpace)
				group = append(group, l)
			}
		}
		result.Comments = append(result.Comments, group)
	}
	return
}

type ErrUnknownError struct {
	Node ast.Node
}

func (e *ErrUnknownError) Error() string {
	return fmt.Sprintf("unknown node %T encountered at %s", e.Node, e.Node.Pos())
}

func NewErrUnknownError(node ast.Node) error {
	return &ErrUnknownError{
		Node: node,
	}
}
