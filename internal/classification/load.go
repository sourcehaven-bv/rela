package classification

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse reads a classification file and checks it for internal consistency:
// allowed keys, value shapes, names, rules, limits and label references. It
// does not know the schema; [Lint] adds the schema checks.
//
// Parse always returns a usable *File, built from the parts that parsed. The
// file is valid only when no issues are returned.
func Parse(data []byte) (*File, []Issue) {
	p := &parser{file: newFile()}
	if len(data) > MaxFileSize {
		p.add(CodeLimit, "", 0, "file is %d bytes; the limit is %d", len(data), MaxFileSize)
		return p.file, p.issues
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		p.add(CodeSyntax, "", 0, "%v", err)
		return p.file, p.issues
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return p.file, nil // empty file
	}
	if !p.checkYAMLFeatures(doc.Content[0], "") {
		return p.file, p.issues
	}
	p.root(doc.Content[0])
	p.resolveReferences()
	return p.file, p.issues
}

type parser struct {
	file   *File
	issues []Issue
}

func (p *parser) add(code, path string, line int, format string, args ...any) {
	p.issues = append(p.issues, Issue{Code: code, Path: path, Line: line, Message: fmt.Sprintf(format, args...)})
}

// checkYAMLFeatures rejects anchors, aliases and merge keys (an edit through
// one would change several places, and sync edits the file) and, in every
// mapping, duplicate and non-string keys.
func (p *parser) checkYAMLFeatures(n *yaml.Node, path string) bool {
	ok := true
	if n.Anchor != "" || n.Kind == yaml.AliasNode {
		p.add(CodeYAMLFeature, path, n.Line, "YAML anchors and aliases are not supported")
		return false
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Tag == "!!merge" {
				p.add(CodeYAMLFeature, path, k.Line, "YAML merge keys (<<) are not supported")
				ok = false
				continue
			}
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
				p.add(CodeInvalidValue, path, k.Line, "keys must be strings; quote %q", k.Value)
				ok = false
				continue
			}
			if seen[k.Value] {
				p.add(CodeDuplicateKey, join(path, k.Value), k.Line, "duplicate key %q", k.Value)
				ok = false
			}
			seen[k.Value] = true
			ok = p.checkYAMLFeatures(n.Content[i+1], join(path, k.Value)) && ok
		}
		return ok
	}
	for i, c := range n.Content {
		ok = p.checkYAMLFeatures(c, path+"["+strconv.Itoa(i)+"]") && ok
	}
	return ok
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// pairs returns a mapping node's key/value pairs, or reports that the node is
// not a mapping. A null value (an empty `key:`) is an empty mapping.
func (p *parser) pairs(n *yaml.Node, path string) ([][2]*yaml.Node, bool) {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
		return nil, true
	}
	if n.Kind != yaml.MappingNode {
		p.add(CodeInvalidValue, path, n.Line, "expected a mapping")
		return nil, false
	}
	out := make([][2]*yaml.Node, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		out = append(out, [2]*yaml.Node{n.Content[i], n.Content[i+1]})
	}
	return out, true
}

func (p *parser) scalar(n *yaml.Node, path string) (string, bool) {
	if n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		p.add(CodeInvalidValue, path, n.Line, "expected a single value")
		return "", false
	}
	return n.Value, true
}

func (p *parser) root(n *yaml.Node) {
	top, ok := p.pairs(n, "")
	if !ok {
		return
	}
	for _, kv := range top {
		k, v := kv[0], kv[1]
		switch k.Value {
		case "labels":
			p.labels(v)
		case "assign":
			p.assignments(v, "assign", p.file.Assign)
		case "assign_relations":
			p.assignments(v, "assign_relations", p.file.AssignRelations)
		case "subject":
			p.boolOverrides(v, "subject", p.file.Overrides.Subject)
		case "subject_link":
			p.boolOverrides(v, "subject_link", p.file.Overrides.SubjectLink)
		case "subject_hops":
			p.hopOverrides(v)
		default:
			p.add(CodeUnknownKey, k.Value, k.Line,
				"unknown key; allowed: labels, assign, assign_relations, subject, subject_link, subject_hops")
		}
	}
}

