package parser

import (
	"fmt"

	"github.com/acorn-io/aml/pkg/ast"
	"github.com/acorn-io/aml/pkg/errors"
	"github.com/acorn-io/aml/pkg/scanner"
	"github.com/acorn-io/aml/pkg/token"
)

// The parser structure holds the parser's internal state.
type parser struct {
	file    *token.File
	offset  int
	errors  []error
	scanner scanner.Scanner

	// Tracing/debugging
	mode      mode // parsing mode
	trace     bool // == (mode & Trace != 0)
	panicking bool // set if we are bailing out due to too many errors.
	indent    int  // indentation used for tracing output

	// Comments
	leadComment *ast.CommentGroup
	comments    *commentState

	// Next token
	pos token.Pos   // token position
	tok token.Token // one token look-ahead
	lit string      // token literal

	// Error recovery
	// (used to limit the number of calls to syncXXX functions
	// w/o making scanning progress - avoids potential endless
	// loops across multiple parser functions during error recovery)
	syncPos token.Pos // last synchronization position
	syncCnt int       // number of calls to syncXXX without progress

	version int
}

func (p *parser) init(filename string, src []byte, mode []Option) {
	p.mode = parseCommentsMode
	p.offset = -1
	for _, f := range mode {
		f(p)
	}
	p.file = token.NewFile(filename, p.offset, len(src))

	var m scanner.Mode
	if p.mode&parseCommentsMode != 0 {
		m = scanner.ScanComments
	}
	eh := func(pos token.Pos, msg string, args []interface{}) {
		p.errors = append(p.errors, errors.NewParserError(pos, msg, args...))
	}
	p.scanner.Init(p.file, src, eh, m)

	p.trace = p.mode&traceMode != 0 // for convenience (p.trace is used frequently)

	p.comments = &commentState{pos: -1}

	p.next()
}

type commentState struct {
	parent *commentState
	pos    int8
	groups []*ast.CommentGroup

	// lists are not attached to nodes themselves. Enclosed expressions may
	// miss a comment due to commas and line termination. closeLists ensures
	// that comments will be passed to someone.
	isList    int
	lastChild ast.Node
	lastPos   int8
}

// openComments reserves the next doc comment for the caller and flushes
func (p *parser) openComments() *commentState {
	child := &commentState{
		parent: p.comments,
	}
	if c := p.comments; c != nil && c.isList > 0 {
		if c.lastChild != nil {
			var groups []*ast.CommentGroup
			for _, cg := range c.groups {
				if cg.Position == 0 {
					groups = append(groups, cg)
				}
			}
			groups = append(groups, ast.Comments(c.lastChild)...)
			for _, cg := range c.groups {
				if cg.Position != 0 {
					cg.Position = c.lastPos
					groups = append(groups, cg)
				}
			}
			ast.SetComments(c.lastChild, groups)
			c.groups = nil
		} else {
			c.lastChild = nil
			// attach before next
			for _, cg := range c.groups {
				cg.Position = 0
			}
			child.groups = c.groups
			c.groups = nil
		}
	}
	if p.leadComment != nil {
		child.groups = append(child.groups, p.leadComment)
		p.leadComment = nil
	}
	p.comments = child
	return child
}

// openList is used to treat a list of comments as a single comment
// position in a production.
func (p *parser) openList() {
	if p.comments.isList > 0 {
		p.comments.isList++
		return
	}
	c := &commentState{
		parent: p.comments,
		isList: 1,
	}
	p.comments = c
}

func (c *commentState) add(g *ast.CommentGroup) {
	g.Position = c.pos
	c.groups = append(c.groups, g)
}

func (p *parser) closeList() {
	c := p.comments
	if c.lastChild != nil {
		for _, cg := range c.groups {
			cg.Position = c.lastPos
			ast.AddComment(c.lastChild, cg)
		}
		c.groups = nil
	}
	switch c.isList--; {
	case c.isList < 0:
		if !p.panicking {
			err := errors.NewParserError(p.pos, "unmatched close list")
			p.errors = append(p.errors, err)
			p.panicking = true
			panic(err)
		}
	case c.isList == 0:
		parent := c.parent
		if len(c.groups) > 0 {
			parent.groups = append(parent.groups, c.groups...)
		}
		parent.pos++
		p.comments = parent
	}
}

