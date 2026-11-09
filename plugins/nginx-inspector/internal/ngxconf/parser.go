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

// isWordDelimiter reports whether c ends an unquoted word. nginx also treats
// '#' as ordinary text inside a word (ngx_conf_read_token only starts a
// comment where a token may start), so a proxy_pass containing /#fragment
// stays one argument.
func isWordDelimiter(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n', ';', '{', '}':
		return true
	}
	return false
}

// writeEscaped decodes the escape sequences nginx recognises (\" \' \\ \t
// \r \n). Any other escaped byte keeps its backslash so regexes such as
// \.php$ are passed through unchanged.
func writeEscaped(sb *strings.Builder, escaped byte) {
	switch escaped {
	case '"', '\'', '\\':
		sb.WriteByte(escaped)
	case 't':
		sb.WriteByte('\t')
	case 'r':
		sb.WriteByte('\r')
	case 'n':
		sb.WriteByte('\n')
	default:
		sb.WriteByte('\\')
		sb.WriteByte(escaped)
	}
}

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
			// Comments start only where a token could start.
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

// readWord reads one argument, following ngx_conf_read_token semantics:
// quotes may open or close anywhere in a word, a '#' inside a word is literal,
// only the escapes nginx recognises are decoded, and text right after a closed
// quote continues the same word ("bot") yields bot) instead of a new token.
func (l *lexer) readWord() (token, error) {
	startLine := l.line
	var sb strings.Builder
	quoted := false
	var quote byte
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if quote != 0 {
			switch {
			case c == '\\' && l.pos+1 < len(l.src):
				l.pos++
				if l.src[l.pos] == '\n' {
					l.line++
				}
				writeEscaped(&sb, l.src[l.pos])
				l.pos++
			case c == quote:
				quote = 0
				l.pos++
			default:
				if c == '\n' {
					l.line++
				}
				sb.WriteByte(c)
				l.pos++
			}
			continue
		}
		switch {
		case c == '\'' || c == '"':
			quote = c
			quoted = true
			l.pos++
		case c == '\\' && l.pos+1 < len(l.src):
			l.pos++
			if l.src[l.pos] == '\n' {
				l.line++
			}
			writeEscaped(&sb, l.src[l.pos])
			l.pos++
		case isWordDelimiter(c):
			goto wordDone
		default:
			sb.WriteByte(c)
			l.pos++
		}
	}
	if quote != 0 {
		return token{}, &ParseError{Line: startLine, Msg: "unterminated quoted string"}
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
	directives, err := ParsePartial(filename, content)
	if err != nil {
		return nil, err
	}
	return directives, nil
}

// ParsePartial parses as much as possible: on error it returns the directives
// completed before the failure together with that error. A byte-truncated file
// or a config with a broken tail can therefore still render its parsed prefix
// instead of failing the whole page.
func ParsePartial(filename, content string) ([]Directive, error) {
	l := &lexer{src: content, line: 1}
	return parseBlock(l, filename, 0)
}

// CompleteBoundary returns the byte length of the longest prefix of content
// that ends right after a complete directive (';') or block ('}'). Cutting a
// byte-truncated file there removes the half-written tail; an enclosing block
// left open is reported by ParsePartial as a partial tree plus an EOF error.
func CompleteBoundary(content string) int {
	l := &lexer{src: content, line: 1}
	boundary := 0
	for {
		tok, err := l.next()
		if err != nil || isEOF(tok) {
			return boundary
		}
		if tok.kind == tokenSemicolon || tok.kind == tokenCloseBrace {
			boundary = l.pos
		}
	}
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
			return out, err
		}
		if isEOF(tok) {
			if pending != nil {
				return out, &ParseError{Line: pending.Line, Msg: "directive missing terminator"}
			}
			if depth > 0 {
				return out, &ParseError{Line: tok.line, Msg: "unexpected end of file, expecting '}'"}
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
				return out, &ParseError{Line: tok.line, Msg: "unexpected '{'"}
			}
			block, berr := parseBlock(l, filename, depth+1)
			pending.Block = block
			pending.File = filename
			out = append(out, *pending)
			pending = nil
			if berr != nil {
				return out, berr
			}
		case tokenCloseBrace:
			if pending != nil {
				return out, &ParseError{Line: pending.Line, Msg: "directive missing terminator before '}'"}
			}
			if depth == 0 {
				return out, &ParseError{Line: tok.line, Msg: "unbalanced '}'"}
			}
			return out, nil
		case tokenWord:
			if pending != nil {
				pending.Args = append(pending.Args, Arg{Value: tok.text, Quoted: tok.quoted})
			} else {
				pending = &Directive{Name: tok.text, Line: tok.line, File: filename}
			}
		}
	}
}
