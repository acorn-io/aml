package cue

import (
	"io/fs"
	"path/filepath"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/load"
	cueparser "cuelang.org/go/cue/parser"
)

func AddFS(target map[string]load.Source, cwd, prependPath string, files fs.FS) error {
	return fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		data, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}

		target[filepath.Join(cwd, prependPath, path)] = load.FromBytes(data)
		return nil
	})
}

func AddFiles(target map[string]load.Source, cwd string, files ...File) error {
	for _, f := range files {
		displayName := f.DisplayName
		if displayName == "" {
			displayName = f.Filename
		}
		parser := ParserFunc(func(name string, src any) (*ast.File, error) {
			return cueparser.ParseFile(name, src, cueparser.ParseComments)
		})
		if f.Parser != nil {
			parser = f.Parser
		}
		ast, err := parser(displayName, f.Data)
		if err != nil {
			return err
		}
		target[filepath.Join(cwd, f.Filename)] = load.FromFile(ast)
	}

	return nil
}
