package triggers

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// rawFile is the part of flows.yaml this package reads. The other top-level
// keys belong to internal/flowcfg, which checks them.
type rawFile struct {
	Triggers yaml.Node `yaml:"triggers"`
}

// rawRule is one entry under `triggers:`.
type rawRule struct {
	Description string      `yaml:"description"`
	On          rawOn       `yaml:"on"`
	Outputs     []yaml.Node `yaml:"outputs"`
}

// rawOn is a rule's `on:` block. `where:` is filter syntax and `when:` a
// predicate expression; they are separate keys because the two syntaxes
// overlap without erroring (see .claude/rules/predicate.md).
type rawOn struct {
	Entity          metamodel.StringOrSlice `yaml:"entity"`
	Property        string                  `yaml:"property"`
	Becomes         string                  `yaml:"becomes"`
	From            string                  `yaml:"from"`
	Created         bool                    `yaml:"created"`
	Updated         bool                    `yaml:"updated"`
	Deleted         bool                    `yaml:"deleted"`
	RelationCreated string                  `yaml:"relation_created"`
	RelationRemoved string                  `yaml:"relation_removed"`
	Faces           metamodel.StringOrSlice `yaml:"faces"`
	Copy            metamodel.StringOrSlice `yaml:"copy"`
	Where           []string                `yaml:"where"`
	When            string                  `yaml:"when"`
}

// rawOutput is one entry of `outputs:`. Exactly one key is set.
type rawOutput struct {
	Set       *rawSet    `yaml:"set"`
	Relate    *rawRelate `yaml:"relate"`
	Create    *rawCreate `yaml:"create"`
	AddToPile *rawPile   `yaml:"add_to_pile"`
	Notify    *rawNotify `yaml:"notify"`
	Script    *rawScript `yaml:"script"`
	Check     *rawCheck  `yaml:"check"`
}

type rawSet struct {
	Properties yaml.Node `yaml:"properties"`
}

type rawRelate struct {
	Type string `yaml:"type"`
	To   string `yaml:"to"`
}

type rawCreate struct {
	Type       string           `yaml:"type"`
	Template   string           `yaml:"template"`
	Properties yaml.Node        `yaml:"properties"`
	Relate     *rawCreateRelate `yaml:"relate"`
	IfExists   string           `yaml:"if_exists"`
}

// rawCreateRelate links the created entity to the subject. `from: subject`
// is the only direction a trigger supports: the relation leads from the
// triggering entity to the created one.
type rawCreateRelate struct {
	Type string `yaml:"type"`
	From string `yaml:"from"`
}

type rawPile struct {
	Pile   string `yaml:"pile"`
	Owner  string `yaml:"owner"`
	Create *bool  `yaml:"create"`
}

type rawNotify struct {
	Pile   string        `yaml:"pile"`
	Reason string        `yaml:"reason"`
	To     rawNotifyPath `yaml:"to"`
}

type rawNotifyPath struct {
	Via      metamodel.StringOrSlice `yaml:"via"`
	Type     string                  `yaml:"type"`
	Property string                  `yaml:"property"`
}

type rawScript struct {
	Lua            string                 `yaml:"lua"`
	File           string                 `yaml:"file"`
	Capabilities   metamodel.Capabilities `yaml:"capabilities"`
	AllowACLBypass metamodel.ACLBypass    `yaml:"allow_acl_bypass"`
	Background     bool                   `yaml:"background"`
	RunAs          string                 `yaml:"run_as"`
	Retry          metamodel.JobRetry     `yaml:"retry"`
}

type rawCheck struct {
	Where    string `yaml:"where"`
	Severity string `yaml:"severity"`
	Message  string `yaml:"message"`
}

// FromSubject is the only `create.relate.from` a trigger takes.
const FromSubject = "subject"

