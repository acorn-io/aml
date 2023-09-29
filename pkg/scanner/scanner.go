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

// Package scanner implements a scanner for CUE source text. It takes a []byte
// as source which can then be tokenized through repeated calls to the Scan
// method.
package scanner

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/acorn-io/aml/pkg/token"
)

// An ErrorHandler is a generic error handler used throughout CUE packages.
//
// The position points to the beginning of the offending value.
type ErrorHandler func(pos token.Pos, msg string, args []interface{})

// A Scanner holds the Scanner's internal state while processing
// a given text. It can be allocated as part of another data
// structure but must be initialized via Init before use.
type Scanner struct {
	// immutable state
	file *token.File  // source file handle
	dir  string       // directory portion of file.Name()
	src  []byte       // source
	errh ErrorHandler // error reporting; or nil
	mode Mode         // scanning mode

	// scanning state
	ch              rune // current character
	offset          int  // character offset
	rdOffset        int  // reading offset (position after current character)
	lineOffset      int  // current line offset
	linesSinceLast  int
	spacesSinceLast int
	insertEOL       bool // insert a comma before next newline

	quoteStack []quoteInfo

	// public state - ok to modify
	ErrorCount int // number of errors encountered
}

type quoteInfo struct {
	char    rune
	numChar int
}

// Read the next Unicode char into s.ch.
// s.ch < 0 means end-of-file.
func (s *Scanner) next() {
	if s.rdOffset < len(s.src) {
		s.offset = s.rdOffset
		if s.ch == '\n' {
			s.lineOffset = s.offset
			s.file.AddLine(s.offset)
		}
		r, w := rune(s.src[s.rdOffset]), 1
		switch {
		case r == 0:
			s.errf(s.offset, "illegal character NUL")
		case r >= utf8.RuneSelf:
			// not ASCII
			r, w = utf8.DecodeRune(s.src[s.rdOffset:])
			if r == utf8.RuneError && w == 1 {
				s.errf(s.offset, "illegal UTF-8 encoding")
			} else if r == bom && s.offset > 0 {
				s.errf(s.offset, "illegal byte order mark")
			}
		}
		s.rdOffset += w
		s.ch = r
	} else {
		s.offset = len(s.src)
		if s.ch == '\n' {
			s.lineOffset = s.offset
			s.file.AddLine(s.offset)
		}
		s.ch = -1 // eof
	}
}

const bom = 0xFEFF // byte order mark, only permitted as very first character

// A Mode value is a set of flags (or 0).
// They control scanner behavior.
type Mode uint

// These constants are options to the Init function.
const (
	ScanComments     Mode = 1 << iota // return comments as COMMENT tokens
	DontInsertCommas                  // do not automatically insert commas
)

// Init prepares the scanner s to tokenize the text src by setting the
// scanner at the beginning of src. The scanner uses the file set file
// for position information and it adds line information for each line.
// It is ok to re-use the same file when re-scanning the same file as
// line information which is already present is ignored. Init causes a
// panic if the file size does not match the src size.
//
// Calls to Scan will invoke the error handler err if they encounter a
// syntax error and err is not nil. Also, for each error encountered,
// the Scanner field ErrorCount is incremented by one. The mode parameter
// determines how comments are handled.
//
// Note that Init may call err if there is an error in the first character
// of the file.
func (s *Scanner) Init(file *token.File, src []byte, eh ErrorHandler, mode Mode) {
	// Explicitly initialize all fields since a scanner may be reused.
	if file.Size() != len(src) {
		panic(fmt.Sprintf("file size (%d) does not match src len (%d)", file.Size(), len(src)))
	}
	s.file = file
	s.dir, _ = filepath.Split(file.Name())
	s.src = src
	s.errh = eh
	s.mode = mode

	s.ch = ' '
	s.offset = 0
	s.rdOffset = 0
	s.lineOffset = 0
	s.insertEOL = false
	s.ErrorCount = 0

	s.next()
	if s.ch == bom {
		s.next() // ignore BOM at file beginning
	}
}

func (s *Scanner) errf(offs int, msg string, args ...interface{}) {
	if s.errh != nil {
		s.errh(s.file.Pos(offs, 0), msg, args)
	}
	s.ErrorCount++
}

var prefix = []byte("//line ")

