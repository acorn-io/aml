// Copyright 2018 The CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package format

import (
	"errors"
	"fmt"
	"strings"

	"github.com/acorn-io/aml/pkg/ast"
	"github.com/acorn-io/aml/pkg/token"
)

func printNode(node interface{}, f *printer) error {
	s := newFormatter(f)

	// format node
	f.allowed = nooverride // gobble initial whitespace.
	switch x := node.(type) {
	case *ast.File:
		s.file(x)
	case ast.Expr:
		s.expr(x)
	case ast.Decl:
		s.decl(x)
	case []ast.Decl:
		s.walkDeclList(x)
	default:
		goto unsupported
	}

	return errors.Join(s.errs...)

unsupported:
	return fmt.Errorf("cue/format: unsupported node type %T", node)
}

// Helper functions for common node lists. They may be empty.

func nestDepth(f *ast.Field) int {
	d := 1
	if s, ok := f.Value.(*ast.StructLit); ok {
		switch {
		case len(s.Elts) != 1:
			d = 0
		default:
			if f, ok := s.Elts[0].(*ast.Field); ok {
				d += nestDepth(f)
			}
		}
	}
	return d
}

// TODO: be more accurate and move to astutil
func hasDocComments(d ast.Decl) bool {
	if len(ast.Comments(d)) > 0 {
		return true
	}
	switch x := d.(type) {
	case *ast.Field:
		return len(ast.Comments(x.Label)) > 0
	case *ast.LetClause:
		return len(x.Ident.Comments()) > 0
	}
	return false
}

func isDefinition(label ast.Label) bool {
	switch x := label.(type) {
	case *ast.Ident:
		return isDef(x.Name)
	}
	return false
}

func isDef(s string) bool {
	return strings.HasPrefix(s, "#") || strings.HasPrefix(s, "_#")
}

func (f *formatter) walkDeclList(list []ast.Decl) {
	f.before(nil)
	d := 0
	for i, x := range list {
		if i > 0 {
			f.print(declcomma)
			nd := 0
			if f, ok := x.(*ast.Field); ok {
				nd = nestDepth(f)
			}
			if f.current.parentSep == newline && (d == 0 || nd != d) {
				f.print(f.formfeed())
			}
			if hasDocComments(x) {
				switch x := list[i-1].(type) {
				case *ast.Field:
					if isDefinition(x.Label) {
						f.print(newsection)
					}

				default:
					f.print(newsection)
				}
			}
		}
		f.decl(x)
		d = 0
		if f, ok := x.(*ast.Field); ok {
			d = nestDepth(f)
		}
		if j := i + 1; j < len(list) {
			switch x := list[j].(type) {
			case *ast.Field:
				switch x := x.Value.(type) {
				case *ast.StructLit:
					// TODO: not entirely correct: could have multiple elements,
					// not have a valid Lbrace, and be marked multiline. This
					// cannot occur for ASTs resulting from a parse, though.
					if x.Lbrace.IsValid() || len(x.Elts) != 1 {
						f.print(f.formfeed())
						continue
					}
				case *ast.ListLit:
					f.print(f.formfeed())
					continue
				}
			}
		}
		f.print(f.current.parentSep)
	}
	f.after(nil)
}

func (f *formatter) walkListElems(list []ast.Expr) {
	f.before(nil)
	for _, x := range list {
		f.before(x)
		switch n := x.(type) {
		case ast.Expr:
			f.exprRaw(n, token.LowestPrec, 1)
		}
		f.print(comma, blank)
		f.after(x)
	}
	f.after(nil)
}

func (f *formatter) walkArgsList(list []ast.Decl, depth int) {
	f.before(nil)
	for _, x := range list {
		f.before(x)
		f.decl(x)
		// I have no idea what this line does but doing this made embedded expressions have a space after the comma
		f.allowed = comma | blank
		f.print(comma, blank)
		f.after(x)
	}
	f.after(nil)
}

func (f *formatter) file(file *ast.File) {
	f.before(file)
	f.walkDeclList(file.Decls)
	f.after(file)
	f.print(token.EOF)
}