func (c *commentState) closeNode(p *parser, n ast.Node) ast.Node {
	if p.comments != c {
		if !p.panicking {
			err := errors.NewParserError(p.pos, "unmatched comments")
			p.errors = append(p.errors, err)
			p.panicking = true
			panic(err)
		}
		return n
	}
	p.comments = c.parent
	if c.parent != nil {
		c.parent.lastChild = n
		c.parent.lastPos = c.pos
		c.parent.pos++
	}
	for _, cg := range c.groups {
		if n != nil {
			if cg != nil {
				ast.AddComment(n, cg)
			}
		}
	}
	c.groups = nil
	return n
}

func (c *commentState) closeExpr(p *parser, n ast.Expr) ast.Expr {
	c.closeNode(p, n)
	return n
}

// ----------------------------------------------------------------------------
// Parsing support

func (p *parser) printTrace(a ...interface{}) {
	const dots = ". . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . "
	const n = len(dots)
	pos := p.file.Position(p.pos)
	fmt.Printf("%5d:%3d: ", pos.Line, pos.Column)
	i := 2 * p.indent
	for i > n {
		fmt.Print(dots)
		i -= n
	}
	// i <= n
	fmt.Print(dots[0:i])
	fmt.Println(a...)
}

func trace(p *parser, msg string) *parser {
	p.printTrace(msg, "(")
	p.indent++
	return p
}

// Usage pattern: defer un(trace(p, "..."))
func un(p *parser) {
	p.indent--
	p.printTrace(")")
}

// Advance to the next
func (p *parser) next0() {
	// Because of one-token look-ahead, print the previous token
	// when tracing as it provides a more readable output. The
	// very first token (!p.pos.IsValid()) is not initialized
	// (it is ILLEGAL), so don't print it .
	if p.trace && p.pos.IsValid() {
		s := p.tok.String()
		switch {
		case p.tok.IsLiteral():
			p.printTrace(s, p.lit)
		case p.tok.IsOperator(), p.tok.IsKeyword():
			p.printTrace("\"" + s + "\"")
		default:
			p.printTrace(s)
		}
	}

	p.pos, p.tok, p.lit = p.scanner.Scan()
}

// Consume a comment and return it and the line on which it ends.
func (p *parser) consumeComment() (comment *ast.Comment, endline int) {
	endline = p.file.Line(p.pos)
	comment = &ast.Comment{Slash: p.pos, Text: p.lit}
	p.next0()

	return
}

// Consume a group of adjacent comments, add it to the parser's
// comments list, and return it together with the line at which
// the last comment in the group ends. A non-comment token or n
// empty lines terminate a comment group.
func (p *parser) consumeCommentGroup(prevLine, n int) (comments *ast.CommentGroup, endline int) {
	var list []*ast.Comment
	var rel token.RelPos
	endline = p.file.Line(p.pos)
	switch endline - prevLine {
	case 0:
		rel = token.Blank
	case 1:
		rel = token.Newline
	default:
		rel = token.NewSection
	}
	for p.tok == token.COMMENT && p.file.Line(p.pos) <= endline+n {
		var comment *ast.Comment
		comment, endline = p.consumeComment()
		list = append(list, comment)
	}

	cg := &ast.CommentGroup{List: list}
	ast.SetRelPos(cg, rel)
	comments = cg
	return
}

// Advance to the next non-comment  In the process, collect
// any comment groups encountered, and refield the last lead and
// line comments.
//
// A lead comment is a comment group that starts and ends in a
// line without any other tokens and that is followed by a non-comment
// token on the line immediately after the comment group.
//
// A line comment is a comment group that follows a non-comment
// token on the same line, and that has no tokens after it on the line
// where it ends.
//
// Lead and line comments may be considered documentation that is
// stored in the AST.
func (p *parser) next() {
	// A leadComment may not be consumed if it leads an inner token of a node.
	if p.leadComment != nil {
		p.comments.add(p.leadComment)
	}
	p.leadComment = nil
	prev := p.pos
	p.next0()
	p.comments.pos++

	if p.tok == token.COMMENT {
		var comment *ast.CommentGroup
		var endline int

		currentLine := p.file.Line(p.pos)
		prevLine := p.file.Line(prev)
		if prevLine == currentLine {
			// The comment is on same line as the previous token; it
			// cannot be a lead comment but may be a line comment.
			comment, endline = p.consumeCommentGroup(prevLine, 0)
			if p.file.Line(p.pos) != endline {
				// The next token is on a different line, thus
				// the last comment group is a line comment.
				comment.Line = true
			}
		}

		// consume successor comments, if any
		endline = -1
		for p.tok == token.COMMENT {
			if comment != nil {
				p.comments.add(comment)
			}
			comment, endline = p.consumeCommentGroup(prevLine, 1)
			prevLine = currentLine
			currentLine = p.file.Line(p.pos)

		}

		if endline+1 == p.file.Line(p.pos) && p.tok != token.EOF {
			// The next token is following on the line immediately after the
			// comment group, thus the last comment group is a lead comment.
			comment.Doc = true
			p.leadComment = comment
		} else {
			p.comments.add(comment)
		}
	}
}