func (p *parser) labels(n *yaml.Node) {
	entries, ok := p.pairs(n, "labels")
	if !ok {
		return
	}
	if len(entries) > MaxLabels {
		// Only the first MaxLabels are read, so references to them still
		// resolve rather than each reporting an undefined label.
		p.add(CodeLimit, "labels", n.Line, "%d labels defined; the limit is %d", len(entries), MaxLabels)
		entries = entries[:MaxLabels]
	}
	for _, kv := range entries {
		name, path := kv[0].Value, join("labels", kv[0].Value)
		switch {
		case reservedName(name):
			p.add(CodeReservedName, path, kv[0].Line, "%q is reserved and cannot be a label name", name)
			continue
		case !labelNamePattern.MatchString(name):
			p.add(CodeInvalidName, path, kv[0].Line,
				"label names use lowercase letters, digits and hyphens, and start with a letter or digit (max 64)")
			continue
		}
		p.file.Labels[name] = p.label(name, kv[1], path, kv[0].Line)
		p.file.LabelOrder = append(p.file.LabelOrder, name)
	}
}

func (p *parser) label(name string, n *yaml.Node, path string, line int) *Label {
	l := &Label{Name: name, Line: line}
	fields, ok := p.pairs(n, path)
	if !ok {
		return l
	}
	for _, kv := range fields {
		k, v, kpath := kv[0], kv[1], join(path, kv[0].Value)
		switch k.Value {
		case "role":
			if s, ok := p.scalar(v, kpath); ok {
				role, valid := parseRole(s)
				if !valid {
					p.add(CodeInvalidValue, kpath, v.Line,
						"unknown role %q; allowed: direct-identifier, quasi-identifier, attribute", s)
				}
				l.Role = role
			}
		case "description":
			l.Description, _ = p.scalar(v, kpath)
		case "reference":
			l.Reference, _ = p.scalar(v, kpath)
		case "meta":
			if v.Kind != yaml.MappingNode {
				p.add(CodeInvalidValue, kpath, v.Line, "meta must be a mapping")
				continue
			}
			meta := map[string]any{}
			if err := v.Decode(&meta); err != nil {
				p.add(CodeInvalidValue, kpath, v.Line, "%v", err)
				continue
			}
			l.Meta = meta
		case "when":
			l.When = p.rule(v, kpath)
		default:
			p.add(CodeUnknownKey, kpath, k.Line,
				"unknown key; allowed: role, description, reference, meta, when")
		}
	}
	return l
}

// rule parses `when:`: a scope plus exactly one condition key.
func (p *parser) rule(n *yaml.Node, path string) *Rule {
	fields, ok := p.pairs(n, path)
	if !ok {
		return nil
	}
	r := &Rule{Scope: ScopeRecord}
	condNode := &yaml.Node{Kind: yaml.MappingNode, Line: n.Line}
	for _, kv := range fields {
		k, v := kv[0], kv[1]
		switch k.Value {
		case "scope":
			s, isScalar := p.scalar(v, join(path, "scope"))
			if !isScalar {
				continue
			}
			switch sc := Scope(s); sc {
			case ScopeRecord, ScopeSubject:
				r.Scope = sc
			default:
				p.add(CodeInvalidValue, join(path, "scope"), v.Line, "unknown scope %q; allowed: record, subject", s)
			}
		case "any_of", "all_of", "count":
			condNode.Content = append(condNode.Content, k, v)
		default:
			p.add(CodeUnknownKey, join(path, k.Value), k.Line, "unknown key; allowed: scope, any_of, all_of, count")
		}
	}
	c, ok := p.cond(condNode, path, 1)
	if !ok {
		return nil
	}
	r.cond = c
	return r
}

