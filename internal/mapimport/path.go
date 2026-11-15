package mapimport

import (
	"errors"
	"fmt"
	"strings"
)

// Path is a compiled selector. The language is deliberately small; it has no
// functions and no arithmetic, and evaluation is linear in the tree:
//
//	$          the document root (only as the first step)
//	.          the current node (alone)
//	name       the field `name`; an XML child element or `@attribute`
//	['a b']    the field `a b`, for a name with other characters
//	*          every field value, or every array item
//	..name     the field `name` at any depth below, the current node included
//	^          the nearest enclosing object
//	[f=value]  keep nodes whose field f has this text ('value' may be quoted)
//	[f]        keep nodes that have the field f
//
// Steps are joined with dots: `catalog..controls`, `props[name=label].value`,
// `^.id`. Arrays flatten: a step that reaches an array yields its items.
type Path struct {
	src      string
	absolute bool
	steps    []step
}

type stepKind int

const (
	stepField stepKind = iota
	stepAll
	stepDescend
	stepParent
	stepFilter
)

type step struct {
	kind     stepKind
	name     string
	value    string
	hasValue bool
}

func (p *Path) String() string { return p.src }

// CompilePath parses a path.
func CompilePath(src string) (*Path, error) {
	s := strings.TrimSpace(src)
	p := &Path{src: s}
	if s == "" {
		return nil, errors.New("empty path")
	}
	if s == "." {
		return p, nil
	}
	if strings.HasPrefix(s, "$") {
		p.absolute = true
		s = s[1:]
		if s == "" {
			return p, nil
		}
		if s[0] != '.' && s[0] != '[' {
			return nil, fmt.Errorf("path %q: expected . or [ after $", src)
		}
	}
	first := true
	for s != "" {
		rest, isStep, err := p.parseStep(s, first)
		if err != nil {
			return nil, fmt.Errorf("path %q: %w", src, err)
		}
		s = rest
		if !isStep {
			continue
		}
		first = false
		if s != "" && s[0] != '.' && s[0] != '[' {
			return nil, fmt.Errorf("path %q: unexpected %q", src, s[:1])
		}
	}
	return p, nil
}

// parseStep parses the step at the start of s and returns the rest. A
// separating dot is consumed with isStep false.
func (p *Path) parseStep(s string, first bool) (tail string, isStep bool, err error) {
	switch {
	case strings.HasPrefix(s, ".."):
		name, rest := cutName(s[2:])
		if name == "" {
			return "", false, errors.New(".. needs a field name")
		}
		p.steps = append(p.steps, step{kind: stepDescend, name: name})
		return rest, true, nil
	case s[0] == '.':
		if first && !p.absolute {
			return "", false, errors.New("a relative path starts with a name")
		}
		rest := s[1:]
		if rest == "" {
			return "", false, errors.New("ends with a dot")
		}
		if rest[0] == '.' || rest[0] == '[' {
			return "", false, errors.New("empty step")
		}
		return rest, false, nil
	case strings.HasPrefix(s, "['") || strings.HasPrefix(s, `["`):
		name, rest, err := quotedName(s)
		if err != nil {
			return "", false, err
		}
		p.steps = append(p.steps, step{kind: stepField, name: name})
		return rest, true, nil
	case s[0] == '[':
		st, rest, err := parseFilter(s)
		if err != nil {
			return "", false, err
		}
		if len(p.steps) == 0 && !p.absolute {
			return "", false, errors.New("a filter follows a step")
		}
		p.steps = append(p.steps, st)
		return rest, true, nil
	case s[0] == '*':
		p.steps = append(p.steps, step{kind: stepAll})
		return s[1:], true, nil
	case s[0] == '^':
		p.steps = append(p.steps, step{kind: stepParent})
		return s[1:], true, nil
	}
	name, rest := cutName(s)
	if name == "" {
		return "", false, fmt.Errorf("unexpected %q", s[:1])
	}
	p.steps = append(p.steps, step{kind: stepField, name: name})
	return rest, true, nil
}

// isNameByte reports the bytes a field name may hold. `@` and `#` start XML
// attribute and text names; `:` and `-` occur in XML and JSON names.
func isNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '_' || c == '-' || c == ':' || c == '@' || c == '#'
}