func (s *Scanner) interpretLineComment(text []byte) {
	if bytes.HasPrefix(text, prefix) {
		// get filename and line number, if any
		if i := bytes.LastIndex(text, []byte{':'}); i > 0 {
			if line, err := strconv.Atoi(string(text[i+1:])); err == nil && line > 0 {
				// valid //line filename:line comment
				filename := string(bytes.TrimSpace(text[len(prefix):i]))
				if filename != "" {
					filename = filepath.Clean(filename)
					if !filepath.IsAbs(filename) {
						// make filename relative to current directory
						filename = filepath.Join(s.dir, filename)
					}
				}
				// update scanner position
				s.file.AddLineInfo(s.lineOffset+len(text)+1, filename, line) // +len(text)+1 since comment applies to next line
			}
		}
	}
}

func (s *Scanner) scanComment() string {
	// initial '/' already consumed; s.ch == '/'
	offs := s.offset - 1 // position of initial '/'
	hasCR := false

	if s.ch == '/' {
		//-style comment
		s.next()
		for s.ch != '\n' && s.ch >= 0 {
			if s.ch == '\r' {
				hasCR = true
			}
			s.next()
		}
		if offs == s.lineOffset {
			// comment starts at the beginning of the current line
			s.interpretLineComment(s.src[offs:s.offset])
		}
		goto exit
	}

	s.errf(offs, "comment not terminated")

exit:
	lit := s.src[offs:s.offset]
	if hasCR {
		// TODO: preserve /r/n
		lit = stripCR(lit)
	}

	return string(lit)
}

func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch >= utf8.RuneSelf && unicode.IsLetter(ch)
}

func isDigit(ch rune) bool {
	// TODO(mpvl): Is this correct?
	return '0' <= ch && ch <= '9' || ch >= utf8.RuneSelf && unicode.IsDigit(ch)
}

func (s *Scanner) scanFieldIdentifier() string {
	offs := s.offset
	if s.ch == '$' {
		s.next()
		return string(s.src[offs:s.offset])
	}
	for isLetter(s.ch) || isDigit(s.ch) || s.ch == '_' {
		s.next()
	}
	return string(s.src[offs:s.offset])
}

func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch - '0')
	case ch == '_':
		return 0
	case 'a' <= ch && ch <= 'f':
		return int(ch - 'a' + 10)
	case 'A' <= ch && ch <= 'F':
		return int(ch - 'A' + 10)
	}
	return 16 // larger than any legal digit val
}

func (s *Scanner) scanMantissa(base int) {
	var last rune
	for digitVal(s.ch) < base {
		if last == '_' && s.ch == '_' {
			s.errf(s.offset, "illegal '_' in number")
		}
		last = s.ch
		s.next()
	}
	if last == '_' {
		s.errf(s.offset-1, "illegal '_' in number")
	}
}

func (s *Scanner) scanNumber(seenDecimalPoint bool) (token.Token, string) {
	// digitVal(s.ch) < 10
	offs := s.offset
	tok := token.NUMBER

	if seenDecimalPoint {
		offs--
		s.scanMantissa(10)
		goto exponent
	}

	if s.ch == '0' {
		// int or float
		offs := s.offset
		s.next()
		// 0 or float
		seenDigits := false
		if s.ch >= '0' && s.ch <= '9' {
			seenDigits = true
			s.scanMantissa(10)
		}
		if s.ch == '.' || s.ch == 'e' || s.ch == 'E' {
			goto fraction
		}
		if seenDigits {
			// integer other than 0 may not start with 0
			s.errf(offs, "illegal integer number")
		}
		goto exit
	}

	// decimal int or float
	s.scanMantissa(10)

fraction:
	if s.ch == '.' {
		if p := s.offset + 1; p < len(s.src) && s.src[p] == '.' {
			// interpret dot as part of a range.
			goto exit
		}
		s.next()
		s.scanMantissa(10)
	}

exponent:
	switch s.ch {
	case 'K', 'k', 'M', 'm', 'G', 'g', 'T', 't', 'P', 'p':
		s.next()
		if s.ch == 'i' {
			s.next()
		}
		goto exit
	}

	if s.ch == 'e' || s.ch == 'E' {
		s.next()
		if s.ch == '-' || s.ch == '+' {
			s.next()
		}
		s.scanMantissa(10)
	}

exit:
	return tok, string(s.src[offs:s.offset])
}