func (p *parser) errf(pos token.Pos, msg string, args ...interface{}) {
	ePos := pos
	p.errors = append(p.errors, errors.NewParserError(ePos, msg, args...))
}

func (p *parser) errorExpected(pos token.Pos, obj string) {
	if pos != p.pos {
		p.errf(pos, "expected %s", obj)
		return
	}
	// the error happened at the current position;
	// make the error message more specific
	if p.tok == token.COMMA && p.lit == "\n" {
		p.errf(pos, "expected %s, found newline", obj)
		return
	}

	if p.tok.IsLiteral() {
		p.errf(pos, "expected %s, found '%s' %s", obj, p.tok, p.lit)
	} else {
		p.errf(pos, "expected %s, found '%s'", obj, p.tok)
	}
}

func (p *parser) expect(tok token.Token) token.Pos {
	pos := p.pos
	if p.tok != tok {
		p.errorExpected(pos, "'"+tok.String()+"'")
	}
	p.next() // make progress
	return pos
}

// expectClosing is like expect but provides a better error message
// for the common case of a missing comma before a newline.
func (p *parser) expectClosing(tok token.Token, context string) token.Pos {
	if p.tok != tok && p.tok == token.COMMA && p.lit == "\n" {
		p.errf(p.pos, "missing ',' before newline in %s", context)
		p.next()
	}
	return p.expect(tok)
}

func (p *parser) expectComma() {
	// semicolon is optional before a closing ')', ']', '}', or newline
	if p.tok != token.RPAREN && p.tok != token.RBRACE && p.tok != token.EOF {
		switch p.tok {
		case token.COMMA:
			p.next()
		default:
			p.errorExpected(p.pos, "','")
			syncExpr(p)
		}
	}
}

func (p *parser) atComma(context string, follow ...token.Token) bool {
	if p.tok == token.COMMA {
		return true
	}
	for _, t := range follow {
		if p.tok == t {
			return false
		}
	}
	// TODO: find a way to detect crossing lines now we don't have a semi.
	if p.lit == "\n" {
		p.errf(p.pos, "missing ',' before newline")
	} else {
		p.errf(p.pos, "missing ',' in %s", context)
	}
	return true // "insert" comma and continue
}

// syncExpr advances to the next field in a field list.
// Used for synchronization after an error.
func syncExpr(p *parser) {
	for {
		switch p.tok {
		case token.COMMA:
			// Return only if parser made some progress since last
			// sync or if it has not reached 10 sync calls without
			// progress. Otherwise consume at least one token to
			// avoid an endless parser loop (it is possible that
			// both parseOperand and parseStmt call syncStmt and
			// correctly do not advance, thus the need for the
			// invocation limit p.syncCnt).
			if p.pos == p.syncPos && p.syncCnt < 10 {
				p.syncCnt++
				return
			}
			if p.syncPos.Before(p.pos) {
				p.syncPos = p.pos
				p.syncCnt = 0
				return
			}
			// Reaching here indicates a parser bug, likely an
			// incorrect token list in this function, but it only
			// leads to skipping of possibly correct code if a
			// previous error is present, and thus is preferred
			// over a non-terminating parse.
		case token.EOF:
			return
		}
		p.next()
	}
}

