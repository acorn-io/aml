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

// This file contains the exported entry points for invoking the

package parser

import (
	"io"

	"github.com/acorn-io/aml/pkg/ast"
	"github.com/acorn-io/aml/pkg/errors"
	"github.com/acorn-io/aml/pkg/parser/filemap"
	"github.com/acorn-io/aml/pkg/token"
)

// Option specifies a parse option.
type Option func(p *parser)

var (
	// Trace causes parsing to print a trace of parsed productions.
	Trace    Option = traceOpt
	traceOpt        = func(p *parser) {
		p.mode |= traceMode
	}
)

// A mode value is a set of flags (or 0).
// They control the amount of source code parsed and other optional
// parser functionality.
type mode uint

const (
	parseCommentsMode mode = 1 << iota // parse comments and add them to AST
	traceMode                          // print a trace of parsed productions
)

func ParseFile(filename string, src io.Reader, mode ...Option) (retFile *ast.File, retErr error) {
	text, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	var (
		pp     parser
		result = ast.File{
			Filename: filename,
		}
	)

	defer func() {
		if pp.panicking {
			_ = recover()
		}

		// set result values
		if retFile == nil {
			retFile = &ast.File{}
		}

		retErr = errors.SanitizeParserErrors(retErr)
	}()

	fileMap, err := filemap.FromBytes(filename, text)
	if err != nil {
		return nil, err
	}

	for _, entry := range fileMap.Files() {
		// reset parser
		pp = parser{}

		// parse source
		pp.init(entry.Filename, entry.Data, mode)
		f := pp.parseFile()
		if f == nil {
			return nil, errors.Join(pp.errors...)
		}

		if len(pp.errors) > 0 {
			return nil, errors.Join(pp.errors...)
		}

		result.Decls = append(result.Decls, f.Decls...)
		result.SetComments(append(result.Comments(), f.Comments()...))
	}

	return &result, nil
}

func ParseExpr(filename string, src io.Reader, mode ...Option) (_ ast.Expr, retErr error) {
	text, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	var p parser
	defer func() {
		if p.panicking {
			_ = recover()
		}
		retErr = errors.SanitizeParserErrors(retErr)
	}()

	// parse expr
	p.init(filename, text, mode)
	// Set up pkg-level scopes to avoid nil-pointer errors.
	// This is not needed for a correct expression x as the
	// parser will be ok with a nil topScope, but be cautious
	// in case of an erroneous x.
	e := p.parseRHS()

	// If a comma was inserted, consume it;
	// report an error if there's more tokens.
	if p.tok == token.COMMA && p.lit == "\n" {
		p.next()
	}

	p.expect(token.EOF)
	return e, errors.Join(p.errors...)
}