func (f *formatter) inlineField(n *ast.Field) *ast.Field {
	// shortcut single-element structs.
	// If the label has a valid position, we assume that an unspecified
	// Lbrace signals the intend to collapse fields.
	if !n.Label.Pos().IsValid() {
		return nil
	}

	obj, ok := n.Value.(*ast.StructLit)
	if !ok || len(obj.Elts) != 1 ||
		obj.Lbrace.IsValid() ||
		(obj.Lbrace.IsValid() && hasDocComments(n)) {
		return nil
	}

	mem, ok := obj.Elts[0].(*ast.Field)
	if !ok {
		return nil
	}

	if hasDocComments(mem) {
		// TODO: this inserts curly braces even in spaces where this
		// may not be desirable, such as:
		// a:
		//   // foo
		//   b: 3
		return nil
	}
	return mem
}

func (f *formatter) decl(decl ast.Decl) {
	if decl == nil {
		return
	}
	defer f.after(decl)
	if !f.before(decl) {
		return
	}

	switch n := decl.(type) {
	case *ast.Field:
		if n.Match != token.NoPos {
			f.print(n.Match, "match", blank)
		}
		f.label(n.Label, n.Constraint)
		f.print(noblank, nooverride, n.Colon, token.COLON)

		if mem := f.inlineField(n); mem != nil {
			switch {
			default:
				f.print(blank, nooverride)
				f.decl(mem)

			case mem.Label.Pos().IsNewline():
				f.print(indent, formfeed)
				f.decl(mem)
				f.indent--
			}
			return
		}

		nextFF := f.nextNeedsFormfeed(n.Value)
		tab := vtab
		if nextFF {
			tab = blank
		}

		f.print(tab)

		if n.Value != nil {
			switch n.Value.(type) {
			case *ast.ListLit, *ast.StructLit:
				f.expr(n.Value)
			default:
				f.print(indent)
				f.expr(n.Value)
				f.markUnindentLine()
			}
		} else {
			f.current.pos++
			f.visitComments(f.current.pos)
		}

		if nextFF {
			f.print(formfeed)
		}

	case *ast.BadDecl:
		f.print(n.From, "*bad decl*", declcomma)

	case *ast.LetClause:
		if !decl.Pos().HasRelPos() || decl.Pos().RelPos() >= token.Newline {
			f.print(formfeed)
		}
		f.print(n.Let, token.LET, blank, nooverride)
		f.expr(n.Ident)
		f.print(noblank, nooverride, n.Colon, token.COLON, blank)
		f.expr(n.Expr)
		f.print(declcomma) // implied

	case *ast.EmbedDecl:
		if !n.Pos().HasRelPos() || n.Pos().RelPos() >= token.Newline {
			f.print(formfeed)
		}
		f.expr(n.Expr)
		f.print(newline, noblank)

	case *ast.CommentGroup:
		f.printComment(n)
		f.print(newsection)

	case ast.Expr:
		f.embedding(n)
	}
}

func (f *formatter) embedding(decl ast.Expr) {
	switch n := decl.(type) {
	case ast.Expr:
		f.exprRaw(n, token.LowestPrec, 1)
	}
}

func (f *formatter) nextNeedsFormfeed(n ast.Expr) bool {
	switch x := n.(type) {
	case *ast.StructLit:
		return true
	case *ast.BasicLit:
		return strings.IndexByte(x.Value, '\n') >= 0
	case *ast.ListLit:
		return true
	}
	return false
}

func (f *formatter) label(l ast.Label, constraint token.Token) {
	f.before(l)
	defer f.after(l)
	switch n := l.(type) {

	case *ast.Ident:
		// Escape an identifier that has invalid characters. This may happen,
		// if the AST is not generated by the parser.
		name := n.Name
		f.print(n.NamePos, name)

	case *ast.BasicLit:
		str := n.Value
		f.print(n.ValuePos, str)

	case *ast.Interpolation:
		f.expr(n)

	default:
		panic(fmt.Sprintf("unknown label type %T", n))
	}
	if constraint != token.ILLEGAL {
		f.print(constraint)
	}
}

func (f *formatter) expr(x ast.Expr) {
	const depth = 1
	f.expr1(x, token.LowestPrec, depth)
}

func (f *formatter) expr0(x ast.Expr, depth int) {
	f.expr1(x, token.LowestPrec, depth)
}

func (f *formatter) expr1(expr ast.Expr, prec1, depth int) {
	if f.before(expr) {
		f.exprRaw(expr, prec1, depth)
	}
	f.after(expr)
}

