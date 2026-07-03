package maat

import (
	"fmt"
	"strconv"
	"strings"
)

// YAMLError is returned when the input uses a YAML feature this parser does not
// support. Like the Python port, we fail loudly rather than guess, because
// silently misparsing documentation metadata is worse than a hard error.
type YAMLError struct{ msg string }

func (e *YAMLError) Error() string { return e.msg }

func yamlErr(format string, args ...any) error {
	return &YAMLError{msg: fmt.Sprintf(format, args...)}
}

// stripComment removes a trailing '#' comment that is not inside quotes. A '#'
// only starts a comment when at the start of the line or preceded by
// whitespace. Iterates over runes so multi-byte characters are handled like
// Python's per-character loop.
func stripComment(line string) string {
	runes := []rune(line)
	inSingle, inDouble := false, false
	for i, ch := range runes {
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '#' && !inSingle && !inDouble:
			if i == 0 || runes[i-1] == ' ' || runes[i-1] == '\t' {
				return string(runes[:i])
			}
		}
	}
	return line
}

// scalar coerces a scalar token into a Go value: string, int64, float64, bool,
// or nil. Quoted strings keep their contents verbatim.
func scalar(raw string) any {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	switch strings.ToLower(s) {
	case "null", "~", "none":
		return nil
	case "true":
		return true
	case "false":
		return false
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

// flowList parses a flow sequence like [a, b, c], respecting quotes.
func flowList(raw string) []any {
	inner := strings.TrimSpace(raw)
	if len(inner) >= 2 {
		inner = inner[1 : len(inner)-1]
	} else {
		inner = ""
	}
	if strings.TrimSpace(inner) == "" {
		return []any{}
	}
	items := []any{}
	var buf strings.Builder
	inS, inD := false, false
	for _, ch := range inner {
		switch {
		case ch == '\'' && !inD:
			inS = !inS
			buf.WriteRune(ch)
		case ch == '"' && !inS:
			inD = !inD
			buf.WriteRune(ch)
		case ch == ',' && !inS && !inD:
			items = append(items, scalar(buf.String()))
			buf.Reset()
		default:
			buf.WriteRune(ch)
		}
	}
	if strings.TrimSpace(buf.String()) != "" {
		items = append(items, scalar(buf.String()))
	}
	return items
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

type yamlLine struct {
	indent int
	text   string
}

type lineReader struct {
	rows []yamlLine
	pos  int
}

func newLineReader(text string) (*lineReader, error) {
	lr := &lineReader{}
	for _, raw := range pySplitlines(text) {
		end := indentOf(raw) + 1
		if end > len(raw) {
			end = len(raw)
		}
		if strings.Contains(raw[:end], "\t") {
			return nil, yamlErr("tabs are not allowed for indentation")
		}
		stripped := strings.TrimRight(stripComment(raw), " \t\r\n\v\f")
		if strings.TrimSpace(stripped) == "" {
			continue
		}
		lr.rows = append(lr.rows, yamlLine{indent: indentOf(stripped), text: strings.TrimSpace(stripped)})
	}
	return lr, nil
}

func (lr *lineReader) peek() (yamlLine, bool) {
	if lr.pos < len(lr.rows) {
		return lr.rows[lr.pos], true
	}
	return yamlLine{}, false
}

func (lr *lineReader) next() yamlLine {
	row := lr.rows[lr.pos]
	lr.pos++
	return row
}

func parseBlock(lr *lineReader, indent int) (any, error) {
	peek, ok := lr.peek()
	if !ok {
		return nil, nil
	}
	if strings.HasPrefix(peek.text, "- ") {
		return parseSequence(lr, indent)
	}
	return parseMapping(lr, indent)
}

func parseSequence(lr *lineReader, indent int) (any, error) {
	seq := []any{}
	for {
		peek, ok := lr.peek()
		if !ok || peek.indent != indent || !strings.HasPrefix(peek.text, "- ") {
			break
		}
		row := lr.next()
		item := strings.TrimSpace(row.text[2:])
		if item != "" {
			if strings.HasPrefix(item, "[") {
				seq = append(seq, flowList(item))
			} else {
				seq = append(seq, scalar(item))
			}
		} else {
			child, err := parseBlock(lr, indent+2)
			if err != nil {
				return nil, err
			}
			seq = append(seq, child)
		}
	}
	return seq, nil
}

func parseMapping(lr *lineReader, indent int) (any, error) {
	mapping := map[string]any{}
	for {
		peek, ok := lr.peek()
		if !ok || peek.indent != indent {
			break
		}
		if strings.HasPrefix(peek.text, "- ") {
			break
		}
		row := lr.next()
		text := row.text
		idx := strings.Index(text, ":")
		if idx < 0 {
			return nil, yamlErr("expected 'key: value', got: %s", pyRepr(text))
		}
		key := strings.TrimSpace(text[:idx])
		rest := strings.TrimSpace(text[idx+1:])
		switch {
		case rest == "":
			child, ok := lr.peek()
			if ok && child.indent > indent {
				val, err := parseBlock(lr, child.indent)
				if err != nil {
					return nil, err
				}
				mapping[key] = val
			} else {
				mapping[key] = nil
			}
		case strings.HasPrefix(rest, "["):
			mapping[key] = flowList(rest)
		default:
			mapping[key] = scalar(rest)
		}
	}
	return mapping, nil
}

// yamlParse parses a YAML-subset document into Go values. An empty document
// yields an empty mapping, matching the Python parser.
func yamlParse(text string) (any, error) {
	lr, err := newLineReader(text)
	if err != nil {
		return nil, err
	}
	first, ok := lr.peek()
	if !ok {
		return map[string]any{}, nil
	}
	return parseBlock(lr, first.indent)
}