// safePos returns a valid file position for a given position: If pos
// is valid to begin with, safePos returns pos. If pos is out-of-range,
// safePos returns the EOF position.
//
// This is hack to work around "artificial" end positions in the AST which
// are computed by adding 1 to (presumably valid) token positions. If the
// token positions are invalid due to parse errors, the resulting end position
// may be past the file's EOF position, which would lead to panics if used
// later on.
func (p *parser) safePos(pos token.Pos) (res token.Pos) {
	defer func() {
		if recover() != nil {
			res = p.file.Pos(p.file.Base()+p.file.Size(), pos.RelPos()) // EOF position
		}
	}()
	_ = p.file.Offset(pos) // trigger a panic if position is out-of-range
	return pos
}

// ----------------------------------------------------------------------------
// Identifiers

func (p *parser) parseIdent() *ast.Ident {
	c := p.openComments()
	pos := p.pos
	name := p.lit
	p.expect(token.IDENT) // use expect() error handling
	ident := &ast.Ident{NamePos: pos, Name: name}
	c.closeNode(p, ident)
	return ident
}

// ----------------------------------------------------------------------------
// Expressions

func (p *parser) parseParens() (expr *ast.ParenExpr) {
	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	lparen := p.pos
	p.next()
	p.openList()
	x := p.parseRHS() // types may be parenthesized: (some type)
	p.closeList()
	rparen := p.expect(token.RPAREN)
	return &ast.ParenExpr{
		Lparen: lparen,
		X:      x,
		Rparen: rparen}
}

func (p *parser) parseLiteral() (lit *ast.BasicLit) {
	if p.trace {
		defer un(trace(p, "Literal"))
	}

	c := p.openComments()
	defer func() { c.closeNode(p, lit) }()

	lit = &ast.BasicLit{
		ValuePos: p.pos,
		Kind:     p.tok,
		Value:    p.lit,
	}

	p.tok.IsLiteral()
	switch p.tok {
	case token.NULL, token.TRUE, token.FALSE, token.NUMBER, token.STRING:
	default:
		p.errf(p.pos, "expected literal but got %s", p.tok)
	}

	p.next()
	return
}

func (p *parser) badExpr(fromPos token.Pos) (expr ast.Expr) {
	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	p.errf(fromPos, "invalid expression")
	return &ast.BadExpr{From: fromPos, To: p.pos}
}

func (p *parser) parseDefault() (expr ast.Expr) {
	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	pos := p.expect(token.DEFAULT)
	if p.tok == token.COLON {
		return &ast.Ident{NamePos: pos, Name: token.DEFAULT.String()}
	}
	x := p.parseExpr()

	return &ast.DefaultExpr{
		Default: pos,
		X:       x,
	}
}

// parseOperand returns an expression.
// Callers must verify the result.
func (p *parser) parseOperand() (expr ast.Expr) {
	if p.trace {
		defer un(trace(p, "Operand"))
	}

	switch p.tok {
	case token.IDENT:
		return p.parseIdent()

	case token.LBRACE:
		return p.parseStruct()

	case token.LBRACK:
		return p.parseList()

	case token.FUNCTION:
		return p.parseFunc()

	case token.LAMBDA:
		return p.parseLambda()

	case token.SCHEMA:
		return p.parseSchema()

	case token.DEFAULT:
		return p.parseDefault()

	case token.NULL, token.TRUE, token.FALSE, token.NUMBER, token.STRING:
		return p.parseLiteral()

	case token.INTERPOLATION:
		return p.parseInterpolation()

	case token.LPAREN:
		return p.parseParens()

	case token.FOR:
		return p.parseFor()

	case token.IF:
		return p.parseIf()
	}

	return p.badExpr(p.pos)
}

func (p *parser) parseIndexOrSlice(x ast.Expr) (expr ast.Expr) {
	if p.trace {
		defer un(trace(p, "IndexOrSlice"))
	}

	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()
	c.pos = 1

	const N = 2
	lbrack := p.expect(token.LBRACK)

	var index [N]ast.Expr
	var colons [N - 1]token.Pos
	if p.tok != token.COLON {
		index[0] = p.parseRHS()
	}
	nColons := 0
	for p.tok == token.COLON && nColons < len(colons) {
		colons[nColons] = p.pos
		nColons++
		p.next()
		if p.tok != token.COLON && p.tok != token.RBRACK && p.tok != token.EOF {
			index[nColons] = p.parseRHS()
		}
	}
	rbrack := p.expect(token.RBRACK)

	if nColons > 0 {
		return &ast.SliceExpr{
			X:      x,
			Lbrack: lbrack,
			Low:    index[0],
			High:   index[1],
			Rbrack: rbrack}
	}

	return &ast.IndexExpr{
		X:      x,
		Lbrack: lbrack,
		Index:  index[0],
		Rbrack: rbrack}
}