func (f *formatter) exprRaw(expr ast.Expr, prec1, depth int) {

	switch x := expr.(type) {
	case *ast.BadExpr:
		f.print(x.From, "INVALID EXPRESSION")

	case *ast.Ident:
		f.print(x.NamePos, x)

	case *ast.DefaultExpr:
		f.print(x.Default, token.DEFAULT, blank)
		f.exprRaw(x.X, token.LowestPrec, depth)

	case *ast.BinaryExpr:
		if depth < 1 {
			f.internalError("depth < 1:", depth)
			depth = 1
		}
		f.binaryExpr(x, prec1, cutoff(x, depth), depth)

	case *ast.UnaryExpr:
		const prec = token.UnaryPrec
		if prec < prec1 {
			// parenthesis needed
			f.print(token.LPAREN, nooverride)
			f.expr(x)
			f.print(token.RPAREN)
		} else {
			// no parenthesis needed
			f.print(x.OpPos, x.Op, nooverride)
			f.expr1(x.X, prec, depth)
		}

	case *ast.BasicLit:
		f.print(x.ValuePos, x)

	case *ast.Interpolation:
		f.before(nil)
		for _, x := range x.Elts {
			f.expr0(x, depth+1)
		}
		f.after(nil)

	case *ast.ParenExpr:
		if _, hasParens := x.X.(*ast.ParenExpr); hasParens {
			// don't print parentheses around an already parenthesized expression
			// TODO: consider making this more general and incorporate precedence levels
			f.expr0(x.X, depth)
		} else {
			f.print(x.Lparen, token.LPAREN)
			f.expr0(x.X, reduceDepth(depth)) // parentheses undo one level of depth
			f.print(x.Rparen, token.RPAREN)
		}

	case *ast.SelectorExpr:
		f.selectorExpr(x, depth)

	case *ast.IndexExpr:
		f.expr1(x.X, token.HighestPrec, 1)
		f.print(x.Lbrack, token.LBRACK)
		f.expr0(x.Index, depth+1)
		f.print(x.Rbrack, token.RBRACK)

	case *ast.SliceExpr:
		f.expr1(x.X, token.HighestPrec, 1)
		f.print(x.Lbrack, token.LBRACK)
		indices := []ast.Expr{x.Low, x.High}
		for i, y := range indices {
			if i > 0 {
				// blanks around ":" if both sides exist and either side is a binary expression
				x := indices[i-1]
				if depth <= 1 && x != nil && y != nil && (isBinary(x) || isBinary(y)) {
					f.print(blank, token.COLON, blank)
				} else {
					f.print(token.COLON)
				}
			}
			if y != nil {
				f.expr0(y, depth+1)
			}
		}
		f.print(x.Rbrack, token.RBRACK)

	case *ast.CallExpr:
		if len(x.Args) > 1 {
			depth++
		}
		f.possibleSelectorExpr(x.Fun, token.HighestPrec, depth)
		f.print(x.Lparen, token.LPAREN)
		f.walkArgsList(x.Args, depth)
		f.print(trailcomma, noblank, x.Rparen, token.RPAREN)

	case *ast.Func:
		f.print(x.Func, token.FUNCTION, blank, nooverride)
		f.exprRaw(x.Body, token.LowestPrec, depth)

	case *ast.Lambda:
		f.print(x.Lambda, token.LAMBDA, blank, nooverride)
		for i, ident := range x.Idents {
			if i > 0 {
				f.print(token.COMMA, blank, nooverride)
			}
			f.print(ident.Pos(), ident.Name, noblank, nooverride)
		}
		f.print(x.Colon, token.COLON, blank, nooverride)
		f.exprRaw(x.Expr, token.LowestPrec, depth)

	case *ast.SchemaLit:
		f.print(x.Schema, token.SCHEMA, blank)
		f.exprRaw(x.Struct, token.LowestPrec, depth)

	case *ast.StructLit:
		var l line
		ws := noblank
		ff := f.formfeed()

		switch {
		case len(x.Elts) == 0:
			if !x.Rbrace.HasRelPos() {
				// collapse curly braces if the body is empty.
				ffAlt := blank | nooverride
				for _, c := range x.Comments() {
					if c.Position == 1 {
						ffAlt = ff
					}
				}
				ff = ffAlt
			}
		case !x.Rbrace.HasRelPos() || !x.Elts[0].Pos().HasRelPos():
			ws |= newline | nooverride
		}
		f.print(x.Lbrace, token.LBRACE, &l, ws, ff, indent)

		f.walkDeclList(x.Elts)
		f.matchUnindent()

		ws = noblank
		if f.lineout != l {
			ws |= newline
			if f.lastTok != token.RBRACE && f.lastTok != token.RBRACK {
				ws |= nooverride
			}
		}
		f.print(ws, x.Rbrace, token.RBRACE)

	case *ast.ListLit:
		f.print(x.Lbrack, token.LBRACK, indent)
		f.walkListElems(x.Elts)
		f.print(trailcomma, noblank)
		f.visitComments(f.current.pos)
		f.matchUnindent()
		f.print(noblank, x.Rbrack, token.RBRACK)

	case *ast.For:
		f.print(x.For, "for", blank)
		f.print(indent)
		f.clause(x.Clause)
		f.expr(x.Struct)

	case *ast.If:
		f.print(x.If, "if", blank)
		f.print(indent)
		f.clause(x.Condition)
		f.expr(x.Struct)
		if x.Else != nil {
			f.expr(x.Else)
		}

	case *ast.Else:
		f.print(x.Else, "else", blank)
		if x.If != nil {
			f.expr(x.If)
		}
		if x.Struct != nil {
			f.expr(x.Struct)
		}

	case *ast.ListComprehension:
		f.print(x.Lbrack, token.LBRACK, indent)
		f.print(x.For, "for", blank)
		f.clause(x.Clause)
		f.print(blank, nooverride)
		f.expr(x.Value)
		f.print(noblank, x.Rbrack, token.RBRACK)

	default:
		panic(fmt.Sprintf("unimplemented type %T", x))
	}
}