func cutName(s string) (name, rest string) {
	i := 0
	for i < len(s) && isNameByte(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

// quotedName parses a `['name']` step: any text between the quotes.
func quotedName(s string) (name, rest string, err error) {
	q := s[1]
	end := strings.IndexByte(s[2:], q)
	if end < 0 || !strings.HasPrefix(s[2+end+1:], "]") {
		return "", "", errors.New("unterminated [" + string(q))
	}
	name = s[2 : 2+end]
	if name == "" {
		return "", "", errors.New("empty quoted name")
	}
	return name, s[2+end+2:], nil
}

// filterBody returns the text between [ and its ]. A quoted value may hold
// a ].
func filterBody(s string) (body, rest string, err error) {
	var quote byte
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == ']':
			return s[1:i], s[i+1:], nil
		}
	}
	return "", "", errors.New("unterminated [")
}

func parseFilter(s string) (step, string, error) {
	body, rest, err := filterBody(s)
	if err != nil {
		return step{}, "", err
	}
	field, value, hasValue := strings.Cut(body, "=")
	field = strings.TrimSpace(field)
	if name, tail := cutName(field); name == "" || tail != "" {
		return step{}, "", fmt.Errorf("filter [%s] needs a field name", body)
	}
	if hasValue {
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '\'' || value[0] == '"') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		} else if strings.ContainsAny(value, `'"`) {
			return step{}, "", fmt.Errorf("filter [%s] has an unbalanced quote", body)
		}
	}
	return step{kind: stepFilter, name: field, value: value, hasValue: hasValue}, rest, nil
}

// Eval returns the nodes p selects from cur (or from root for a `$` path),
// in document order.
func (p *Path) Eval(cur, root *Node) []*Node {
	start := cur
	if p.absolute {
		start = root
	}
	if start == nil {
		return nil
	}
	nodes := flatten(nil, start)
	for _, st := range p.steps {
		nodes = st.apply(nodes)
		if len(nodes) == 0 {
			return nil
		}
	}
	return nodes
}

func (st step) apply(in []*Node) []*Node {
	var out []*Node
	switch st.kind {
	case stepField:
		for _, n := range in {
			out = flatten(out, n.get(st.name))
		}
	case stepAll:
		for _, n := range in {
			switch n.kind {
			case kindObject:
				for _, k := range n.keys {
					out = flatten(out, n.fields[k])
				}
			case kindArray:
				out = flatten(out, n)
			case kindScalar:
			}
		}
	case stepDescend:
		// One visited set for the whole step: a node reached from two
		// inputs is searched and yielded once.
		visited := map[*Node]bool{}
		for _, n := range in {
			out = descend(out, n, st.name, visited)
		}
	case stepParent:
		seen := map[*Node]bool{}
		for _, n := range in {
			p := n.parent
			for p != nil && p.kind == kindArray {
				p = p.parent
			}
			if p != nil && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	case stepFilter:
		for _, n := range in {
			if st.keeps(n) {
				out = append(out, n)
			}
		}
	}
	return out
}

func (st step) keeps(n *Node) bool {
	vals := flatten(nil, n.get(st.name))
	if !st.hasValue {
		return len(vals) > 0
	}
	for _, v := range vals {
		if t, ok := v.text(); ok && t == st.value {
			return true
		}
	}
	return false
}

// flatten appends v to out, or v's items (recursively) when v is an array.
func flatten(out []*Node, v *Node) []*Node {
	if v == nil {
		return out
	}
	if v.kind != kindArray {
		return append(out, v)
	}
	for _, it := range v.items {
		out = flatten(out, it)
	}
	return out
}

// descend appends the values of every field called name in n's subtree, n
// included, in document order. A match is searched too, so nested controls
// inside a control are found. A node in visited is skipped with its subtree,
// which was searched when it was first visited.
func descend(out []*Node, n *Node, name string, visited map[*Node]bool) []*Node {
	if visited[n] {
		return out
	}
	visited[n] = true
	switch n.kind {
	case kindObject:
		for _, k := range n.keys {
			v := n.fields[k]
			if k != name {
				out = descend(out, v, name, visited)
				continue
			}
			for _, it := range flatten(nil, v) {
				if visited[it] {
					continue
				}
				out = append(out, it)
				out = descend(out, it, name, visited)
			}
		}
	case kindArray:
		for _, it := range n.items {
			out = descend(out, it, name, visited)
		}
	case kindScalar:
	}
	return out
}
