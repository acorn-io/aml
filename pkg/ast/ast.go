package ast

import (
	"strings"

	"github.com/acorn-io/aml/pkg/token"
)

// A Node represents any node in the abstract syntax tree.
type Node interface {
	Pos() token.Pos // position of first character belonging to the node
	End() token.Pos // position of first character immediately after the node

	// pos reports the pointer to the position of first character belonging to
	// the node or nil if there is no such position.
	pos() *token.Pos

	commentInfo() *comments
}

func getPos(n Node) token.Pos {
	p := n.pos()
	if p == nil {
		return token.NoPos
	}
	return *p
}

// SetRelPos sets the relative position of a node without modifying its
// file position. Setting it to token.NoRelPos allows a node to adopt default
// formatting.
func SetRelPos(n Node, p token.RelPos) {
	ptr := n.pos()
	if ptr == nil {
		return
	}
	pos := *ptr
	*ptr = pos.WithRel(p)
}

// An Expr is implemented by all expression nodes.
type Expr interface {
	Node
	IsExpr() bool
}

type isExpr struct{}

func (isExpr) IsExpr() bool { return true }

// A Decl node is implemented by all declarations.
type Decl interface {
	Node
	IsDecl() bool
}

type isDecl struct{}

func (isDecl) IsDecl() bool { return true }

// A Label is any production that can be used as a LHS label.
type Label interface {
	Node

	IsLabel() bool
}

type isLabel struct{}

func (l isLabel) IsLabel() bool { return true }

// Clause nodes are part of comprehensions.
type Clause interface {
	Node
	IsClause() bool
}

type isClause struct{}

func (isClause) IsClause() bool { return true }

// Comments

type comments struct {
	groups *[]*CommentGroup
}

func (c *comments) commentInfo() *comments { return c }

func (c *comments) Comments() []*CommentGroup {
	if c.groups == nil {
		return []*CommentGroup{}
	}
	return *c.groups
}

// // AddComment adds the given comments to the fields.
// // If line is true the comment is inserted at the preceding token.

func (c *comments) AddComment(cg *CommentGroup) {
	if cg == nil {
		return
	}
	if c.groups == nil {
		a := []*CommentGroup{cg}
		c.groups = &a
		return
	}

	*c.groups = append(*c.groups, cg)
	a := *c.groups
	for i := len(a) - 2; i >= 0 && a[i].Position > cg.Position; i-- {
		a[i], a[i+1] = a[i+1], a[i]
	}
}

func (c *comments) SetComments(cgs []*CommentGroup) {
	if c.groups == nil {
		a := cgs
		c.groups = &a
		return
	}
	*c.groups = cgs
}

// A Comment node represents a single //-style
type Comment struct {
	Slash token.Pos // position of "/" starting the comment
	Text  string    // comment text (excluding '\n' for //-style comments)
}

func (c *Comment) Comments() []*CommentGroup { return nil }
func (c *Comment) AddComment(*CommentGroup)  {}
func (c *Comment) commentInfo() *comments    { return nil }
func (c *Comment) Pos() token.Pos            { return c.Slash }
func (c *Comment) pos() *token.Pos           { return &c.Slash }
func (c *Comment) End() token.Pos            { return c.Slash.Add(len(c.Text)) }

// A CommentGroup represents a sequence of comments
// with no other tokens and no empty lines between.
type CommentGroup struct {
	// TODO: remove and use the token position of the first comment.
	Doc  bool
	Line bool // true if it is on the same line as the node's end pos.

	// Position indicates where a comment should be attached if a node has
	// multiple tokens. 0 means before the first token, 1 means before the
	// second, etc. For instance, for a field, the positions are:
	//    <0> Label <1> ":" <2> Expr <3> "," <4>
	Position int8
	List     []*Comment // len(List) > 0

	isDecl
}

