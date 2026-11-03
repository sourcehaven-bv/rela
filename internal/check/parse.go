package check

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// rawFinding is a parsed finding before it is checked against the body.
// Positions follow the contract: 1-based line and column, column and length
// in code points.
type rawFinding struct {
	Rule         string
	Severity     string
	Message      string
	Line         int
	Column       int
	Length       int
	Match        string
	Replacements []string
	Link         string
}

// valeAlert is one entry of Vale's --output=JSON. Span is 1-based and
// inclusive, in code points within Line.
type valeAlert struct {
	Action struct {
		Name   string   `json:"Name"`
		Params []string `json:"Params"`
	} `json:"Action"`
	Suggestions []string `json:"Suggestions"`
	Span        [2]int   `json:"Span"`
	Check       string   `json:"Check"`
	Link        string   `json:"Link"`
	Message     string   `json:"Message"`
	Severity    string   `json:"Severity"`
	Match       string   `json:"Match"`
	Line        int      `json:"Line"`
}

// parseVale reads Vale's JSON output: an object keyed by file path. rela
// passes exactly one file, so every key's alerts belong to the body.
func parseVale(out []byte) ([]rawFinding, error) {
	var files map[string][]valeAlert
	if err := json.Unmarshal(out, &files); err != nil {
		return nil, fmt.Errorf("parse vale output: %w", err)
	}
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var fs []rawFinding
	for _, k := range keys {
		for _, a := range files[k] {
			f := rawFinding{
				Rule:     a.Check,
				Severity: a.Severity,
				Message:  a.Message,
				Line:     a.Line,
				Column:   a.Span[0],
				Match:    a.Match,
				Link:     a.Link,
			}
			if a.Span[1] >= a.Span[0] && a.Span[0] > 0 {
				f.Length = a.Span[1] - a.Span[0] + 1
			}
			switch {
			case a.Action.Name == "replace" && len(a.Action.Params) > 0:
				f.Replacements = a.Action.Params
			case len(a.Suggestions) > 0:
				f.Replacements = a.Suggestions
			}
			fs = append(fs, f)
		}
	}
	return fs, nil
}

// relaOutput is the native format: {"findings": [...]}, positions as in the
// contract. Unknown fields are ignored so the format can grow.
type relaOutput struct {
	Findings []struct {
		Rule         string   `json:"rule"`
		Severity     string   `json:"severity"`
		Message      string   `json:"message"`
		Line         int      `json:"line"`
		Column       int      `json:"column"`
		Length       int      `json:"length"`
		Replacements []string `json:"replacements"`
		Link         string   `json:"link"`
	} `json:"findings"`
}

func parseRela(out []byte) ([]rawFinding, error) {
	dec := json.NewDecoder(bytes.NewReader(out))
	var o relaOutput
	if err := dec.Decode(&o); err != nil {
		return nil, fmt.Errorf("parse rela checker output: %w", err)
	}
	if dec.More() {
		return nil, errors.New("parse rela checker output: trailing data after the JSON object")
	}
	fs := make([]rawFinding, 0, len(o.Findings))
	for _, f := range o.Findings {
		fs = append(fs, rawFinding{
			Rule: f.Rule, Severity: f.Severity, Message: f.Message,
			Line: f.Line, Column: f.Column, Length: f.Length,
			Replacements: f.Replacements, Link: f.Link,
		})
	}
	return fs, nil
}

// resolve turns parsed findings into [Finding] values for body: it normalizes the
// severity, drops findings without a message, and checks each position
// against the body. A position that does not fit is cleared, not dropped. Match
// is always the body's own text at the position, never the tool's echo.
func resolve(checker string, raw []rawFinding, body []byte) []Finding {
	text := []rune(strings.ReplaceAll(string(body), "\r\n", "\n"))
	if !utf8.Valid(body) {
		text = nil
	}
	lineStarts := []int{0}
	for i, r := range text {
		if r == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	out := make([]Finding, 0, len(raw))
	for _, r := range raw {
		if strings.TrimSpace(r.Message) == "" {
			continue
		}
		f := Finding{
			Checker:      checker,
			Rule:         r.Rule,
			Severity:     normalizeSeverity(r.Severity),
			Message:      r.Message,
			Replacements: r.Replacements,
			Link:         r.Link,
		}
		if m, ok := textAt(text, lineStarts, r.Line, r.Column, r.Length); ok {
			f.Line, f.Column, f.Length, f.Match = r.Line, r.Column, r.Length, m
		}
		out = append(out, f)
	}
	return out
}

// textAt returns the length code points at line:column (both 1-based), or
// false when the start is not on the line or the range runs past the body.
// The range may continue past the line end: Vale reports a match that spans
// a line break (`wordt gebruik\ngemaakt`) as one span on its first line, and
// the line break counts as one code point.
func textAt(text []rune, lineStarts []int, line, column, length int) (string, bool) {
	if line < 1 || line > len(lineStarts) || column < 1 || length < 1 {
		return "", false
	}
	start := lineStarts[line-1] + column - 1
	lineEnd := len(text)
	if line < len(lineStarts) {
		lineEnd = lineStarts[line] - 1
	}
	if start >= lineEnd || start+length > len(text) {
		return "", false
	}
	return string(text[start : start+length]), true
}

func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case SeverityError:
		return SeverityError
	case SeveritySuggestion:
		return SeveritySuggestion
	default:
		return SeverityWarning
	}
}