func (p *parser) parseCall(fun ast.Expr) (expr *ast.CallExpr) {
	if p.trace {
		defer un(trace(p, "Call"))
	}
	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	p.openList()
	defer p.closeList()

	lparen := p.expect(token.LPAREN)

	args := p.parseCallArgs()
	rparen := p.expectClosing(token.RPAREN, "argument list")

	return &ast.CallExpr{
		Fun:    fun,
		Lparen: lparen,
		Args:   args,
		Rparen: rparen}
}

func (p *parser) parseCallArgs() (list []ast.Decl) {
	if p.trace {
		defer un(trace(p, "CallArgs"))
	}
	p.openList()
	defer p.closeList()

	for p.tok != token.RPAREN && p.tok != token.EOF {
		list = append(list, p.parseCallArg())
	}

	return
}

func (p *parser) parseFieldList() (list []ast.Decl) {
	if p.trace {
		defer un(trace(p, "FieldList"))
	}
	p.openList()
	defer p.closeList()

	for p.tok != token.RBRACE && p.tok != token.EOF {
		list = append(list, p.parseDecl())
	}

	return
}

func (p *parser) parseLetDecl() (decl ast.Decl) {
	if p.trace {
		defer un(trace(p, "Let"))
	}

	c := p.openComments()
	defer func() { c.closeNode(p, decl) }()

	letPos := p.expect(token.LET)
	ident := p.parseIdent()
	assign := p.expect(token.COLON)
	expr := p.parseRHS()

	return &ast.LetClause{
		Let:   letPos,
		Ident: ident,
		Colon: assign,
		Expr:  expr,
	}
}

func (p *parser) parseElse() (expr *ast.Else) {
	if p.trace {
		defer un(trace(p, "Else"))
	}

	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	var (
		elsePos   = p.expect(token.ELSE)
		ifExpr    *ast.If
		structLit *ast.StructLit
	)

	switch p.tok {
	case token.IF:
		ifExpr = p.parseIf()
	case token.LBRACE:
		structLit = p.parseStruct()
	default:
		p.errf(p.pos, "expected %s or %s after an %s", token.IF, token.LBRACE, token.ELSE)
		structLit = &ast.StructLit{}
	}

	return &ast.Else{
		Else:   elsePos,
		If:     ifExpr,
		Struct: structLit,
	}
}

func (p *parser) parseIf() (expr *ast.If) {
	if p.trace {
		defer un(trace(p, "If"))
	}

	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	var (
		ifPos      = p.expect(token.IF)
		clause     = p.parseIfClause()
		structExpr = p.parseStruct()
		elif       *ast.Else
	)

	if p.tok == token.ELSE {
		elif = p.parseElse()
	}

	return &ast.If{
		If:        ifPos,
		Else:      elif,
		Condition: clause,
		Struct:    structExpr,
	}
}

func (p *parser) parseListComprehensionBody() (expr *ast.ListComprehension) {
	if p.trace {
		defer un(trace(p, "ListComprehension"))
	}

	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	forPos := p.expect(token.FOR)
	clause := p.parseForClause()
	body := p.parseExpr()

	return &ast.ListComprehension{
		For:    forPos,
		Clause: clause,
		Value:  body,
	}
}

func (p *parser) parseFor() (expr *ast.For) {
	if p.trace {
		defer un(trace(p, "For"))
	}

	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	forPos := p.expect(token.FOR)
	clause := p.parseForClause()
	structExpr := p.parseStruct()

	return &ast.For{
		For:    forPos,
		Clause: clause,
		Struct: structExpr,
	}
}