func (g *CommentGroup) Pos() token.Pos            { return getPos(g) }
func (g *CommentGroup) pos() *token.Pos           { return g.List[0].pos() }
func (g *CommentGroup) End() token.Pos            { return g.List[len(g.List)-1].End() }
func (g *CommentGroup) Comments() []*CommentGroup { return nil }
func (g *CommentGroup) AddComment(*CommentGroup)  {}
func (g *CommentGroup) commentInfo() *comments    { return nil }

func isWhitespace(ch byte) bool { return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' }

func stripTrailingWhitespace(s string) string {
	i := len(s)
	for i > 0 && isWhitespace(s[i-1]) {
		i--
	}
	return s[0:i]
}

// Text returns the text of the comment.
// Comment markers (//, /*, and */), the first space of a line comment, and
// leading and trailing empty lines are removed. Multiple empty lines are
// reduced to one, and trailing space on lines is trimmed. Unless the result
// is empty, it is newline-terminated.
func (g *CommentGroup) Text() string {
	if g == nil {
		return ""
	}
	comments := make([]string, len(g.List))
	for i, c := range g.List {
		comments[i] = c.Text
	}

	lines := make([]string, 0, 10) // most comments are less than 10 lines
	for _, c := range comments {
		// Remove comment markers.
		// The parser has given us exactly the comment text.
		switch c[1] {
		case '/':
			//-style comment (no newline at the end)
			c = c[2:]
			// strip first space - required for Example tests
			if len(c) > 0 && c[0] == ' ' {
				c = c[1:]
			}
		case '*':
			/*-style comment */
			c = c[2 : len(c)-2]
		}

		// Split on newlines.
		cl := strings.Split(c, "\n")

		// Walk lines, stripping trailing white space and adding to list.
		for _, l := range cl {
			lines = append(lines, stripTrailingWhitespace(l))
		}
	}

	// Remove leading blank lines; convert runs of
	// interior blank lines to a single blank line.
	n := 0
	for _, line := range lines {
		if line != "" || n > 0 && lines[n-1] != "" {
			lines[n] = line
			n++
		}
	}
	lines = lines[0:n]

	// Add final "" entry to get trailing newline from Join.
	if n > 0 && lines[n-1] != "" {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// A Field represents a field declaration in a struct.
type Field struct {
	Label      Label
	Constraint token.Token // token.ILLEGAL (no constraint), token.OPTION
	Colon      token.Pos
	// Match is set to the position of the match token if this is a regexp matching field and Label will be a string
	// token that should be interpreted as a regexp
	Match token.Pos
	Value Expr

	comments
	isDecl
}

func (d *Field) Pos() token.Pos  { return d.Label.Pos() }
func (d *Field) pos() *token.Pos { return d.Label.pos() }
func (d *Field) End() token.Pos {
	return d.Value.End()
}

// ----------------------------------------------------------------------------
// Expressions and types
//
// An expression is represented by a tree consisting of one
// or more of the following concrete expression nodes.

// A BadExpr node is a placeholder for expressions containing
// syntax errors for which no correct expression nodes can be
// created.
type BadExpr struct {
	From, To token.Pos // position range of bad expression

	comments
	isExpr
}

func (x *BadExpr) Pos() token.Pos  { return x.From }
func (x *BadExpr) pos() *token.Pos { return &x.From }
func (x *BadExpr) End() token.Pos  { return x.To }

// An Ident node represents an left-hand side identifier,
type Ident struct {
	NamePos token.Pos // identifier position

	Name string

	comments
	isLabel
	isExpr
}

func (x *Ident) Pos() token.Pos  { return x.NamePos }
func (x *Ident) pos() *token.Pos { return &x.NamePos }
func (x *Ident) End() token.Pos  { return x.NamePos.Add(len(x.Name)) }

// A BasicLit node represents a literal of basic type.
type BasicLit struct {
	ValuePos token.Pos   // literal position
	Kind     token.Token // INT, FLOAT, or STRING
	Value    string      // literal string; e.g. 42, 0x7f, 3.14, 1_234_567, 1e-9, 2.4i, 'a', '\x7f', "foo", or '\m\n\o'

	comments
	isExpr
	isLabel
}

func (x *BasicLit) Pos() token.Pos  { return x.ValuePos }
func (x *BasicLit) pos() *token.Pos { return &x.ValuePos }
func (x *BasicLit) End() token.Pos  { return x.ValuePos.Add(len(x.Value)) }

type Interpolation struct {
	Elts []Expr // interleaving of strings and expressions.

	comments
	isExpr
	isLabel
}

func (x *Interpolation) Pos() token.Pos  { return x.Elts[0].Pos() }
func (x *Interpolation) pos() *token.Pos { return x.Elts[0].pos() }
func (x *Interpolation) End() token.Pos  { return x.Elts[len(x.Elts)-1].Pos() }

// A Func node represents a function expression.
type Func struct {
	Func token.Pos // position of "function"
	Body *StructLit

	comments
	isExpr
}

func (x *Func) Pos() token.Pos  { return x.Func }
func (x *Func) pos() *token.Pos { return &x.Func }
func (x *Func) End() token.Pos  { return x.Body.End() }

// A Lambda node represents a lambda function expression.
type Lambda struct {
	Lambda token.Pos // position of "lambda"
	Colon  token.Pos // position of "colon"
	Idents []*Ident
	Expr   Expr

	comments
	isExpr
}

func (x *Lambda) Pos() token.Pos  { return x.Lambda }
func (x *Lambda) pos() *token.Pos { return &x.Lambda }
func (x *Lambda) End() token.Pos  { return x.Expr.End() }

// A StructLit node represents a literal struct.
type StructLit struct {
	Lbrace token.Pos // position of "{"
	Elts   []Decl    // list of elements; or nil
	Rbrace token.Pos // position of "}"

	comments
	isExpr
}

func (x *StructLit) Pos() token.Pos { return getPos(x) }
func (x *StructLit) pos() *token.Pos {
	if x.Lbrace == token.NoPos && len(x.Elts) > 0 {
		return x.Elts[0].pos()
	}
	return &x.Lbrace
}
func (x *StructLit) End() token.Pos {
	if x.Rbrace == token.NoPos && len(x.Elts) > 0 {
		return x.Elts[len(x.Elts)-1].End()
	}
	return x.Rbrace.Add(1)
}

// A SchemaLit node represents a literal schema definition.
type SchemaLit struct {
	Schema token.Pos // position of token.SCHEMA
	Struct *StructLit

	comments
	isExpr
}

func (x *SchemaLit) Pos() token.Pos { return getPos(x) }
func (x *SchemaLit) pos() *token.Pos {
	return &x.Struct.Lbrace
}
func (x *SchemaLit) End() token.Pos {
	return x.Struct.End()
}

// A ListLit node represents a literal list.
type ListLit struct {
	Lbrack token.Pos // position of "["
	Elts   []Expr    // list of composite elements; or nil
	Rbrack token.Pos // position of "]"

	comments
	isExpr
}

func (x *ListLit) Pos() token.Pos  { return x.Lbrack }
func (x *ListLit) pos() *token.Pos { return &x.Lbrack }
func (x *ListLit) End() token.Pos  { return x.Rbrack.Add(1) }

// A ListComprehension node represents literal list with and embedded for expression
type ListComprehension struct {
	Lbrack token.Pos // position of "["
	For    token.Pos
	Rbrack token.Pos // position of "]"
	Clause *ForClause
	Value  Expr

	comments
	isExpr
}

func (x *ListComprehension) Pos() token.Pos  { return x.Lbrack }
func (x *ListComprehension) pos() *token.Pos { return &x.Lbrack }
func (x *ListComprehension) End() token.Pos  { return x.Rbrack.Add(1) }

// A For node represents a for expression
type For struct {
	For    token.Pos
	Clause *ForClause
	Struct *StructLit

	comments
	isExpr
}

func (x *For) Pos() token.Pos  { return x.For }
func (x *For) pos() *token.Pos { return &x.For }
func (x *For) End() token.Pos  { return x.Struct.End() }

// A ForClause node represents a for clause in a comprehension.
type ForClause struct {
	Key    *Ident
	Comma  token.Pos
	Value  *Ident
	In     token.Pos
	Source Expr

	comments
	isClause
}

func (x *ForClause) Pos() token.Pos {
	if x.Key == nil {
		return x.Value.Pos()
	}
	return x.Key.Pos()
}
func (x *ForClause) pos() *token.Pos {
	if x.Key == nil {
		return x.Value.pos()
	}
	return x.Key.pos()
}
func (x *ForClause) End() token.Pos { return x.Source.End() }

// A If node represents an if expression
type If struct {
	If        token.Pos
	Condition *IfClause
	Struct    *StructLit
	Else      *Else

	comments
	isExpr
}

func (x *If) Pos() token.Pos  { return x.If }
func (x *If) pos() *token.Pos { return &x.If }
func (x *If) End() token.Pos  { return x.Struct.End() }

// An Else node represents an else or else if expression after an if expression
type Else struct {
	Else   token.Pos
	If     *If
	Struct *StructLit

	comments
	isExpr
}

func (x *Else) Pos() token.Pos  { return x.Else }
func (x *Else) pos() *token.Pos { return &x.Else }
func (x *Else) End() token.Pos {
	if x.If != nil {
		return x.If.End()
	}
	return x.Struct.End()
}

// A IfClause node represents an if guard clause in a comprehension.
type IfClause struct {
	Condition Expr

	comments
	isClause
}

func (x *IfClause) Pos() token.Pos  { return x.Condition.Pos() }
func (x *IfClause) pos() *token.Pos { return x.Condition.pos() }
func (x *IfClause) End() token.Pos  { return x.Condition.End() }

// A LetClause node represents a let clause in a comprehension.
type LetClause struct {
	Let   token.Pos
	Ident *Ident
	Colon token.Pos
	Expr  Expr

	comments
	isClause
	isDecl
}

func (x *LetClause) Pos() token.Pos  { return x.Let }
func (x *LetClause) pos() *token.Pos { return &x.Let }
func (x *LetClause) End() token.Pos  { return x.Expr.End() }

// A ParenExpr node represents a parenthesized expression.
type ParenExpr struct {
	Lparen token.Pos // position of "("
	X      Expr      // parenthesized expression
	Rparen token.Pos // position of ")"

	comments
	isExpr
}

func (x *ParenExpr) Pos() token.Pos  { return x.Lparen }
func (x *ParenExpr) pos() *token.Pos { return &x.Lparen }
func (x *ParenExpr) End() token.Pos  { return x.Rparen.Add(1) }

type DefaultExpr struct {
	Default token.Pos // position of default token
	X       Expr      // expression to apply default to

	comments
	isExpr
}

func (x *DefaultExpr) Pos() token.Pos  { return x.Default }
func (x *DefaultExpr) pos() *token.Pos { return &x.Default }
func (x *DefaultExpr) End() token.Pos  { return x.X.End() }

// A SelectorExpr node represents an expression followed by a selector.
type SelectorExpr struct {
	X   Expr  // expression
	Sel Label // field selector

	comments
	isExpr
}

func (x *SelectorExpr) Pos() token.Pos  { return x.X.Pos() }
func (x *SelectorExpr) pos() *token.Pos { return x.X.pos() }
func (x *SelectorExpr) End() token.Pos  { return x.Sel.End() }

// An IndexExpr node represents an expression followed by an index.
type IndexExpr struct {
	X      Expr      // expression
	Lbrack token.Pos // position of "["
	Index  Expr      // index expression
	Rbrack token.Pos // position of "]"

	comments
	isExpr
}

func (x *IndexExpr) Pos() token.Pos  { return x.X.Pos() }
func (x *IndexExpr) pos() *token.Pos { return x.X.pos() }
func (x *IndexExpr) End() token.Pos  { return x.Rbrack.Add(1) }

// An SliceExpr node represents an expression followed by slice indices.
type SliceExpr struct {
	X      Expr      // expression
	Lbrack token.Pos // position of "["
	Low    Expr      // begin of slice range; or nil
	High   Expr      // end of slice range; or nil
	Rbrack token.Pos // position of "]"

	comments
	isExpr
}

func (x *SliceExpr) Pos() token.Pos  { return x.X.Pos() }
func (x *SliceExpr) pos() *token.Pos { return x.X.pos() }
func (x *SliceExpr) End() token.Pos  { return x.Rbrack.Add(1) }

// A CallExpr node represents an expression followed by an argument list.
type CallExpr struct {
	Fun    Expr      // function expression
	Lparen token.Pos // position of "("
	Args   []Decl    // function arguments; or nil
	Rparen token.Pos // position of ")"

	comments
	isExpr
}

func (x *CallExpr) Pos() token.Pos  { return x.Fun.Pos() }
func (x *CallExpr) pos() *token.Pos { return x.Fun.pos() }
func (x *CallExpr) End() token.Pos  { return x.Rparen.Add(1) }

// A UnaryExpr node represents a unary expression.
type UnaryExpr struct {
	OpPos token.Pos   // position of Op
	Op    token.Token // operator
	X     Expr        // operand

	comments
	isExpr
}

func (x *UnaryExpr) Pos() token.Pos  { return x.OpPos }
func (x *UnaryExpr) pos() *token.Pos { return &x.OpPos }
func (x *UnaryExpr) End() token.Pos  { return x.X.End() }

// A BinaryExpr node represents a binary expression.
type BinaryExpr struct {
	X     Expr        // left operand
	OpPos token.Pos   // position of Op
	Op    token.Token // operator
	Y     Expr        // right operand

	comments
	isExpr
}

func (x *BinaryExpr) Pos() token.Pos  { return x.X.Pos() }
func (x *BinaryExpr) pos() *token.Pos { return x.X.pos() }
func (x *BinaryExpr) End() token.Pos  { return x.Y.End() }

// ----------------------------------------------------------------------------
// Convenience functions for Idents

func (x *Ident) String() string {
	if x != nil {
		return x.Name
	}
	return "<nil>"
}

// ----------------------------------------------------------------------------
// Declarations

// A BadDecl node is a placeholder for declarations containing
// syntax errors for which no correct declaration nodes can be
// created.
type BadDecl struct {
	From, To token.Pos // position range of bad declaration

	comments
	isDecl
}

func (d *BadDecl) Pos() token.Pos  { return d.From }
func (d *BadDecl) pos() *token.Pos { return &d.From }
func (d *BadDecl) End() token.Pos  { return d.To }

// An EmbedDecl node represents a single expression used as a declaration.
// The expressions in this declaration is what will be emitted as
// configuration output.
//
// An EmbedDecl may only appear at the top level.
type EmbedDecl struct {
	Expr Expr

	comments
	isDecl
}

func (d *EmbedDecl) Pos() token.Pos  { return d.Expr.Pos() }
func (d *EmbedDecl) pos() *token.Pos { return d.Expr.pos() }
func (d *EmbedDecl) End() token.Pos  { return d.Expr.End() }

// ----------------------------------------------------------------------------
// Files and packages

// A File node represents a Go source file.
//
// The Comments list contains all comments in the source file in order of
// appearance, including the comments that are pointed to from other nodes
// via Doc and Comment fields.
type File struct {
	Filename string
	Decls    []Decl // top-level declarations; or nil
	comments
}

func (f *File) Pos() token.Pos {
	if len(f.Decls) > 0 {
		return f.Decls[0].Pos()
	}
	return token.NoPos
}

func (f *File) pos() *token.Pos {
	if len(f.Decls) > 0 {
		return f.Decls[0].pos()
	}
	return nil
}

func (f *File) End() token.Pos {
	if n := len(f.Decls); n > 0 {
		return f.Decls[n-1].End()
	}
	return token.NoPos
}