func (f *formatter) clause(clause ast.Clause) {
	switch n := clause.(type) {
	case *ast.ForClause:
		if n.Key != nil {
			f.label(n.Key, token.ILLEGAL)
			f.print(n.Comma, token.COMMA, blank)
		} else {
			f.current.pos++
			f.visitComments(f.current.pos)
		}
		f.label(n.Value, token.ILLEGAL)
		f.print(blank, n.In, "in", blank)
		f.expr(n.Source)
		f.markUnindentLine()

	case *ast.IfClause:
		f.print(indent)
		f.expr(n.Condition)
		f.markUnindentLine()

	case *ast.LetClause:
		f.print(n.Let, token.LET, blank, nooverride)
		f.print(indent)
		f.expr(n.Ident)
		f.print(noblank, nooverride, n.Colon, token.COLON, blank)
		f.expr(n.Expr)
		f.markUnindentLine()

	default:
		panic("unknown clause type")
	}
}

func walkBinary(e *ast.BinaryExpr) (has6, has7, has8 bool, maxProblem int) {
	switch e.Op.Precedence() {
	case 6:
		has6 = true
	case 7:
		has7 = true
	case 8:
		has8 = true
	}

	switch l := e.X.(type) {
	case *ast.BinaryExpr:
		if l.Op.Precedence() < e.Op.Precedence() {
			// parens will be inserted.
			// pretend this is an *syntax.ParenExpr and do nothing.
			break
		}
		h6, h7, h8, mp := walkBinary(l)
		has6 = has6 || h6
		has7 = has7 || h7
		has8 = has8 || h8
		if maxProblem < mp {
			maxProblem = mp
		}
	}

	switch r := e.Y.(type) {
	case *ast.BinaryExpr:
		if r.Op.Precedence() <= e.Op.Precedence() {
			// parens will be inserted.
			// pretend this is an *syntax.ParenExpr and do nothing.
			break
		}
		h6, h7, h8, mp := walkBinary(r)
		has6 = has6 || h6
		has7 = has7 || h7
		has8 = has8 || h8
		if maxProblem < mp {
			maxProblem = mp
		}

	case *ast.UnaryExpr:
		switch e.Op.String() + r.Op.String() {
		case "/*":
			maxProblem = 8
		case "++", "--":
			if maxProblem < 6 {
				maxProblem = 6
			}
		}
	}
	return
}

