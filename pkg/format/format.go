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
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/acorn-io/aml/pkg/ast"
	"github.com/acorn-io/aml/pkg/parser"
	"github.com/acorn-io/aml/pkg/token"
)

// An Option sets behavior of the formatter.
type Option func(c *config)

func Node(node ast.Node, opt ...Option) ([]byte, error) {
	cfg := newConfig(opt)
	return cfg.fprint(node)
}

func Format(b io.Reader, opt ...Option) ([]byte, error) {
	f, err := parser.ParseFile("", b)
	if err != nil {
		return nil, fmt.Errorf("parse: %s", err)
	}

	return Node(f, opt...)
}

type config struct {
	UseSpaces bool
	TabIndent bool
	Tabwidth  int // default: 4
	Indent    int // default: 0 (all code is indented at least by this much)
}

func newConfig(opt []Option) *config {
	cfg := &config{
		Tabwidth:  8,
		TabIndent: true,
		UseSpaces: true,
	}
	for _, o := range opt {
		o(cfg)
	}
	return cfg
}

// Config defines the output of Fprint.
func (cfg *config) fprint(node interface{}) (out []byte, err error) {
	var p printer
	p.init(cfg)
	if err = printNode(node, &p); err != nil {
		return p.output, err
	}

	padchar := byte('\t')
	if cfg.UseSpaces {
		padchar = byte(' ')
	}

	twmode := tabwriter.StripEscape | tabwriter.TabIndent | tabwriter.DiscardEmptyColumns
	if cfg.TabIndent {
		twmode |= tabwriter.TabIndent
	}

	buf := &bytes.Buffer{}
	tw := tabwriter.NewWriter(buf, 0, cfg.Tabwidth, 1, padchar, twmode)

	// write printer result via tabwriter/trimmer to output
	if _, err = tw.Write(p.output); err != nil {
		return
	}

	err = tw.Flush()
	if err != nil {
		return buf.Bytes(), err
	}

	b := buf.Bytes()
	if !cfg.TabIndent {
		b = bytes.ReplaceAll(b, []byte{'\t'}, bytes.Repeat([]byte{' '}, cfg.Tabwidth))
	}
	return b, nil
}

// A formatter walks a syntax.Node, interspersed with comments and spacing
// directives, in the order that they would occur in printed form.
type formatter struct {
	*printer

	stack    []frame
	current  frame
	nestExpr int
}

func newFormatter(p *printer) *formatter {
	f := &formatter{
		printer: p,
		current: frame{
			settings: settings{
				nodeSep:   newline,
				parentSep: newline,
			},
		},
	}
	return f
}

type whiteSpace int

const (
	ignore whiteSpace = 0

	// write a space, or disallow it
	blank whiteSpace = 1 << iota
	vtab             // column marker
	noblank

	nooverride

	comma      // print a comma, unless trailcomma overrides it
	trailcomma // print a trailing comma unless closed on same line
	declcomma  // write a comma when not at the end of line

	newline    // write a line in a table
	formfeed   // next line is not part of the table
	newsection // add two newlines

	indent   // request indent an extra level after the next newline
	unindent // unindent a level after the next newline
	indented // element was indented.
)

type frame struct {
	cg  []*ast.CommentGroup
	pos int8

	settings
}

type settings struct {
	// separator is blank if the current node spans a single line and newline
	// otherwise.
	nodeSep   whiteSpace
	parentSep whiteSpace
	override  whiteSpace
}

// suppress spurious linter warning: field is actually used.
func init() {
	s := settings{}
	_ = s.override
}

func (f *formatter) print(a ...interface{}) {
	for _, x := range a {
		f.Print(x)
		switch x.(type) {
		case string, token.Token: // , *syntax.BasicLit, *syntax.Ident:
			f.current.pos++
		}
	}
}

func (f *formatter) formfeed() whiteSpace {
	if f.current.nodeSep == blank {
		return blank
	}
	return formfeed
}

func (f *formatter) wsOverride(def whiteSpace) whiteSpace {
	if f.current.override == ignore {
		return def
	}
	return f.current.override
}

func (f *formatter) onOneLine(node ast.Node) bool {
	a := node.Pos()
	b := node.End()
	if a.IsValid() && b.IsValid() {
		return f.lineFor(a) == f.lineFor(b)
	}
	// TODO: walk and look at relative positions to determine the same?
	return false
}

func (f *formatter) before(node ast.Node) bool {
	f.stack = append(f.stack, f.current)
	f.current = frame{settings: f.current.settings}
	f.current.parentSep = f.current.nodeSep

	if node != nil {
		s, ok := node.(*ast.StructLit)
		if ok && len(s.Elts) <= 1 && f.current.nodeSep != blank && f.onOneLine(node) {
			f.current.nodeSep = blank
		}
		f.current.cg = ast.Comments(node)
		f.visitComments(f.current.pos)
		return true
	}
	return false
}

func (f *formatter) after(node ast.Node) {
	f.visitComments(127)
	p := len(f.stack) - 1
	f.current = f.stack[p]
	f.stack = f.stack[:p]
	f.current.pos++
	f.visitComments(f.current.pos)
}

func (f *formatter) visitComments(until int8) {
	c := &f.current

	printed := false
	for ; len(c.cg) > 0 && c.cg[0].Position <= until; c.cg = c.cg[1:] {
		if printed {
			f.Print(newsection)
		}
		printed = true
		f.printComment(c.cg[0])
	}
}

func (f *formatter) printComment(cg *ast.CommentGroup) {
	f.Print(cg)

	printBlank := false
	if cg.Doc && len(f.output) > 0 {
		f.Print(newline)
		printBlank = true
	}
	for _, c := range cg.List {
		isEnd := strings.HasPrefix(c.Text, "//")
		if !printBlank {
			if isEnd {
				f.Print(vtab)
			} else {
				f.Print(blank)
			}
		}
		f.Print(c.Slash)
		f.Print(c)
		if isEnd {
			f.Print(newline)
			if cg.Doc {
				f.Print(nooverride)
			}
		}
	}
}