func (p *parser) checkAndParseValidLabel(expr ast.Expr) (matchPos token.Pos, _ ast.Label, _ bool) {
	label, ok := expr.(ast.Label)
	if !ok {
		return token.NoPos, nil, false
	}

	// Only allow string basic literals
	if basicList, ok := label.(*ast.BasicLit); ok && basicList.Kind != token.STRING {
		return token.NoPos, nil, false
	} else if ident, ok := label.(*ast.Ident); ok {
		if ident.Name == "match" {
			expr := p.parseExpr()
			if str, ok := expr.(*ast.BasicLit); ok && str.Kind == token.STRING {
				return ident.Pos(), str, true
			} else if str, ok := expr.(*ast.Interpolation); ok {
				return ident.Pos(), str, true
			}
			p.errf(ident.End(), "expected string after match")
		}
	}

	return token.NoPos, label, true
}

func (p *parser) parseCallArg() (decl ast.Decl) {
	if p.trace {
		defer un(trace(p, "CallArg"))
	}

	c := p.openComments()
	defer func() { c.closeNode(p, decl) }()
	defer func() {
		if p.atComma("func arg", token.RPAREN) {
			p.next()
		}
	}()

	return p.parseDeclInline()
}

func (p *parser) parseDecl() (decl ast.Decl) {
	if p.trace {
		defer un(trace(p, "Decl"))
	}

	c := p.openComments()
	defer func() { c.closeNode(p, decl) }()
	defer func() {
		if p.atComma("struct literal", token.RBRACE, token.EOF) {
			p.next()
		}
	}()

	return p.parseDeclInline()
}

func (p *parser) parseDeclInline() (decl ast.Decl) {
	if p.trace {
		defer un(trace(p, "Decl"))
	}

	switch p.tok {
	case token.LET:
		return p.parseLetDecl()
	}

	field := &ast.Field{}
	expr := p.parseExpr()
	if match, label, ok := p.checkAndParseValidLabel(expr); ok {
		field.Label = label
		field.Match = match
	} else {
		return &ast.EmbedDecl{
			Expr: expr,
		}
	}

	if p.tok == token.OPTION {
		// If an option is found then it must be a field at this point
		field.Constraint = token.OPTION
		p.next()
	} else if p.tok != token.COLON {
		// It's a valid label but no colon found, so it's an embedded decl
		return &ast.EmbedDecl{
			Expr: expr,
		}
	}

	field.Colon = p.pos
	// consume
	p.expect(token.COLON)

	decl = p.parseDeclInline()
	switch node := decl.(type) {
	case *ast.EmbedDecl:
		field.Value = node.Expr
	default:
		field.Value = &ast.StructLit{
			Elts: []ast.Decl{decl},
		}
	}

	return field
}

func (p *parser) parseSchema() (expr *ast.SchemaLit) {
	if p.trace {
		defer un(trace(p, "SchemaLit"))
	}

	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	schema := p.expect(token.SCHEMA)
	s := p.parseStruct()

	return &ast.SchemaLit{
		Schema: schema,
		Struct: s,
	}
}

func (p *parser) parseStruct() (expr *ast.StructLit) {
	if p.trace {
		defer un(trace(p, "StructLit"))
	}

	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	lbrace := p.expect(token.LBRACE)

	elts := p.parseStructBody()
	rbrace := p.expectClosing(token.RBRACE, "struct literal")
	return &ast.StructLit{
		Lbrace: lbrace,
		Elts:   elts,
		Rbrace: rbrace,
	}
}

func (p *parser) parseStructBody() []ast.Decl {
	if p.trace {
		defer un(trace(p, "StructBody"))
	}

	var elts []ast.Decl

	if p.tok != token.RBRACE {
		elts = p.parseFieldList()
	}

	return elts
}

func (p *parser) parseForClause() (clause *ast.ForClause) {
	c := p.openComments()
	defer func() { c.closeNode(p, clause) }()

	var (
		key, value *ast.Ident
		comma      token.Pos
	)

	value = p.parseIdent()
	if p.tok == token.COMMA {
		comma = p.expect(token.COMMA)
		key = value
		value = p.parseIdent()
	}
	c.pos = 4

	inTok := p.expect(token.IN)
	source := p.parseRHS()

	return &ast.ForClause{
		Key:    key,
		Comma:  comma,
		Value:  value,
		In:     inTok,
		Source: source,
	}
}

func (p *parser) parseIfClause() (clause *ast.IfClause) {
	c := p.openComments()
	defer func() { c.closeNode(p, clause) }()

	return &ast.IfClause{
		Condition: p.parseRHS(),
	}
}

