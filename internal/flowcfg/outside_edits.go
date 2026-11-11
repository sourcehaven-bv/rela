package flowcfg

import (
	"fmt"
	"maps"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// MaxOutsideEditRules caps the property rules of one `outside_edits:`
// level: the flow's, or one state's.
const MaxOutsideEditRules = 64

// EditKind is what an `outside_edits:` rule lets a person do with a
// property outside the flow.
type EditKind string

// The rule kinds.
const (
	// EditAllow leaves the property as acl.yaml grants it.
	EditAllow EditKind = "allow"
	// EditDeny makes the property read-only outside the flow.
	EditDeny EditKind = "deny"
	// EditPermission leaves the property writable only to a principal who
	// holds [EditRule.Permission] on the subject.
	EditPermission EditKind = "permission"
)

// EditRule is one `outside_edits:` rule.
type EditRule struct {
	Kind EditKind
	// Permission is the permission an [EditPermission] rule asks for;
	// empty for the other kinds.
	Permission string
}

// editLevel is the rules of one level: a default and per-property rules.
type editLevel struct {
	// def is nil when the level declares no `default:`.
	def   *EditRule
	props map[string]EditRule
}

// OutsideEdits is a flow's `outside_edits:`: which subject properties stay
// writable outside the flow while a run of the flow is in a state. It
// narrows field writes only; acl.yaml still decides who may write at all.
type OutsideEdits struct {
	flow   editLevel
	states map[string]editLevel
}

// Rule returns the rule for property in state: the state's rule for the
// property, else the state's default, else the flow's rule for the
// property, else the flow's default, else allow. A state the flow does
// not declare, or a final one, has no rules of its own.
//
// Nil: accepted; a nil OutsideEdits allows everything.
func (o *OutsideEdits) Rule(state, property string) EditRule {
	if o == nil {
		return EditRule{Kind: EditAllow}
	}
	if lvl, ok := o.states[state]; ok {
		if r, ok := lvl.props[property]; ok {
			return r
		}
		if lvl.def != nil {
			return *lvl.def
		}
	}
	if r, ok := o.flow.props[property]; ok {
		return r
	}
	if o.flow.def != nil {
		return *o.flow.def
	}
	return EditRule{Kind: EditAllow}
}

// Permissions returns, sorted and without repeats, the permissions the
// [EditPermission] rules of o ask for, at every level.
//
// Nil: accepted; a nil OutsideEdits asks for none.
func (o *OutsideEdits) Permissions() []string {
	if o == nil {
		return nil
	}
	var out []string
	add := func(lvl editLevel) {
		if lvl.def != nil && lvl.def.Kind == EditPermission {
			out = append(out, lvl.def.Permission)
		}
		for _, r := range lvl.props {
			if r.Kind == EditPermission {
				out = append(out, r.Permission)
			}
		}
	}
	add(o.flow)
	for _, lvl := range o.states {
		add(lvl)
	}
	return slices.Compact(slices.Sorted(slices.Values(out)))
}

// rawEditLevel is one level of `outside_edits:`. States appear only at the
// flow level; [loader.outsideEdits] refuses them in a state.
type rawEditLevel struct {
	Default    yaml.Node            `yaml:"default"`
	Properties map[string]yaml.Node `yaml:"properties"`
	States     yaml.Node            `yaml:"states"`
}

// rawEditRule is the mapping form of a rule: `{permission: P}`.
type rawEditRule struct {
	Permission string `yaml:"permission"`
}

// outsideEdits compiles a flow's `outside_edits:`, or returns nil when it
// is absent.
func (l *loader) outsideEdits(key string, n *yaml.Node, f *Flow) *OutsideEdits {
	if n.Kind == 0 || n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
		return nil
	}
	var raw rawEditLevel
	if err := decodeNode(n, &raw); err != nil {
		l.add(key, err.Error())
		return nil
	}
	out := &OutsideEdits{states: map[string]editLevel{}}
	out.flow = l.editLevel(key, &raw, f.subject)
	names, nodes, ok := l.mapping(key+".states", &raw.States)
	if !ok {
		return out
	}
	for i, name := range names {
		sk := key + ".states." + name
		st, declared := f.State(name)
		switch {
		case !declared:
			l.add(sk, fmt.Sprintf("flow %s has no state %q", f.name, name))
			continue
		case st.Final:
			l.add(sk, "is final: a run in a final state is ending, so the flow's own rules apply")
			continue
		}
		var lvl rawEditLevel
		if err := decodeNode(nodes[i], &lvl); err != nil {
			l.add(sk, err.Error())
			continue
		}
		if lvl.States.Kind != 0 {
			l.add(sk+".states", "only the flow level of outside_edits: takes states:")
			continue
		}
		out.states[name] = l.editLevel(sk, &lvl, f.subject)
	}
	if out.flow.empty() && len(out.states) == 0 {
		// No rule restricts anything; a ceiling would only cost run
		// lookups.
		l.warn(key, "declares no rules")
		return nil
	}
	return out
}

