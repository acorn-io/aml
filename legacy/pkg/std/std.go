package std

import (
	"embed"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"
)

var (
	//go:embed std.cue
	fs      embed.FS
	Library Def
)

type Def struct {
	Imports    []*ast.ImportSpec
	Unresolved []*ast.Ident
	Decls      []ast.Decl
	Functions  map[string]bool
}

func init() {
	data, err := fs.ReadFile("std.cue")
	if err != nil {
		panic(err)
	}
	stdData, err := parser.ParseFile("std.cue", data)
	if err != nil {
		panic(err)
	}
	functions := map[string]bool{}
	for _, e := range stdData.Decls[1].(*ast.LetClause).Expr.(*ast.StructLit).Elts {
		functions[e.(*ast.Field).Label.(*ast.Ident).Name] = true
	}

	Library.Imports = stdData.Imports
	Library.Unresolved = stdData.Unresolved
	Library.Decls = stdData.Decls
	Library.Functions = functions
}