func (s *Scanner) scanString(offs int, quote quoteInfo) (token.Token, string) {
	// ", """, `, or ``` opening already consumed

	tok := token.STRING

	hasCR := false
	extra := 0
	for {
		if (quote.numChar != 3 && s.ch == '\n') || s.ch < 0 {
			s.errf(offs, "string literal not terminated")
			lit := s.src[offs:s.offset]
			if hasCR {
				lit = stripCR(lit)
			}
			return tok, string(lit)
		}

		n := s.consumeRepeating(quote.char, quote.numChar)
		if n == quote.numChar {
			break
		}
		if s.ch == '\r' && quote.numChar == 3 {
			hasCR = true
		}
		if s.ch == '\\' {
			s.next()
			if s.ch == quote.char {
				s.next()
			} else if quote.char == '"' && s.ch == '(' {
				extra = 1
				tok = token.INTERPOLATION
				s.quoteStack = append(s.quoteStack, quote)
				break
			} else if quote.char == '"' {
				s.next()
			}
		} else {
			s.next()
		}
	}
	lit := s.src[offs : s.offset+extra]
	if hasCR {
		lit = stripCR(lit)
	}
	return tok, string(lit)
}

func (s *Scanner) consumeRepeating(quote rune, max int) (n int) {
	for ; n < max; n++ {
		if s.ch != quote {
			return n
		}
		s.next()
	}
	return n
}

func stripCR(b []byte) []byte {
	c := make([]byte, len(b))
	i := 0
	for _, ch := range b {
		if ch != '\r' {
			c[i] = ch
			i++
		}
	}
	return c[:i]
}

// recoverParen is an approximate recovery mechanism to recover from invalid
// attributes.
func (s *Scanner) recoverParen(open int) {
	for {
		switch s.ch {
		case '\n', -1:
			return
		case '(':
			open++
		case ')':
			if open--; open == 0 {
				return
			}
		}
		s.next()
	}
}

func (s *Scanner) skipWhitespace(inc int) {
	for {
		switch s.ch {
		case ' ', '\t':
			s.spacesSinceLast += inc
		case '\n':
			s.linesSinceLast += inc
			if s.insertEOL {
				return
			}
		case '\r':
		default:
			return
		}
		s.next()
	}
}

// Helper functions for scanning multi-byte tokens such as >> += >>= .
// Different routines recognize different length tok_i based on matches
// of ch_i. If a token ends in '=', the result is tok1 or tok3
// respectively. Otherwise, the result is tok0 if there was no other
// matching character, or tok2 if the matching character was ch2.

func (s *Scanner) switch2(tok0, tok1 token.Token) token.Token {
	if s.ch == '=' {
		s.next()
		return tok1
	}
	return tok0
}

func (s *Scanner) popInterpolation() quoteInfo {
	quote := s.quoteStack[len(s.quoteStack)-1]
	s.quoteStack = s.quoteStack[:len(s.quoteStack)-1]
	return quote
}

// ResumeInterpolation resumes scanning of a string interpolation.
func (s *Scanner) ResumeInterpolation() string {
	quote := s.popInterpolation()
	_, str := s.scanString(s.offset-1, quote)
	return str
}

// Offset returns the current position offset.
func (s *Scanner) Offset() int {
	return s.offset
}

