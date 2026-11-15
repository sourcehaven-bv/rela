package mapimport

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/ingest"
)

// template is a string with {{path}} references, rendered with the ingest
// interpolation: each substituted value is flattened to one line and is never
// scanned again.
type template struct {
	src   string
	paths map[string]*Path
}

func compileTemplate(src string) (*template, error) {
	t := &template{src: src, paths: map[string]*Path{}}
	rest := src
	for {
		_, after, found := strings.Cut(rest, "{{")
		if !found {
			break
		}
		ref, remainder, closed := strings.Cut(after, "}}")
		if !closed {
			return nil, errors.New("template has an unterminated {{")
		}
		ref = strings.TrimSpace(ref)
		p, err := CompilePath(ref)
		if err != nil {
			return nil, fmt.Errorf("template: %w", err)
		}
		t.paths[ref] = p
		rest = remainder
	}
	if len(t.paths) == 0 {
		return nil, errors.New("template has no {{path}} reference; use value: for a constant")
	}
	return t, nil
}

// render expands the template for one record. A reference that matches
// several values joins them with join, or is an error when join is nil.
func (t *template) render(cur, root *Node, join *string) (string, error) {
	var firstErr error
	out := ingest.Interpolate(t.src, func(ref string) string {
		texts, err := texts(t.paths[ref], cur, root)
		if err == nil && len(texts) > 1 && join == nil {
			err = fmt.Errorf("{{%s}} matched %d values; set join", ref, len(texts))
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return ""
		}
		sep := ""
		if join != nil {
			sep = *join
		}
		return strings.Join(texts, sep)
	})
	return out, firstErr
}

// texts evaluates p and returns the text of each match, dropping empty ones.
func texts(p *Path, cur, root *Node) ([]string, error) {
	if p == nil {
		return nil, nil
	}
	var out []string
	for _, n := range p.Eval(cur, root) {
		s, ok := n.text()
		if !ok {
			return nil, fmt.Errorf("%s matched a structure at %s, not a value", p, n.Location())
		}
		if s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}
