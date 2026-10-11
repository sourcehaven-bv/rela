package configedit

import "strings"

// keepLayout carries an edit into the original file text.
//
// Re-encoding a YAML syntax tree keeps comments but not everything else the
// author wrote: blank lines between sections disappear and some spacing is
// normalized. So the edit is replayed as a three-way merge of lines. The base
// is the original tree re-encoded with no edits, one side is the original
// text, and the other is the edited tree re-encoded. Lines only the original
// has (the blank lines) stay; lines only the edit changed take the edit.
// Where both differ from the base in the same place, the edit wins, because
// it is the change being saved.
func keepLayout(original, base, edited string) string {
	if base == edited {
		return original
	}
	o, b, e := splitLines(original), splitLines(base), splitLines(edited)
	ob, ok1 := matchLines(b, o) // base index → original index
	eb, ok2 := matchLines(b, e) // base index → edited index
	if !ok1 || !ok2 {
		return edited
	}

	var out []string
	bi, oi, ei := 0, 0, 0
	for {
		// The next base line both sides still have marks the end of a chunk.
		next := bi
		for next < len(b) && (ob[next] < 0 || eb[next] < 0) {
			next++
		}
		oEnd, eEnd := len(o), len(e)
		if next < len(b) {
			oEnd, eEnd = ob[next], eb[next]
		}
		baseChunk, origChunk, editChunk := b[bi:next], o[oi:oEnd], e[ei:eEnd]
		switch {
		case equalLines(editChunk, baseChunk):
			out = append(out, origChunk...)
		case equalLines(origChunk, baseChunk), equalLines(origChunk, editChunk):
			out = append(out, editChunk...)
		default:
			out = append(out, mergeConflict(origChunk, editChunk)...)
		}
		if next >= len(b) {
			break
		}
		out = append(out, o[oEnd])
		bi, oi, ei = next+1, oEnd+1, eEnd+1
	}
	return strings.Join(out, "")
}

// mergeConflict resolves a chunk both sides changed. The edit's lines are
// taken, keeping the original's leading blank lines so a section separator
// above an edited item survives.
func mergeConflict(orig, edit []string) []string {
	var out []string
	for _, line := range orig {
		if strings.TrimSpace(line) != "" {
			break
		}
		out = append(out, line)
	}
	return append(out, edit...)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// maxEditLines bounds the edit distance matchLines searches. Its trace grows
// with the square of the distance, so a file whose layout differs from the
// re-encoding on most lines would otherwise cost gigabytes; past the bound
// the caller writes the plain re-encoding instead.
const maxEditLines = 2000

// matchLines returns, for each line of a, the index of the line of b it is
// matched with in a shortest edit script, or -1. It is Myers' O(ND) diff.
// ok is false when the edit distance exceeds maxEditLines.
func matchLines(a, b []string) (match []int, ok bool) {
	n, m := len(a), len(b)
	match = make([]int, n)
	for i := range match {
		match[i] = -1
	}
	maxD := n + m
	offset := maxD + 1
	v := make([]int, 2*maxD+3)
	// trace[d] holds v for diagonals -d-1..d+1 as round d starts, which is
	// every value backtrack reads for that round.
	trace := make([][]int, 0, min(maxD, maxEditLines)+1)
	found := false
	for d := 0; d <= maxD && !found; d++ {
		if d > maxEditLines {
			return nil, false
		}
		snapshot := make([]int, 2*d+3)
		copy(snapshot, v[offset-d-1:offset+d+2])
		trace = append(trace, snapshot)
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				found = true
				break
			}
		}
	}
	backtrack(trace, n, m, match)
	return match, true
}

// backtrack walks the trace of matchLines back from (n, m) to recover the
// diagonal (matching) moves, recording each in match.
func backtrack(trace [][]int, n, m int, match []int) {
	x, y := n, m
	for d := len(trace) - 1; d >= 0 && (x > 0 || y > 0); d-- {
		vd := trace[d]
		at := func(k int) int { return vd[k+d+1] }
		k := x - y
		var prevK int
		if k == -d || (k != d && at(k-1) < at(k+1)) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := at(prevK)
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x--
			y--
			match[x] = y
		}
		if d > 0 {
			x, y = prevX, prevY
		}
	}
}
