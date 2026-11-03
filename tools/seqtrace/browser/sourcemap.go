package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// SourceMap answers which source file a position in a built script came
// from. It reads only what that needs from a source map v3: the sources and
// the generated column and source index of each segment.
type SourceMap struct {
	sources []string
	lines   [][]segment // per generated line, sorted by column
}

// segmentWithSource is the field count of a segment that names a source.
const segmentWithSource = 4

type segment struct {
	col, src int
}

// LoadSourceMap reads a source map file.
func LoadSourceMap(path string) (*SourceMap, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSourceMap(b)
}

// ParseSourceMap parses a source map v3 document.
func ParseSourceMap(b []byte) (*SourceMap, error) {
	var raw struct {
		Version    int      `json:"version"`
		SourceRoot string   `json:"sourceRoot"`
		Sources    []string `json:"sources"`
		Mappings   string   `json:"mappings"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("source map: %w", err)
	}
	if raw.Version != 3 {
		return nil, fmt.Errorf("source map: version %d, want 3", raw.Version)
	}
	m := &SourceMap{sources: make([]string, len(raw.Sources))}
	for i, s := range raw.Sources {
		m.sources[i] = raw.SourceRoot + s
	}
	src := 0
	for line := range strings.SplitSeq(raw.Mappings, ";") {
		var segs []segment
		col := 0
		for field := range strings.SplitSeq(line, ",") {
			if field == "" {
				continue
			}
			v, err := decodeVLQ(field)
			if err != nil {
				return nil, err
			}
			col += v[0]
			// A segment with a source has at least four fields: column,
			// source, source line and source column. A one-field segment
			// starts an unmapped range, which must not fall to the previous
			// segment's source.
			if len(v) >= segmentWithSource {
				src += v[1]
				segs = append(segs, segment{col: col, src: src})
			} else {
				segs = append(segs, segment{col: col, src: -1})
			}
		}
		sort.Slice(segs, func(i, j int) bool { return segs[i].col < segs[j].col })
		m.lines = append(m.lines, segs)
	}
	return m, nil
}

// Source returns the source of the 1-based line and column, as a browser
// stack trace gives them.
func (m *SourceMap) Source(line, col int) (string, bool) {
	if line < 1 || line > len(m.lines) {
		return "", false
	}
	segs := m.lines[line-1]
	i := sort.Search(len(segs), func(i int) bool { return segs[i].col >= col }) - 1
	if i < 0 || segs[i].src < 0 || segs[i].src >= len(m.sources) {
		return "", false
	}
	return m.sources[segs[i].src], true
}

const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// decodeVLQ decodes the base64 VLQ values of one segment.
func decodeVLQ(s string) ([]int, error) {
	var out []int
	value, shift := 0, 0
	for i := range len(s) {
		d := strings.IndexByte(base64Chars, s[i])
		if d < 0 {
			return nil, fmt.Errorf("source map: bad mapping character %q", s[i])
		}
		value += (d & 31) << shift
		if d&32 != 0 {
			shift += 5
			continue
		}
		if value&1 != 0 {
			out = append(out, -(value >> 1))
		} else {
			out = append(out, value>>1)
		}
		value, shift = 0, 0
	}
	if shift != 0 {
		return nil, fmt.Errorf("source map: truncated mapping %q", s)
	}
	return out, nil
}