// cond parses a condition: a selector string, or a mapping with exactly one
// of any_of, all_of, count.
func (p *parser) cond(n *yaml.Node, path string, depth int) (cond, bool) {
	if depth > MaxRuleDepth {
		p.add(CodeLimit, path, n.Line, "rule is nested deeper than %d levels", MaxRuleDepth)
		return cond{}, false
	}
	if n.Kind == yaml.ScalarNode {
		sel, ok := p.selector(n, path)
		return cond{kind: condSelector, sel: sel}, ok
	}
	fields, ok := p.pairs(n, path)
	if !ok {
		return cond{}, false
	}
	if len(fields) != 1 {
		p.add(CodeInvalidRule, path, n.Line, "a condition needs exactly one of any_of, all_of, count")
		return cond{}, false
	}
	k, v := fields[0][0], fields[0][1]
	kpath := join(path, k.Value)
	switch k.Value {
	case "any_of", "all_of":
		if v.Kind != yaml.SequenceNode || len(v.Content) == 0 {
			p.add(CodeInvalidRule, kpath, v.Line, "%s needs a non-empty list", k.Value)
			return cond{}, false
		}
		c := cond{kind: condAllOf}
		if k.Value == "any_of" {
			c.kind = condAnyOf
		}
		ok := true
		for i, item := range v.Content {
			child, childOK := p.cond(item, kpath+"["+strconv.Itoa(i)+"]", depth+1)
			ok = ok && childOK
			c.children = append(c.children, child)
		}
		return c, ok
	case "count":
		return p.count(v, kpath)
	}
	p.add(CodeUnknownKey, kpath, k.Line, "unknown key; allowed: any_of, all_of, count")
	return cond{}, false
}

func (p *parser) count(n *yaml.Node, path string) (cond, bool) {
	fields, ok := p.pairs(n, path)
	if !ok {
		return cond{}, false
	}
	c := cond{kind: condCount}
	var haveOf, haveMin bool
	for _, kv := range fields {
		k, v, kpath := kv[0], kv[1], join(path, kv[0].Value)
		switch k.Value {
		case "of":
			sel, selOK := p.selector(v, kpath)
			c.sel, haveOf, ok = sel, true, ok && selOK
		case "min":
			n, err := strconv.Atoi(v.Value)
			if v.Kind != yaml.ScalarNode || v.Tag != "!!int" || err != nil || n < 1 || n > maxCountMin {
				p.add(CodeInvalidRule, kpath, v.Line, "min must be a whole number from 1 to %d", maxCountMin)
				ok = false
				continue
			}
			c.min, haveMin = n, true
		default:
			p.add(CodeUnknownKey, kpath, k.Line, "unknown key; allowed: of, min")
			ok = false
		}
	}
	if !haveOf || !haveMin {
		p.add(CodeInvalidRule, path, n.Line, "count needs both of and min")
		return cond{}, false
	}
	return c, ok
}

func (p *parser) selector(n *yaml.Node, path string) (selector, bool) {
	s, ok := p.scalar(n, path)
	if !ok {
		return selector{}, false
	}
	if role, isRole := strings.CutPrefix(s, "@"); isRole {
		r, valid := parseRole(role)
		if !valid {
			p.add(CodeInvalidRule, path, n.Line,
				"unknown role selector %q; allowed: @direct-identifier, @quasi-identifier, @attribute", s)
			return selector{}, false
		}
		return selector{role: r}, true
	}
	return selector{label: s}, true
}

// assignments parses `assign:` or `assign_relations:` into dst.
func (p *parser) assignments(n *yaml.Node, section string, dst map[string]TypeAssignments) {
	types, ok := p.pairs(n, section)
	if !ok {
		return
	}
	for _, tkv := range types {
		typeName, tpath := tkv[0].Value, join(section, tkv[0].Value)
		ta := TypeAssignments{Fields: map[string]Assignment{}, Line: tkv[0].Line}
		fields, ok := p.pairs(tkv[1], tpath)
		if ok {
			for _, fkv := range fields {
				if a, ok := p.assignment(fkv[1], join(tpath, fkv[0].Value)); ok {
					a.Line = fkv[0].Line
					ta.Fields[fkv[0].Value] = a
				}
			}
		}
		dst[typeName] = ta
	}
}

