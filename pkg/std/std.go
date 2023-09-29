package std

import (
	"embed"

	"github.com/acorn-io/aml/pkg/ast"
	"github.com/acorn-io/aml/pkg/parser"
)

var (
	//go:embed std.acorn
	fs   embed.FS
	File *ast.File
)

const stdFile = "std.acorn"

func init() {
	f, err := fs.Open(stdFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	File, err = parser.ParseFile(stdFile, f)
	if err != nil {
		panic(err)
	}
}
