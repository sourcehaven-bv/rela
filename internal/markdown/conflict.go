package markdown

import "strings"

// Git's conflict markers. Git writes each at column 0 of its own line. The
// start and end markers are followed by a space and a label, or by nothing;
// the separator stands alone.
const (
	ConflictMarkerStart = "<<<<<<<"
	ConflictMarkerMid   = "======="
	ConflictMarkerEnd   = ">>>>>>>"
)

// ConflictBlock is one git conflict region. Line numbers are 1-based.
type ConflictBlock struct {
	StartLine int    // line of the start marker
	MidLine   int    // line of the separator
	EndLine   int    // line of the end marker
	OursRef   string // label after the start marker (e.g. "HEAD")
	TheirsRef string // label after the end marker (e.g. a branch name)
}

// ConflictBlocks returns the git conflict blocks in content, in order.
//
// A block is a start marker line, then a separator line, then an end marker
// line, each written as git writes it at column 0. A marker line on its own
// is not a conflict (BUG-GYM7ZBS), and neither is a marker anywhere but
// column 0, as in inline code, prose or an indented code block (BUG-WN6D).
//
// Fenced code does not exempt a block. Git moves lines that both sides share
// out of a conflict, so a real conflict inside a code block often has the
// fence line above its start marker, and a document cannot be told apart
// from such a merge. Reading it as text would let the next save store the
// merge for good. A document that shows a complete block indents its marker
// lines inside the code fence instead.
func ConflictBlocks(content string) []ConflictBlock {
	if !strings.Contains(content, ConflictMarkerStart) {
		return nil
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	var blocks []ConflictBlock
	for i := 0; i < len(lines); i++ {
		if _, ok := labelAfter(lines[i], ConflictMarkerStart); !ok {
			continue
		}
		// A start marker without a complete block behind it is text, and
		// the scan goes on from the next line.
		if b, ok := conflictBlockAt(lines, i); ok {
			blocks = append(blocks, b)
			i = b.EndLine - 1
		}
	}
	return blocks
}

// conflictBlockAt returns the block whose start marker is lines[start], and
// false when no separator and end marker follow it before the next start
// marker or the end of content. A later start marker begins its own block,
// which also keeps the scan linear: each search stops where the next begins.
func conflictBlockAt(lines []string, start int) (ConflictBlock, bool) {
	ours, _ := labelAfter(lines[start], ConflictMarkerStart)
	b := ConflictBlock{StartLine: start + 1, OursRef: ours}
	for j := start + 1; j < len(lines); j++ {
		line := lines[j]
		if _, ok := labelAfter(line, ConflictMarkerStart); ok {
			return ConflictBlock{}, false
		}
		if b.MidLine == 0 {
			if line == ConflictMarkerMid {
				b.MidLine = j + 1
			}
			continue
		}
		if theirs, ok := labelAfter(line, ConflictMarkerEnd); ok {
			b.EndLine = j + 1
			b.TheirsRef = theirs
			return b, true
		}
	}
	return ConflictBlock{}, false
}

// labelAfter reports whether line is marker as git writes it: the marker
// alone, or followed by a space and a label, which it returns.
func labelAfter(line, marker string) (string, bool) {
	rest, ok := strings.CutPrefix(line, marker)
	if !ok || (rest != "" && rest[0] != ' ') {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// HasConflictMarkers reports whether content holds a git conflict block, as
// [ConflictBlocks] defines one.
func HasConflictMarkers(content []byte) bool {
	return HasConflictMarkersString(string(content))
}

// HasConflictMarkersString is the string-typed companion of
// [HasConflictMarkers]; same semantics.
func HasConflictMarkersString(content string) bool {
	return len(ConflictBlocks(content)) > 0
}
