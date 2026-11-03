package migration

import (
	"log/slog"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// foldWidth is the column at which [refoldFolded] wraps folded block
// scalars, the width yaml.v2 and most editors use. Columns are counted in
// bytes, so non-ASCII text wraps a little early; that changes only the
// layout.
const foldWidth = 80

// refoldFolded restores the layout of folded block scalars (`>`) in YAML
// that yaml.v3 emitted.
//
// yaml.v3 keeps a folded scalar's style but never wraps it, so a long
// paragraph comes back as one line, and a clipped (`>`) or stripped (`>-`)
// block gains a blank line after it. Folding joins lines with one space, so
// breaking a content line at a single space between two words does not
// change the value. This function re-wraps those lines at [foldWidth] and
// drops the trailing blank lines that chomping discards anyway.
//
// The blocks are found by parsing the YAML, not by matching text, so a
// comment or literal block that looks like a folded header is never
// touched. A block with an indentation indicator (`>2`, emitted for a value
// that starts with a space) is left as it is.
//
// The result is checked: if it does not decode to the same value as the
// input, the input is returned unchanged. A layout fix must never cost a
// value.
func refoldFolded(in []byte) []byte {
	var doc yaml.Node
	if err := yaml.Unmarshal(in, &doc); err != nil {
		return in
	}
	var headers []int
	collectFoldedHeaders(&doc, &headers)
	if len(headers) == 0 {
		return in
	}
	out := []byte(refoldBlocks(string(in), headers))

	var before, after any
	if err := doc.Decode(&before); err != nil {
		return in
	}
	if err := yaml.Unmarshal(out, &after); err != nil || !reflect.DeepEqual(before, after) {
		// Expected only for values DeepEqual cannot compare to themselves,
		// such as .nan; the file is then written as the encoder left it.
		slog.Debug("migration: kept encoder layout, re-wrapped folded scalars did not round-trip")
		return in
	}
	return out
}

// collectFoldedHeaders appends the 1-based line of every folded block
// scalar's header (the line holding the `>` indicator) under n.
func collectFoldedHeaders(n *yaml.Node, lines *[]int) {
	if n.Kind == yaml.ScalarNode && n.Style&yaml.FoldedStyle != 0 {
		*lines = append(*lines, n.Line)
	}
	for _, c := range n.Content {
		collectFoldedHeaders(c, lines)
	}
}

// refoldBlocks re-wraps the folded blocks whose headers sit on the given
// 1-based lines of text.
func refoldBlocks(text string, headers []int) string {
	// The file's final newline ends the last line; it is not a blank line
	// that chomping may drop.
	body, final := strings.CutSuffix(text, "\n")
	lines := strings.Split(body, "\n")
	isHeader := make(map[int]bool, len(headers))
	for _, h := range headers {
		isHeader[h-1] = true
	}

	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		out = append(out, lines[i])
		if !isHeader[i] {
			i++
			continue
		}
		block, next := refoldBlock(lines[i], lines[i+1:])
		out = append(out, block...)
		i += 1 + next
	}
	result := strings.Join(out, "\n")
	if final {
		result += "\n"
	}
	return result
}

// refoldBlock re-wraps the content of the folded block opened by header.
// rest holds the lines after the header. It returns the new content lines
// and how many lines of rest the block used.
func refoldBlock(header string, rest []string) (content []string, used int) {
	indicator := header[strings.LastIndexByte(header, '>')+1:]
	indicator, _, _ = strings.Cut(indicator, " ")
	if strings.ContainsAny(indicator, "123456789") {
		return nil, 0
	}
	keep := strings.Contains(indicator, "+")

	// The content indent is the indent of the first non-blank line; the
	// block runs until a non-blank line indented less than that.
	indent := -1
	end := 0
	for ; end < len(rest); end++ {
		l := rest[end]
		trimmed := strings.TrimLeft(l, " ")
		ind := len(l) - len(trimmed)
		if indent < 0 {
			if trimmed == "" {
				continue
			}
			indent = ind
			continue
		}
		if ind < indent && trimmed != "" {
			break
		}
	}
	if indent <= 0 {
		return nil, 0
	}
	block := rest[:end]
	if !keep {
		for len(block) > 0 && isBlankIn(block[len(block)-1], indent) {
			block = block[:len(block)-1]
		}
	}
	wrapped := make([]string, 0, len(block))
	for _, l := range block {
		wrapped = append(wrapped, wrapFolded(l, indent)...)
	}
	return wrapped, end
}

// isBlankIn reports whether l is an empty line of a block scalar whose
// content is indented by indent: only spaces, no more than indent of them.
// A longer run of spaces is content.
func isBlankIn(l string, indent int) bool {
	return len(l) <= indent && strings.TrimLeft(l, " ") == ""
}

// wrapFolded splits one content line of a folded block, indented by indent
// spaces, at single spaces so that no piece is wider than [foldWidth]
// where a break point exists. A line that starts with more whitespace is
// "more indented": its line breaks are literal, so it is left as it is.
func wrapFolded(line string, indent int) []string {
	if len(line) <= foldWidth || indent >= foldWidth/2 || len(line) <= indent {
		return []string{line}
	}
	if c := line[indent]; c == ' ' || c == '\t' {
		return []string{line}
	}
	prefix, rest := line[:indent], line[indent:]
	var pieces []string
	for len(prefix)+len(rest) > foldWidth {
		cut := breakPoint(rest, foldWidth-len(prefix))
		if cut < 0 {
			break
		}
		pieces = append(pieces, prefix+rest[:cut])
		rest = rest[cut+1:]
	}
	return append(pieces, prefix+rest)
}

// breakPoint returns the index of the last single space in s at or before
// limit, or the first one after it when a word is longer than limit, or -1
// when s has no single space to break at. A space next to another space or
// a tab is not a break point: folding would not restore the run, and a
// continuation line starting with a tab would be read as more indented.
func breakPoint(s string, limit int) int {
	single := func(k int) bool {
		return s[k] == ' ' && !isBlankByte(s[k-1]) && !isBlankByte(s[k+1])
	}
	for k := min(limit, len(s)-2); k > 0; k-- {
		if single(k) {
			return k
		}
	}
	for k := max(limit+1, 1); k < len(s)-1; k++ {
		if single(k) {
			return k
		}
	}
	return -1
}

func isBlankByte(c byte) bool { return c == ' ' || c == '\t' }