func (p *parser) assignment(n *yaml.Node, path string) (Assignment, bool) {
	if n.Kind == yaml.ScalarNode && n.Tag != "!!null" {
		switch n.Value {
		case StateNone:
			return Assignment{State: None}, true
		case StateNeedsReview:
			return Assignment{State: NeedsReview}, true
		}
		p.add(CodeInvalidValue, path, n.Line,
			"expected a list of labels, %q or %q; write a single label as [%s]", StateNone, StateNeedsReview, n.Value)
		return Assignment{}, false
	}
	if n.Kind != yaml.SequenceNode {
		p.add(CodeInvalidValue, path, n.Line, "expected a list of labels, %q or %q", StateNone, StateNeedsReview)
		return Assignment{}, false
	}
	if len(n.Content) == 0 {
		p.add(CodeInvalidValue, path, n.Line, "an empty list is ambiguous; write %q for a field without labels", StateNone)
		return Assignment{}, false
	}
	a := Assignment{State: Labeled}
	for i, item := range n.Content {
		name, ok := p.scalar(item, path+"["+strconv.Itoa(i)+"]")
		if !ok {
			return Assignment{}, false
		}
		if !slices.Contains(a.Labels, name) {
			a.Labels = append(a.Labels, name)
		}
	}
	return a, true
}

func (p *parser) boolOverrides(n *yaml.Node, section string, dst map[string]bool) {
	entries, ok := p.pairs(n, section)
	if !ok {
		return
	}
	for _, kv := range entries {
		path := join(section, kv[0].Value)
		v := kv[1]
		if v.Kind != yaml.ScalarNode || v.Tag != "!!bool" {
			p.add(CodeInvalidValue, path, v.Line, "expected true or false")
			continue
		}
		dst[kv[0].Value] = strings.EqualFold(v.Value, "true")
		p.file.Overrides.Lines[path] = kv[0].Line
	}
}

func (p *parser) hopOverrides(n *yaml.Node) {
	entries, ok := p.pairs(n, "subject_hops")
	if !ok {
		return
	}
	for _, kv := range entries {
		path := join("subject_hops", kv[0].Value)
		v := kv[1]
		hops, err := strconv.Atoi(v.Value)
		if v.Kind != yaml.ScalarNode || v.Tag != "!!int" || err != nil || hops < 1 || hops > MaxSubjectHops {
			p.add(CodeLimit, path, v.Line, "subject_hops must be a whole number from 1 to %d", MaxSubjectHops)
			continue
		}
		p.file.Overrides.SubjectHops[kv[0].Value] = hops
		p.file.Overrides.Lines[path] = kv[0].Line
	}
}

// resolveReferences reports labels used in assignments or rules that are not
// defined under `labels:`.
func (p *parser) resolveReferences() {
	f := p.file
	for _, name := range f.LabelOrder {
		l := f.Labels[name]
		if l.When == nil {
			continue
		}
		for _, ref := range l.When.cond.referencedLabels() {
			if _, ok := f.Labels[ref]; !ok {
				p.add(CodeUndefined, join("labels", name)+".when", l.Line, "label %q is not defined", ref)
			}
		}
	}
	for _, section := range []struct {
		name string
		m    map[string]TypeAssignments
	}{{"assign", f.Assign}, {"assign_relations", f.AssignRelations}} {
		for _, typeName := range sortedKeys(section.m) {
			ta := section.m[typeName]
			for _, field := range sortedKeys(ta.Fields) {
				for _, ref := range ta.Fields[field].Labels {
					l, ok := f.Labels[ref]
					switch {
					case !ok:
						p.add(CodeUndefined, section.name+"."+typeName+"."+field, ta.Fields[field].Line,
							"label %q is not defined", ref)
					case l.When != nil:
						p.add(CodeInvalidValue, section.name+"."+typeName+"."+field, ta.Fields[field].Line,
							"label %q is derived from its rule and cannot be assigned to a field", ref)
					}
				}
			}
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