func cutoff(e *ast.BinaryExpr, depth int) int {
	has6, has7, has8, maxProblem := walkBinary(e)
	if maxProblem > 0 {
		return maxProblem + 1
	}
	if (has6 || has7) && has8 {
		if depth == 1 {
			return 8
		}
		if has7 {
			return 7
		}
		return 6
	}
	if has6 && has7 {
		if depth == 1 {
			return 7
		}
		return 6
	}
	if depth == 1 {
		return 8
	}
	return 6
}

func diffPrec(expr ast.Expr, prec int) int {
	x, ok := expr.(*ast.BinaryExpr)
	if !ok || prec != x.Op.Precedence() {
		return 1
	}
	return 0
}

func reduceDepth(depth int) int {
	depth--
	if depth < 1 {
		depth = 1
	}
	return depth
}

// Format the binary expression: decide the cutoff and then format.
// Let's call depth == 1 Normal mode, and depth > 1 Compact mode.
// (Algorithm suggestion by Russ Cox.)
//
// The precedences are:
//
//	7             *  /  % quo rem div mod
//	6             +  -
//	5             ==  !=  <  <=  >  >=
//	4             &&
//	3             ||
//	2             &
//	1             |
//
// The only decision is whether there will be spaces around levels 6 and 7.
// There are never spaces at level 8 (unary), and always spaces at levels 5 and below.
//
// To choose the cutoff, look at the whole expression but excluding primary
// expressions (function calls, parenthesized exprs), and apply these rules:
//
//  1. If there is a binary operator with a right side unary operand
//     that would clash without a space, the cutoff must be (in order):
//
//     /*	8
//     ++	7 // not necessary, but to avoid confusion
//     --	7
//
//     (Comparison operators always have spaces around them.)
//
//  2. If there is a mix of level 7 and level 6 operators, then the cutoff
//     is 7 (use spaces to distinguish precedence) in Normal mode
//     and 6 (never use spaces) in Compact mode.
//
//  3. If there are no level 6 operators or no level 7 operators, then the
//     cutoff is 8 (always use spaces) in Normal mode
//     and 6 (never use spaces) in Compact mode.
func (f *formatter) binaryExpr(x *ast.BinaryExpr, prec1, cutoff, depth int) {
	f.nestExpr++
	defer func() { f.nestExpr-- }()

	prec := x.Op.Precedence()
	if prec < prec1 {
		// parenthesis needed
		// Note: The parser inserts an syntax.ParenExpr node; thus this case
		//       can only occur if the AST is created in a different way.
		// defer p.pushComment(nil).pop()
		f.print(token.LPAREN, nooverride)
		f.expr0(x, reduceDepth(depth)) // parentheses undo one level of depth
		f.print(token.RPAREN)
		return
	}

	printBlank := prec < cutoff

	f.expr1(x.X, prec, depth+diffPrec(x.X, prec))
	f.print(nooverride)
	if printBlank {
		f.print(blank)
	}
	f.print(x.OpPos, x.Op)
	if x.Y.Pos().IsNewline() {
		// at least one line break, but respect an extra empty line
		// in the source
		f.print(formfeed)
		printBlank = false // no blank after line break
	} else {
		f.print(nooverride)
	}
	if printBlank {
		f.print(blank)
	}
	f.expr1(x.Y, prec+1, depth+1)
}

func isBinary(expr ast.Expr) bool {
	_, ok := expr.(*ast.BinaryExpr)
	return ok
}

func (f *formatter) possibleSelectorExpr(expr ast.Expr, prec1, depth int) bool {
	if x, ok := expr.(*ast.SelectorExpr); ok {
		return f.selectorExpr(x, depth)
	}
	f.expr1(expr, prec1, depth)
	return false
}

// selectorExpr handles an *syntax.SelectorExpr node and returns whether x spans
// multiple lines.
func (f *formatter) selectorExpr(x *ast.SelectorExpr, depth int) bool {
	f.expr1(x.X, token.HighestPrec, depth)
	f.print(token.PERIOD)
	if x.Sel.Pos().IsNewline() {
		f.print(indent, formfeed)
		f.expr(x.Sel.(ast.Expr))
		f.print(unindent)
		return true
	}
	f.print(noblank)
	f.expr(x.Sel.(ast.Expr))
	return false
}
