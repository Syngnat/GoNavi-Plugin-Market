// Package ngxconf parses nginx configuration text into a directive tree.
// It handles comments, quoted arguments, escape sequences and nested blocks.
package ngxconf

import (
	"fmt"
	"strings"
)

// Arg is one directive argument; Quoted marks quoted values so the UI can
// distinguish 'value with spaces' from bare words.
type Arg struct {
	Value  string `json:"value"`
	Quoted bool   `json:"quoted,omitempty"`
}

// Directive is one nginx directive or block. Block is nil for simple directives.
type Directive struct {
	Name  string      `json:"name"`
	Args  []Arg       `json:"args,omitempty"`
	Line  int         `json:"line"`
	File  string      `json:"file,omitempty"`
	Block []Directive `json:"block,omitempty"`
}

// FirstArg returns the first argument value or "".
func (d *Directive) FirstArg() string {
	if len(d.Args) > 0 {
		return d.Args[0].Value
	}
	return ""
}

// ArgValues returns plain argument values.
func (d *Directive) ArgValues() []string {
	out := make([]string, 0, len(d.Args))
	for _, a := range d.Args {
		out = append(out, a.Value)
	}
	return out
}

// ParseError carries the offending line number.
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

type tokenKind int

const (
	tokenWord tokenKind = iota
	tokenOpenBrace
	tokenCloseBrace
	tokenSemicolon
)

type token struct {
	kind   tokenKind
	text   string
	quoted bool
	line   int
}

// eofToken is the sentinel returned once the source is exhausted; a real
// word can never be empty because readWord errors instead.
func eofToken(line int) token { return token{kind: tokenWord, line: line} }

func isEOF(t token) bool { return t.kind == tokenWord && t.text == "" && !t.quoted }

type lexer struct {
	src  string
	pos  int
	line int
}

func (l *lexer) next() (token, error) {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n':
			l.line++
			l.pos++
		case c == ' ' || c == '\t' || c == '\r':
			l.pos++
		case c == '#':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		case c == '{':
			l.pos++
			return token{kind: tokenOpenBrace, line: l.line}, nil
		case c == '}':
			l.pos++
			return token{kind: tokenCloseBrace, line: l.line}, nil
		case c == ';':
			l.pos++
			return token{kind: tokenSemicolon, line: l.line}, nil
		default:
			return l.readWord()
		}
	}
	return eofToken(l.line), nil
}

func (l *lexer) readWord() (token, error) {
	startLine := l.line
	var sb strings.Builder
	quoted := false
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '\'' || c == '"' {
			quote := c
			quoted = true
			l.pos++
			closed := false
			for l.pos < len(l.src) {
				ch := l.src[l.pos]
				if ch == '\\' && l.pos+1 < len(l.src) {
					l.pos++
					escaped := l.src[l.pos]
					if escaped != quote && escaped != '\\' {
						sb.WriteByte('\\')
					}
					sb.WriteByte(escaped)
					l.pos++
					continue
				}
				if ch == quote {
					l.pos++
					closed = true
					break
				}
				if ch == '\n' {
					l.line++
				}
				sb.WriteByte(ch)
				l.pos++
			}
			if !closed {
				return token{}, &ParseError{Line: startLine, Msg: "unterminated quoted string"}
			}
			break
		}
		switch {
		case c == '\\' && l.pos+1 < len(l.src):
			l.pos++
			escaped := l.src[l.pos]
			if escaped == '\n' {
				l.line++
				l.pos++
				continue
			}
			sb.WriteByte(escaped)
			l.pos++
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ';' || c == '{' || c == '}' || c == '#':
			goto wordDone
		default:
			sb.WriteByte(c)
			l.pos++
		}
	}
wordDone:
	if sb.Len() == 0 && !quoted {
		return token{}, &ParseError{Line: startLine, Msg: "empty token"}
	}
	return token{kind: tokenWord, text: sb.String(), quoted: quoted, line: startLine}, nil
}

// Parse turns one config file into a directive tree. filename tags every
// directive so include expansion can attribute lines to source files.
func Parse(filename, content string) ([]Directive, error) {
	l := &lexer{src: content, line: 1}
	return parseBlock(l, filename, 0)
}

func parseBlock(l *lexer, filename string, depth int) ([]Directive, error) {
	if depth > 32 {
		return nil, &ParseError{Line: l.line, Msg: "block nesting too deep"}
	}
	var out []Directive
	var pending *Directive
	for {
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		if isEOF(tok) {
			if pending != nil {
				return nil, &ParseError{Line: pending.Line, Msg: "directive missing terminator"}
			}
			if depth > 0 {
				return nil, &ParseError{Line: tok.line, Msg: "unexpected end of file, expecting '}'"}
			}
			return out, nil
		}
		switch tok.kind {
		case tokenSemicolon:
			if pending != nil {
				out = append(out, *pending)
				pending = nil
			}
		case tokenOpenBrace:
			if pending == nil {
				return nil, &ParseError{Line: tok.line, Msg: "unexpected '{'"}
			}
			block, err := parseBlock(l, filename, depth+1)
			if err != nil {
				return nil, err
			}
			pending.Block = block
			pending.File = filename
			out = append(out, *pending)
			pending = nil
		case tokenCloseBrace:
			if pending != nil {
				return nil, &ParseError{Line: pending.Line, Msg: "directive missing terminator before '}'"}
			}
			if depth == 0 {
				return nil, &ParseError{Line: tok.line, Msg: "unbalanced '}'"}
			}
			return out, nil
		case tokenWord:
			if pending != nil {
				pending.Args = append(pending.Args, Arg{Value: tok.text, Quoted: tok.quoted})
			} else {
				pending = &Directive{Name: tok.text, Line: tok.line}
			}
		}
	}
}