// Parse decodes the `triggers:` block of a flows.yaml document and checks
// every rule against meta. It returns the rules in document order, which is
// the order they fire in. A document without `triggers:` yields no rules.
//
// Every problem is a load error, never a skip: a dropped `where:` clause or
// scope would widen or silence a rule without a word.
//
// Nil: meta is rejected — the checks need the schema.
func Parse(data []byte, meta *metamodel.Metamodel) ([]Rule, error) {
	if meta == nil {
		return nil, errors.New("triggers: Parse requires a metamodel")
	}
	var raw rawFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("flows.yaml: %w", err)
	}
	n := &raw.Triggers
	if n.Kind == 0 || (n.Kind == yaml.ScalarNode && n.Tag == "!!null") {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("flows.yaml: line %d: `triggers:` must be a mapping of names to rules", n.Line)
	}
	if hasAlias(n) {
		return nil, errors.New("flows.yaml: `triggers:` may not use YAML anchors or aliases")
	}
	var (
		rules []Rule
		errs  []string
		seen  = map[string]bool{}
	)
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		name := key.Value
		where := fmt.Sprintf("trigger %q", name)
		if msg := nameProblem(name); msg != "" {
			errs = append(errs, fmt.Sprintf("line %d: %s: %s", key.Line, where, msg))
			continue
		}
		if seen[name] {
			errs = append(errs, fmt.Sprintf("line %d: %s is declared twice", key.Line, where))
			continue
		}
		seen[name] = true
		rule, ruleErrs := decodeRule(name, val)
		if len(ruleErrs) > 0 {
			errs = append(errs, ruleErrs...)
			continue
		}
		rules = append(rules, rule)
	}
	if len(errs) == 0 {
		expandAbstractTypes(meta, rules)
		errs = append(errs, Check(meta, rules)...)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("flows.yaml: triggers:\n  %s", strings.Join(errs, "\n  "))
	}
	return rules, nil
}

// nameProblem returns why name cannot name a trigger, or "". The rule is the
// one automation names had: a name is kept byte for byte when automations
// move from schema.yaml, because audit records (`automation:<name>`) and
// queued background jobs refer to it.
func nameProblem(name string) string {
	switch {
	case strings.TrimSpace(name) == "":
		return "a trigger needs a name"
	case strings.ContainsAny(name, ".:"):
		return "a trigger name may not contain `.` or `:`"
	}
	return ""
}

// decodeRule decodes one rule strictly and converts it to a [Rule].
func decodeRule(name string, n *yaml.Node) (rule Rule, errs []string) {
	where := fmt.Sprintf("trigger %q", name)
	var raw rawRule
	if err := decodeNode(n, &raw); err != nil {
		return Rule{}, []string{fmt.Sprintf("%s: %v", where, err)}
	}
	whenFilters := make([]*filter.Filter, 0, len(raw.On.Where))
	for _, w := range raw.On.Where {
		f, err := filter.Parse(w)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: where clause %q: %v", where, w, err))
			continue
		}
		whenFilters = append(whenFilters, f)
	}
	rule = Rule{
		Name:        name,
		Description: raw.Description,
		On: Trigger{
			Entity:          []string(raw.On.Entity),
			Property:        raw.On.Property,
			Becomes:         raw.On.Becomes,
			From:            raw.On.From,
			Created:         raw.On.Created,
			Updated:         raw.On.Updated,
			RelationCreated: raw.On.RelationCreated,
			RelationRemoved: raw.On.RelationRemoved,
			Deleted:         raw.On.Deleted,
			Faces:           []string(raw.On.Faces),
			When:            whenFilters,
			Condition:       raw.On.When,
			Copy:            []string(raw.On.Copy),
		},
	}
	for i := range raw.Outputs {
		at := fmt.Sprintf("%s: outputs[%d]", where, i)
		actions, check, outErrs := decodeOutput(at, &raw.Outputs[i])
		errs = append(errs, outErrs...)
		rule.Do = append(rule.Do, actions...)
		if check != nil {
			rule.Validate = append(rule.Validate, *check)
		}
	}
	errs = append(errs, placeholderErrors(where, rule)...)
	return rule, errs
}