func (p *parser) parseLambda() (expr ast.Expr) {
	if p.trace {
		defer un(trace(p, "Lambda"))
	}

	lambda := p.expect(token.LAMBDA)
	var idents []*ast.Ident

	for p.tok != token.COLON {
		idents = append(idents, p.parseIdent())
		if p.tok == token.COMMA {
			p.next()
		}
	}

	colon := p.expect(token.COLON)
	body := p.parseExpr()

	return &ast.Lambda{
		Lambda: lambda,
		Colon:  colon,
		Idents: idents,
		Expr:   body,
	}
}

func (p *parser) parseFunc() (expr ast.Expr) {
	if p.trace {
		defer un(trace(p, "Function"))
	}
	fun := p.expect(token.FUNCTION)
	body := p.parseStruct()

	return &ast.Func{
		Func: fun,
		Body: body,
	}
}

func (p *parser) parseList() (expr ast.Expr) {
	if p.trace {
		defer un(trace(p, "ListLiteral"))
	}

	lbrack := p.expect(token.LBRACK)

	if p.tok == token.FOR {
		body := p.parseListComprehensionBody()
		body.Lbrack = lbrack
		body.Rbrack = p.expect(token.RBRACK)
		return body
	}

	elts := p.parseListElements()

	rbrack := p.expectClosing(token.RBRACK, "list literal")
	return &ast.ListLit{
		Lbrack: lbrack,
		Elts:   elts,
		Rbrack: rbrack,
	}
}

func (p *parser) parseListElements() (list []ast.Expr) {
	if p.trace {
		defer un(trace(p, "ListElements"))
	}
	p.openList()
	defer p.closeList()

	for p.tok != token.RBRACK && p.tok != token.EOF {
		expr, ok := p.parseListElement()
		list = append(list, expr)
		if !ok {
			break
		}
	}

	return
}

func (p *parser) parseListElement() (expr ast.Expr, ok bool) {
	if p.trace {
		defer un(trace(p, "ListElement"))
	}
	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	expr = p.parseExpr()

	// Enforce there is an explicit comma. We could also allow the
	// omission of commas in lists, but this gives rise to some ambiguities
	// with list comprehensions.
	if p.tok == token.COMMA && p.lit != "," {
		p.next()
		// Allow missing comma for last element, though, to be compliant
		// with JSON.
		if p.tok == token.RBRACK {
			return expr, false
		}
		p.errf(p.pos, "missing ',' before newline in list literal")
	} else if !p.atComma("list literal", token.RBRACK, token.FOR, token.IF) {
		return expr, false
	}
	p.next()

	return expr, true
}

// TODO: this seems useless
// checkExpr checks that x is an expression (and not a type).
func (p *parser) checkExpr(x ast.Expr) ast.Expr {
	switch unparen(x).(type) {
	case *ast.BadExpr:
	case *ast.Ident:
	case *ast.BasicLit:
	case *ast.Interpolation:
	case *ast.Func:
	case *ast.Lambda:
	case *ast.StructLit:
	case *ast.ListLit:
	case *ast.ParenExpr:
		panic("unreachable")
	case *ast.SelectorExpr:
	case *ast.IndexExpr:
	case *ast.SliceExpr:
	case *ast.CallExpr:
	case *ast.UnaryExpr:
	case *ast.BinaryExpr:
	case *ast.ListComprehension:
	case *ast.SchemaLit:
	case *ast.DefaultExpr:
	default:
		// all other nodes are not proper expressions
		p.errorExpected(x.Pos(), "expression")
		x = &ast.BadExpr{
			From: x.Pos(), To: p.safePos(x.End()),
		}
	}
	return x
}

// If x is of the form (T), unparen returns unparen(T), otherwise it returns x.
func unparen(x ast.Expr) ast.Expr {
	if p, isParen := x.(*ast.ParenExpr); isParen {
		x = unparen(p.X)
	}
	return x
}

// If lhs is set and the result is an identifier, it is not resolved.
func (p *parser) parsePrimaryExpr() ast.Expr {
	if p.trace {
		defer un(trace(p, "PrimaryExpr"))
	}

	return p.parsePrimaryExprTail(p.parseOperand())
}