// empty reports whether the level declares no rule.
func (lvl editLevel) empty() bool { return lvl.def == nil && len(lvl.props) == 0 }

// editLevel compiles the default and property rules of one level.
func (l *loader) editLevel(key string, raw *rawEditLevel, subject string) editLevel {
	var lvl editLevel
	if raw.Default.Kind != 0 {
		if r, ok := l.editRule(key+".default", &raw.Default); ok {
			lvl.def = &r
		}
	}
	if len(raw.Properties) > MaxOutsideEditRules {
		l.add(key+".properties", fmt.Sprintf("names %d properties; at most %d", len(raw.Properties),
			MaxOutsideEditRules))
		return lvl
	}
	def, _ := l.meta.GetEntityDef(subject)
	lvl.props = make(map[string]EditRule, len(raw.Properties))
	for _, name := range slices.Sorted(maps.Keys(raw.Properties)) {
		pk := key + ".properties." + name
		pd, declared := def.Properties[name]
		switch {
		case !declared:
			l.add(pk, fmt.Sprintf("%s has no property %q", subject, name))
			continue
		case pd.Computed != "" || pd.Rollup != nil:
			l.add(pk, fmt.Sprintf("%s.%s is derived; nobody writes it", subject, name))
			continue
		case metamodel.IsReservedName(name):
			l.add(pk, fmt.Sprintf("%q is reserved; nobody writes it", name))
			continue
		}
		n := raw.Properties[name]
		if r, ok := l.editRule(pk, &n); ok {
			lvl.props[name] = r
		}
	}
	return lvl
}

// editRule compiles one rule: `allow`, `deny` or `{permission: P}`.
func (l *loader) editRule(key string, n *yaml.Node) (EditRule, bool) {
	if n.Kind == yaml.ScalarNode {
		switch EditKind(n.Value) {
		case EditAllow:
			return EditRule{Kind: EditAllow}, true
		case EditDeny:
			return EditRule{Kind: EditDeny}, true
		case EditPermission:
			// A permission rule is a mapping; the bare word is an error below.
		}
		l.add(key, fmt.Sprintf("is %q; it is allow, deny or {permission: <name>}", n.Value))
		return EditRule{}, false
	}
	var raw rawEditRule
	if err := decodeNode(n, &raw); err != nil {
		l.add(key, "is allow, deny or {permission: <name>}: "+err.Error())
		return EditRule{}, false
	}
	if raw.Permission == "" {
		l.add(key+".permission", "is required: the permission that lets a person edit outside the flow")
		return EditRule{}, false
	}
	if l.policy != nil && !l.policyGrants(raw.Permission) {
		l.warn(key+".permission", fmt.Sprintf("no role in acl.yaml grants permission %q, so nobody can edit "+
			"outside the flow", raw.Permission))
	}
	return EditRule{Kind: EditPermission, Permission: raw.Permission}, true
}

// policyGrants reports whether some role in acl.yaml grants permission.
func (l *loader) policyGrants(permission string) bool {
	for _, role := range l.policy.Roles {
		if slices.Contains(role.Permissions, permission) {
			return true
		}
	}
	return false
}

// outsideEditsNeedPolicy refuses `outside_edits:` without acl.yaml. Only
// the field write gate enforces it, and without acl.yaml no surface gates
// fields, so the rules would be silently ignored. [ParseFlow] does not
// check: a run's stored declaration compiles for its states and branches,
// and the rules always come from the current file.
func (l *loader) outsideEditsNeedPolicy() {
	if l.policy != nil {
		return
	}
	for _, name := range slices.Sorted(maps.Keys(l.c.flows)) {
		if l.c.flows[name].outsideEdits != nil {
			l.add("flows."+name+".outside_edits", "needs acl.yaml: only the field write gate enforces it, "+
				"and without acl.yaml no surface gates fields")
		}
	}
}