// decodeOutput converts one output entry into engine actions (a `set` with
// several properties becomes one action per property, in document order) or
// a check.
func decodeOutput(at string, n *yaml.Node) (actions []Action, check *Validation, errs []string) {
	var o rawOutput
	if err := decodeNode(n, &o); err != nil {
		return nil, nil, []string{fmt.Sprintf("%s: %v", at, err)}
	}
	set := 0
	for _, p := range []bool{
		o.Set != nil, o.Relate != nil, o.Create != nil, o.AddToPile != nil,
		o.Notify != nil, o.Script != nil, o.Check != nil,
	} {
		if p {
			set++
		}
	}
	if set != 1 {
		return nil, nil, []string{at + ": an output holds exactly one of set, relate, create, " +
			"add_to_pile, notify, script or check"}
	}
	switch {
	case o.Set != nil:
		props, err := scalarMap(&o.Set.Properties)
		if err != nil {
			return nil, nil, []string{fmt.Sprintf("%s: set.properties: %v", at, err)}
		}
		if len(props) == 0 {
			return nil, nil, []string{at + ": set needs `properties:`"}
		}
		actions := make([]Action, 0, len(props))
		for _, kv := range props {
			actions = append(actions, Action{Set: kv[0], Value: kv[1]})
		}
		return actions, nil, nil
	case o.Relate != nil:
		return []Action{{CreateRelation: &CreateRelationAction{Relation: o.Relate.Type, To: o.Relate.To}}}, nil, nil
	case o.Create != nil:
		return decodeCreate(at, o.Create)
	case o.AddToPile != nil:
		p := o.AddToPile
		return []Action{{AddToPile: &AddToPileAction{
			Pile: p.Pile, Owner: p.Owner, Create: p.Create == nil || *p.Create,
		}}}, nil, nil
	case o.Notify != nil:
		nt := o.Notify
		return []Action{{Notify: &NotifyAction{
			Pile: nt.Pile, Reason: nt.Reason, Via: []string(nt.To.Via), Type: nt.To.Type, Property: nt.To.Property,
		}}}, nil, nil
	case o.Script != nil:
		s := o.Script
		if (s.Lua == "") == (s.File == "") {
			return nil, nil, []string{at + ": script needs exactly one of `lua:` and `file:`"}
		}
		return []Action{{
			Lua: s.Lua, LuaFile: s.File, Capabilities: s.Capabilities, AllowACLBypass: s.AllowACLBypass,
			Background: s.Background, RunAs: s.RunAs, Retry: s.Retry,
		}}, nil, nil
	default:
		c := o.Check
		if strings.TrimSpace(c.Message) == "" {
			return nil, nil, []string{at + ": check needs `message:`"}
		}
		return nil, &Validation{Check: c.Where, Severity: c.Severity, Message: c.Message}, nil
	}
}

func decodeCreate(at string, c *rawCreate) (actions []Action, check *Validation, errs []string) {
	props, err := scalarMap(&c.Properties)
	if err != nil {
		return nil, nil, []string{fmt.Sprintf("%s: create.properties: %v", at, err)}
	}
	if strings.TrimSpace(c.Type) == "" {
		errs = append(errs, at+": create needs `type:`")
	}
	switch c.IfExists {
	case "", IfExistsSkip, IfExistsError, IfExistsReplace:
	default:
		errs = append(errs, fmt.Sprintf("%s: create.if_exists %q is not one of %s, %s or %s",
			at, c.IfExists, IfExistsSkip, IfExistsError, IfExistsReplace))
	}
	ce := &CreateEntityAction{Type: c.Type, Template: c.Template, IfExists: c.IfExists}
	if c.Relate != nil {
		if c.Relate.From != FromSubject {
			errs = append(errs, fmt.Sprintf("%s: create.relate.from must be %q: the relation leads from the "+
				"triggering entity to the created one", at, FromSubject))
		}
		if strings.TrimSpace(c.Relate.Type) == "" {
			errs = append(errs, at+": create.relate needs `type:`")
		}
		ce.Relation = c.Relate.Type
	}
	if len(props) > 0 {
		ce.Properties = make(map[string]string, len(props))
		for _, kv := range props {
			ce.Properties[kv[0]] = kv[1]
		}
	}
	if len(errs) > 0 {
		return nil, nil, errs
	}
	return []Action{{CreateEntity: ce}}, nil, nil
}

// scalarMap reads a mapping of scalars in document order. A null value reads
// as "", as it did when the value was decoded into a string.
func scalarMap(n *yaml.Node) ([][2]string, error) {
	if n.Kind == 0 || (n.Kind == yaml.ScalarNode && n.Tag == "!!null") {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, errors.New("must be a mapping of property names to values")
	}
	out := make([][2]string, 0, len(n.Content)/2)
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if v.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("property %q: a value is a scalar or a template", k.Value)
		}
		if seen[k.Value] {
			return nil, fmt.Errorf("property %q is set twice", k.Value)
		}
		seen[k.Value] = true
		val := v.Value
		if v.Tag == "!!null" {
			val = ""
		}
		out = append(out, [2]string{k.Value, val})
	}
	return out, nil
}

// placeholderErrors refuses a template placeholder the engine does not
// replace. Such a placeholder used to be written as literal text, so a
// typo, or the state-output spelling `{{subject.title}}`, wrote `{{...}}`
// into the data without a word.
func placeholderErrors(where string, r Rule) []string {
	var errs []string
	check := func(field, tmpl string) {
		for _, ph := range placeholders(tmpl) {
			if !knownPlaceholder(ph) {
				errs = append(errs, fmt.Sprintf("%s: %s: unknown placeholder {{%s}}; use new.<property>, "+
					"old.<property>, entity.id, entity.type, user.name, user.email, today or now", where, field, ph))
			}
		}
	}
	for i, a := range r.Do {
		at := fmt.Sprintf("action %d", i+1)
		check(at+" set", a.Value)
		if a.CreateRelation != nil {
			check(at+" relate.to", a.CreateRelation.To)
		}
		if a.CreateEntity != nil {
			check(at+" create.template", a.CreateEntity.Template)
			for _, k := range sortedKeys(a.CreateEntity.Properties) {
				check(at+" create.properties."+k, a.CreateEntity.Properties[k])
			}
		}
		if a.AddToPile != nil {
			check(at+" add_to_pile.pile", a.AddToPile.Pile)
			check(at+" add_to_pile.owner", a.AddToPile.Owner)
		}
	}
	for i, v := range r.Validate {
		check(fmt.Sprintf("check %d message", i+1), v.Message)
	}
	return errs
}