func (p *parser) parsePrimaryExprTail(operand ast.Expr) ast.Expr {
	x := operand
L:
	for {
		switch p.tok {
		case token.PERIOD:
			c := p.openComments()
			c.pos = 1
			p.next()
			switch p.tok {
			case token.IDENT:
				x = &ast.SelectorExpr{
					X:   p.checkExpr(x),
					Sel: p.parseIdent(),
				}
			default:
				pos := p.pos
				p.errorExpected(pos, "selector")
				p.next() // make progress
				x = &ast.SelectorExpr{X: x, Sel: &ast.Ident{NamePos: pos, Name: "_"}}
			}
			c.closeNode(p, x)
		case token.LBRACK:
			x = p.parseIndexOrSlice(p.checkExpr(x))
		case token.LPAREN:
			x = p.parseCall(p.checkExpr(x))
		default:
			break L
		}
	}

	return x
}

// If lhs is set and the result is an identifier, it is not resolved.
func (p *parser) parseUnaryExpr() ast.Expr {
	if p.trace {
		defer un(trace(p, "UnaryExpr"))
	}

	switch p.tok {
	case token.ADD, token.SUB, token.NOT:
		pos, op := p.pos, p.tok
		c := p.openComments()
		p.next()
		return c.closeExpr(p, &ast.UnaryExpr{
			OpPos: pos,
			Op:    op,
			X:     p.checkExpr(p.parseUnaryExpr()),
		})
	}

	return p.parsePrimaryExpr()
}

func (p *parser) tokPrec() (token.Token, int) {
	tok := p.tok
	if tok == token.IDENT {
		return tok, 0
	}
	return tok, tok.Precedence()
}

// If lhs is set and the result is an identifier, it is not resolved.
func (p *parser) parseBinaryExpr(prec1 int) ast.Expr {
	if p.trace {
		defer un(trace(p, "BinaryExpr"))
	}
	p.openList()
	defer p.closeList()

	return p.parseBinaryExprTail(prec1, p.parseUnaryExpr())
}

func (p *parser) parseBinaryExprTail(prec1 int, x ast.Expr) ast.Expr {
	for {
		op, prec := p.tokPrec()
		if prec < prec1 {
			return x
		}
		c := p.openComments()
		c.pos = 1
		pos := p.expect(p.tok)
		x = c.closeExpr(p, &ast.BinaryExpr{
			X:     p.checkExpr(x),
			OpPos: pos,
			Op:    op,
			// Treat nested expressions as RHS.
			Y: p.checkExpr(p.parseBinaryExpr(prec + 1))})
	}
}

func (p *parser) parseInterpolation() (expr ast.Expr) {
	c := p.openComments()
	defer func() { c.closeNode(p, expr) }()

	p.openList()
	defer p.closeList()

	cc := p.openComments()

	lit := p.lit
	pos := p.pos
	p.next()
	last := &ast.BasicLit{ValuePos: pos, Kind: token.STRING, Value: lit}
	exprs := []ast.Expr{last}

	for p.tok == token.LPAREN {
		c.pos = 1
		p.expect(token.LPAREN)
		cc.closeExpr(p, last)

		exprs = append(exprs, p.parseRHS())

		cc = p.openComments()
		if p.tok != token.RPAREN {
			p.errf(p.pos, "expected ')' for string interpolation")
		}
		lit = p.scanner.ResumeInterpolation()
		pos = p.pos
		p.next()
		last = &ast.BasicLit{
			ValuePos: pos,
			Kind:     token.STRING,
			Value:    lit,
		}
		exprs = append(exprs, last)
	}
	cc.closeExpr(p, last)
	return &ast.Interpolation{Elts: exprs}
}

// Callers must check the result (using checkExpr), depending on context.
func (p *parser) parseExpr() (expr ast.Expr) {
	if p.trace {
		defer un(trace(p, "Expression"))
	}

	c := p.openComments()
	defer func() { c.closeExpr(p, expr) }()

	return p.parseBinaryExpr(token.LowestPrec + 1)
}

func (p *parser) parseRHS() ast.Expr {
	x := p.checkExpr(p.parseExpr())
	return x
}

// ----------------------------------------------------------------------------
// Source files

func (p *parser) parseFile() *ast.File {
	if p.trace {
		defer un(trace(p, "File"))
	}

	c := p.comments

	if p.errors != nil {
		return nil
	}

	decls := p.parseFieldList()
	p.expect(token.EOF)

	f := &ast.File{
		Decls: decls,
	}

	c.closeNode(p, f)
	return f
}