// Scan scans the next token and returns the token position, the token,
// and its literal string if applicable. The source end is indicated by
// EOF.
//
// If the returned token is a literal (IDENT, INT, FLOAT,
// IMAG, CHAR, STRING) or COMMENT, the literal string
// has the corresponding value.
//
// If the returned token is a keyword, the literal string is the keyword.
//
// If the returned token is Comma, the corresponding
// literal string is "," if the comma was present in the source,
// and "\n" if the semicolon was inserted because of a newline or
// at EOF.
//
// If the returned token is ILLEGAL, the literal string is the
// offending character.
//
// In all other cases, Scan returns an empty literal string.
//
// For more tolerant parsing, Scan will return a valid token if
// possible even if a syntax error was encountered. Thus, even
// if the resulting token sequence contains no illegal tokens,
// a client may not assume that no error occurred. Instead it
// must check the scanner's ErrorCount or the number of calls
// of the error handler, if there was one installed.
//
// Scan adds line information to the file added to the file
// set with Init. Token positions are relative to that file
// and thus relative to the file set.
func (s *Scanner) Scan() (pos token.Pos, tok token.Token, lit string) {
scanAgain:
	s.skipWhitespace(1)

	var rel token.RelPos
	switch {
	case s.linesSinceLast > 1:
		rel = token.NewSection
	case s.linesSinceLast == 1:
		rel = token.Newline
	case s.spacesSinceLast > 0:
		rel = token.Blank
	default:
		rel = token.NoSpace
	}
	// current token start
	offset := s.offset
	pos = s.file.Pos(offset, rel)

	// determine token value
	insertEOL := false
	var quote quoteInfo
	switch ch := s.ch; {
	case '0' <= ch && ch <= '9':
		insertEOL = true
		tok, lit = s.scanNumber(false)
	case isLetter(ch), ch == '$':
		lit = s.scanFieldIdentifier()
		tok = token.Lookup(lit)
		insertEOL = true
		break
	default:
		s.next() // always make progress
		switch ch {
		case -1:
			if s.insertEOL {
				s.insertEOL = false // EOF consumed
				return s.file.Pos(offset, token.Elided), token.COMMA, "\n"
			}
			tok = token.EOF

		case '\n':
			// we only reach here if s.insertComma was
			// set in the first place and exited early
			// from s.skipWhitespace()
			s.insertEOL = false // newline consumed
			p := s.file.Pos(offset, token.Elided)
			s.skipWhitespace(1)
			// Don't elide comma before a ',' or ':' to ensure JSON
			// conformance. Note that cue fmt should immediately undo those.
			if s.ch == ',' || s.ch == ':' {
				return s.Scan()
			}
			return p, token.COMMA, "\n"

		case '"', '`':
			insertEOL = true
			offs := s.offset - 1
			quote.char = ch
			quote.numChar = 1
			switch n := s.consumeRepeating(ch, 2); n {
			case 0:
				tok, lit = s.scanString(offs, quote)
			case 1:
				tok, lit = token.STRING, string(s.src[offs:s.offset])
			case 2:
				quote.numChar = 3
				tok, lit = s.scanString(offs, quote)
			}
		case ':':
			tok = token.COLON
		case '?':
			tok = token.OPTION
			insertEOL = true
		case '.':
			if '0' <= s.ch && s.ch <= '9' {
				insertEOL = true
				tok, lit = s.scanNumber(true)
			} else {
				tok = token.PERIOD
			}
		case ',':
			tok = token.COMMA
			lit = ","
		case '(':
			tok = token.LPAREN
		case ')':
			insertEOL = true
			tok = token.RPAREN
		case '[':
			tok = token.LBRACK
		case ']':
			insertEOL = true
			tok = token.RBRACK
		case '{':
			tok = token.LBRACE
		case '}':
			insertEOL = true
			tok = token.RBRACE
		case '+':
			tok = token.ADD // Consider ++ for list concatenate.
		case '-':
			tok = token.SUB
		case '*':
			tok = token.MUL
		case '/':
			if s.ch == '/' {
				// comment
				if s.insertEOL {
					// reset position to the beginning of the comment
					s.ch = '/'
					s.offset = s.file.Offset(pos)
					s.rdOffset = s.offset + 1
					s.insertEOL = false // newline consumed
					return s.file.Pos(offset, token.Elided), token.COMMA, "\n"
				}
				comment := s.scanComment()
				if s.mode&ScanComments == 0 {
					// skip comment
					s.insertEOL = false // newline consumed
					goto scanAgain
				}
				tok = token.COMMENT
				lit = comment
			} else {
				tok = token.QUO
			}
		// We no longer use %, but seems like a useful token to use for
		// something else at some point.
		// case '%':
		case '<':
			tok = s.switch2(token.LSS, token.LEQ)
		case '>':
			tok = s.switch2(token.GTR, token.GEQ)
		case '=':
			if s.ch == '~' {
				s.next()
				tok = token.MAT
			} else if s.ch == '=' {
				s.next()
				tok = token.EQL
			} else {
				s.errf(s.file.Offset(pos), "illegal character %#U", ch)
				tok = token.ILLEGAL
			}
		case '!':
			if s.ch == '~' {
				s.next()
				tok = token.NMAT
			} else {
				tok = s.switch2(token.NOT, token.NEQ)
			}
		case '&':
			if s.ch == '&' {
				s.next()
				tok = token.LAND
			} else {
				s.errf(s.file.Offset(pos), "illegal character %#U", ch)
				tok = token.ILLEGAL
			}
		case '|':
			if s.ch == '|' {
				s.next()
				tok = token.LOR
			} else {
				s.errf(s.file.Offset(pos), "illegal character %#U", ch)
				tok = token.ILLEGAL
			}
		default:
			// next reports unexpected BOMs - don't repeat
			if ch != bom {
				s.errf(s.file.Offset(pos), "illegal character %#U", ch)
			}
			insertEOL = s.insertEOL // preserve insertSemi info
			tok = token.ILLEGAL
			lit = string(ch)
		}
	}
	if s.mode&DontInsertCommas == 0 {
		s.insertEOL = insertEOL
	}

	s.linesSinceLast = 0
	s.spacesSinceLast = 0
	return
}