// placeholders returns the contents of every `{{...}}` in s, as written.
// They are not trimmed: Interpolate replaces only the exact forms, so
// `{{ new.title }}` would be written as literal text.
func placeholders(s string) []string {
	var out []string
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			return out
		}
		j := strings.Index(s[i+2:], "}}")
		if j < 0 {
			return out
		}
		out = append(out, s[i+2:i+2+j])
		s = s[i+2+j+2:]
	}
}

func knownPlaceholder(ph string) bool {
	if strings.ContainsFunc(ph, unicode.IsSpace) {
		return false
	}
	switch ph {
	case "now", "today", "user.name", "user.email", "entity.id", "entity.type":
		return true
	}
	for _, prefix := range []string{"new.", "old."} {
		if rest, ok := strings.CutPrefix(ph, prefix); ok && rest != "" {
			return true
		}
	}
	return false
}

// expandAbstractTypes replaces each abstract type in a rule's `on.entity`
// with its variants, keeping order and dropping duplicates, as the schema
// loader did for automations: an event always names a concrete type.
func expandAbstractTypes(meta *metamodel.Metamodel, rules []Rule) {
	for i := range rules {
		names := rules[i].On.Entity
		if names == nil {
			continue
		}
		out := make([]string, 0, len(names))
		add := func(n string) {
			if !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
		for _, n := range names {
			if variants, ok := metamodel.VariantsOf(meta, n); ok {
				for _, v := range variants {
					add(v)
				}
				continue
			}
			add(n)
		}
		rules[i].On.Entity = out
	}
}

// hasAlias reports whether n or a node below it is an alias or an anchor.
func hasAlias(n *yaml.Node) bool {
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return true
	}
	return slices.ContainsFunc(n.Content, hasAlias)
}

// decodeNode decodes one node strictly, refusing unknown keys. It decodes
// the node in place, so every line an error names is a line of the file.
// yaml.v3 decodes strictly only through a Decoder, which would need the node
// written out again and would count lines from that copy. [unknownKeys]
// finds the unknown keys instead.
func decodeNode(n *yaml.Node, out any) error {
	if errs := unknownKeys(n, reflect.TypeOf(out)); len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return n.Decode(out)
}

// nodeType is the type of a field that holds raw YAML. Its keys are checked
// where it is decoded.
var nodeType = reflect.TypeFor[yaml.Node]()

// unknownKeys returns one message per mapping key under n that t declares no
// field for, in document order. It reads the `yaml:` tags the way the
// decoder does. A node of the wrong kind is left to the decoder, which
// reports it.
func unknownKeys(n *yaml.Node, t reflect.Type) []string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nodeType {
		return nil
	}
	var errs []string
	switch t.Kind() {
	case reflect.Struct:
		if n.Kind != yaml.MappingNode {
			return nil
		}
		fields := yamlFields(t)
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			ft, ok := fields[k.Value]
			if !ok {
				errs = append(errs, fmt.Sprintf("line %d: field %s not found in type %s", k.Line, k.Value, t))
				continue
			}
			errs = append(errs, unknownKeys(v, ft)...)
		}
	case reflect.Slice, reflect.Array:
		if n.Kind != yaml.SequenceNode {
			return nil
		}
		for _, item := range n.Content {
			errs = append(errs, unknownKeys(item, t.Elem())...)
		}
	case reflect.Map:
		if n.Kind != yaml.MappingNode {
			return nil
		}
		for i := 1; i < len(n.Content); i += 2 {
			errs = append(errs, unknownKeys(n.Content[i], t.Elem())...)
		}
	default:
		// A scalar holds no keys.
	}
	return errs
}

// yamlFields maps each key a struct decodes to the type of its field. An
// untagged field decodes from its lowercased name, as in yaml.v3, and an
// inline struct adds its own keys.
func yamlFields(t reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	for f := range t.Fields() {
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("yaml")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if slices.Contains(strings.Split(opts, ","), "inline") && f.Type.Kind() == reflect.Struct {
			maps.Copy(out, yamlFields(f.Type))
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		out[name] = f.Type
	}
	return out
}
