package triggers

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestParse_NoTriggers pins that a document without rules yields no rules
// and no error.
func TestParse_NoTriggers(t *testing.T) {
	t.Parallel()
	meta := mustMeta(t, checkSchema)
	for _, tc := range []struct{ name, doc string }{
		{"absent file", ""},
		{"blank", "  \n"},
		{"only flows", "flows: {}\n"},
		{"null triggers", "triggers:\n"},
		{"empty triggers", "triggers: {}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rules, err := Parse([]byte(tc.doc), meta)
			if err != nil || len(rules) != 0 {
				t.Fatalf("Parse = %v, %v; want no rules and no error", rules, err)
			}
		})
	}
}

func TestParse_RequiresMetamodel(t *testing.T) {
	t.Parallel()
	if _, err := Parse([]byte("triggers: {}\n"), nil); err == nil {
		t.Fatal("Parse accepted a nil metamodel")
	}
}

// TestParse_Refused covers every decode error: each names the problem, and
// the trigger when there is one.
func TestParse_Refused(t *testing.T) {
	t.Parallel()
	const created = `{entity: task, created: true}`
	tests := []struct {
		name  string
		flows string
		want  string
	}{
		{"not yaml", "triggers: [\n", "flows.yaml:"},
		{"triggers is a list", "triggers: [a, b]\n", "`triggers:` must be a mapping of names to rules"},

		// Strict decode: an unknown key is an error, never ignored.
		{"unknown rule key", "triggers:\n  a:\n    on: {entity: task, created: true}\n    do: []\n",
			"field do not found"},
		{"unknown on key", oneTrigger(`{entity: task, created: true, condition: "x"}`), "field condition not found"},
		{"unknown output key", oneTrigger(created, `{set: {properties: {title: x}, value: y}}`),
			"field value not found"},
		{"old script key", oneTrigger(created, `{script: {lua_file: p.lua}}`), "field lua_file not found"},
		{"unknown create key", oneTrigger(created, `{create: {type: task, relation: blocks}}`),
			"field relation not found"},
		{"old output name", oneTrigger(created, `{create_entity: {type: task}}`), "field create_entity not found"},

		// Exactly one key per output.
		{"two keys in one output", oneTrigger(created, `{set: {properties: {title: x}}, script: {lua: "x = 1"}}`),
			"an output holds exactly one of set, relate, create, add_to_pile, notify, script or check"},
		{"empty output", oneTrigger(created, `{}`), "an output holds exactly one of"},

		// set.
		{"set without properties", oneTrigger(created, `{set: {}}`), "set needs `properties:`"},
		{"set properties not a mapping", oneTrigger(created, `{set: {properties: [title]}}`),
			"set.properties: must be a mapping of property names to values"},
		{"set a non-scalar value", oneTrigger(created, `{set: {properties: {title: [a]}}}`),
			`property "title": a value is a scalar or a template`},

		// script.
		{"script with neither lua nor file", oneTrigger(created, `{script: {background: true}}`),
			"script needs exactly one of `lua:` and `file:`"},
		{"script with both lua and file", oneTrigger(created, `{script: {lua: "x()", file: p.lua}}`),
			"script needs exactly one of `lua:` and `file:`"},
		{"bad allow_acl_bypass", oneTrigger(created, `{script: {lua: "x()", allow_acl_bypass: all}}`), "trigger \"a\""},
		{"bad retry", oneTrigger(created, `{script: {file: p.lua, background: true, retry: forever}}`),
			"trigger \"a\""},

		// create.
		{"create without type", oneTrigger(created, `{create: {properties: {title: x}}}`), "create needs `type:`"},
		{"create.relate from the target", oneTrigger(created,
			`{create: {type: task, relate: {type: blocks, from: target}}}`), "create.relate.from must be \"subject\""},
		{"create.relate without from", oneTrigger(created, `{create: {type: task, relate: {type: blocks}}}`),
			"create.relate.from must be \"subject\""},
		{"create.relate without type", oneTrigger(created, `{create: {type: task, relate: {from: subject}}}`),
			"create.relate needs `type:`"},
		{"unknown if_exists", oneTrigger(created, `{create: {type: task, if_exists: sometimes}}`),
			`create.if_exists "sometimes" is not one of skip, error or replace`},

		// check.
		{"check without message", oneTrigger(created, `{check: {where: "title=x"}}`), "check needs `message:`"},

		// Names.
		{"empty name", "triggers:\n  \"\":\n    on: {entity: task, created: true}\n", "a trigger needs a name"},
		{"blank name", "triggers:\n  \" \":\n    on: {entity: task, created: true}\n", "a trigger needs a name"},
		{"name with a dot", "triggers:\n  a.b:\n    on: {entity: task, created: true}\n",
			"a trigger name may not contain `.` or `:`"},
		{"name with a colon", "triggers:\n  \"a:b\":\n    on: {entity: task, created: true}\n",
			"a trigger name may not contain `.` or `:`"},

		// Anchors and aliases.
		{"alias", "triggers:\n  a: &rule\n    on: {entity: task, created: true}\n  b: *rule\n",
			"may not use YAML anchors or aliases"},
		{"anchor only", "triggers:\n  a:\n    on: &on {entity: task, created: true}\n",
			"may not use YAML anchors or aliases"},

		// Placeholders the engine does not replace.
		{"state-output placeholder in set", oneTrigger(created, `{set: {properties: {title: "{{subject.title}}"}}}`),
			"action 1 set: unknown placeholder {{subject.title}}"},
		{"typo in relate.to", oneTrigger(created, `{relate: {type: blocks, to: "{{nwe.id}}"}}`),
			"action 1 relate.to: unknown placeholder {{nwe.id}}"},
		{"bare prefix in create.properties", oneTrigger(created,
			`{create: {type: task, properties: {title: "{{new.}}"}}}`),
			"action 1 create.properties.title: unknown placeholder {{new.}}"},
		{"unknown placeholder in create.template", oneTrigger(created,
			`{create: {type: task, template: "{{kind}}"}}`), "action 1 create.template: unknown placeholder {{kind}}"},
		{"placeholder with inner spaces", oneTrigger(created, `{set: {properties: {title: "{{ new.title }}"}}}`),
			"action 1 set: unknown placeholder {{ new.title }}"},
		{"unknown placeholder in add_to_pile.owner", oneTrigger(created,
			`{add_to_pile: {pile: Inbox, owner: "{{owner}}"}}`), "action 1 add_to_pile.owner: unknown placeholder"},
		{"current_user without a field", oneTrigger(created, `{relate: {type: blocks, to: "{{current_user}}"}}`),
			"action 1 relate.to: unknown placeholder {{current_user}}"},
		{"current_user field other than id", oneTrigger(created,
			`{relate: {type: blocks, to: "{{current_user.name}}"}}`),
			"action 1 relate.to: unknown placeholder {{current_user.name}}"},
		{"unknown placeholder in a check message", oneTrigger(created,
			`{check: {where: "title=x", message: "{{entity.title}}"}}`),
			"check 1 message: unknown placeholder {{entity.title}}"},

		// where: is filter syntax; a clause that does not parse fails the load.
		{"unparseable where clause", oneTrigger(`{entity: task, created: true, where: ["novalueoroperator"]}`),
			`where clause "novalueoroperator"`},
	}
	meta := mustMeta(t, checkSchema)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rules, err := Parse([]byte(tc.flows), meta)
			if err == nil {
				t.Fatalf("Parse accepted the document: %+v", rules)
			}
			if rules != nil {
				t.Errorf("Parse returned rules beside an error: %+v", rules)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestParse_DecodeErrorLines pins that a decode error names the line of the
// file, not a line counted from the rule it is in.
func TestParse_DecodeErrorLines(t *testing.T) {
	t.Parallel()
	// The blank line and the comment would shift the line if the rule were
	// written out again and decoded from the copy.
	const head = "flows: {}\n\n# Triggers.\ntriggers:\n  ok:\n    on: {entity: task, created: true}\n" +
		"    outputs: [{set: {properties: {title: x}}}]\n  a:\n\n    on:\n      entity: task\n"
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"unknown rule key", head + "      created: true\n    do: []\n", "line 13: field do not found"},
		{"unknown on key", head + "      created: true\n      condition: x\n", "line 13: field condition not found"},
		{"unknown output key", head + "      created: true\n    outputs:\n\n      - set: {properties: {title: x}}\n" +
			"        value: y\n", "line 16: field value not found"},
		{"unknown capabilities key", head + "      created: true\n    outputs:\n      - script:\n" +
			"          lua: x()\n          capabilities: {htp: true}\n", "line 16: field htp not found"},
		{"wrong value type", head + "      created: [x]\n", "line 12:"},
	}
	meta := mustMeta(t, checkSchema)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse([]byte(tc.doc), meta)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

// TestParse_DuplicateName pins that two rules cannot share a name: audit
// records and queued jobs find a rule by it.
func TestParse_DuplicateName(t *testing.T) {
	t.Parallel()
	doc := "triggers:\n  a:\n    on: {entity: task, created: true}\n  a:\n    on: {entity: task, updated: true}\n"
	_, err := Parse([]byte(doc), mustMeta(t, checkSchema))
	if err == nil || !strings.Contains(err.Error(), `line 4: trigger "a" is declared twice`) {
		t.Fatalf("err = %v, want the second \"a\" refused as declared twice", err)
	}
}

// TestParse_ReportsEveryRule pins that the problems of every rule are
// reported together, not only the first.
func TestParse_ReportsEveryRule(t *testing.T) {
	t.Parallel()
	doc := `
triggers:
  first:
    on: {entity: task, created: true}
    outputs: [{set: {}}]
  second:
    on: {entity: task, created: true}
    outputs: [{script: {}}]
`
	got := problems(func() error { _, err := Parse([]byte(doc), mustMeta(t, checkSchema)); return err }())
	if len(got) != 2 || !strings.Contains(got[0], `trigger "first"`) || !strings.Contains(got[1], `trigger "second"`) {
		t.Fatalf("problems = %q, want one for each trigger", got)
	}
}

// TestParse_Decodes pins how each key of flows.yaml lands in a [Rule].
func TestParse_Decodes(t *testing.T) {
	t.Parallel()
	meta := mustMeta(t, checkSchema)
	tests := []struct {
		name  string
		flows string
		want  Rule
	}{
		{
			name: "on keys",
			flows: `
triggers:
  a:
    description: Moves the ticket.
    on:
      entity: page
      property: status
      from: open
      becomes: done
      faces: published
      where: ["status!=", "title=x"]
      when: "entity.status == 'done'"
    outputs: [{set: {properties: {title: x}}}]
`,
			want: Rule{Name: "a", Description: "Moves the ticket.", On: Trigger{
				Entity: []string{"page"}, Property: "status", From: "open", Becomes: "done",
				Faces: []string{"published"}, When: mustFilters(t, "status!=", "title=x"),
				Condition: "entity.status == 'done'",
			}, Do: defaultDo},
		},
		{
			name:  "relation keys",
			flows: oneTrigger(`{entity: [task], relation_created: blocks, relation_removed: blocks}`),
			want: Rule{Name: "a", On: Trigger{
				Entity: []string{"task"}, RelationCreated: "blocks", RelationRemoved: "blocks",
			}, Do: defaultDo},
		},
		{
			name:  "copy",
			flows: oneTrigger(`{entity: ticket, copy: [spawn], updated: true}`),
			want: Rule{
				Name: "a", On: Trigger{Entity: []string{"ticket"}, Copy: []string{"spawn"}, Updated: true}, Do: defaultDo,
			},
		},
		{
			name:  "deleted",
			flows: oneTrigger(`{entity: task, deleted: true}`, `{script: {file: gone.lua, background: true}}`),
			want: Rule{Name: "a", On: Trigger{Entity: []string{"task"}, Deleted: true},
				Do: []Action{{LuaFile: "gone.lua", Background: true}}},
		},
		{
			// One set output with several properties becomes one action per
			// property, in document order.
			name: "multi-property set",
			flows: oneTrigger(`{entity: task, created: true}`,
				`{set: {properties: {zeta: "{{new.title}}", alpha: "1", mid: ~}}}`,
				`{set: {properties: {title: x}}}`),
			want: Rule{Name: "a", On: Trigger{Entity: []string{"task"}, Created: true}, Do: []Action{
				{Set: "zeta", Value: "{{new.title}}"}, {Set: "alpha", Value: "1"}, {Set: "mid", Value: ""},
				{Set: "title", Value: "x"},
			}},
		},
		{
			name:  "relate",
			flows: oneTrigger(`{entity: task, created: true}`, `{relate: {type: blocks, to: "{{new.title}}"}}`),
			want: Rule{Name: "a", On: Trigger{Entity: []string{"task"}, Created: true}, Do: []Action{
				{CreateRelation: &CreateRelationAction{Relation: "blocks", To: "{{new.title}}"}},
			}},
		},
		{
			name: "create",
			flows: oneTrigger(`{entity: task, created: true}`,
				`{create: {type: task, template: "{{new.title}}", properties: {title: "Copy of {{new.title}}"},`+
					` relate: {type: blocks, from: subject}, if_exists: replace}}`),
			want: Rule{Name: "a", On: Trigger{Entity: []string{"task"}, Created: true}, Do: []Action{
				{CreateEntity: &CreateEntityAction{
					Type: "task", Template: "{{new.title}}", Properties: map[string]string{"title": "Copy of {{new.title}}"},
					Relation: "blocks", IfExists: IfExistsReplace,
				}},
			}},
		},
		{
			name: "create with defaults",
			flows: oneTrigger(`{entity: task, created: true}`, `{create: {type: task}}`,
				`{create: {type: bug, if_exists: skip}}`, `{create: {type: chore, if_exists: error}}`),
			want: Rule{Name: "a", On: Trigger{Entity: []string{"task"}, Created: true}, Do: []Action{
				{CreateEntity: &CreateEntityAction{Type: "task"}},
				{CreateEntity: &CreateEntityAction{Type: "bug", IfExists: IfExistsSkip}},
				{CreateEntity: &CreateEntityAction{Type: "chore", IfExists: IfExistsError}},
			}},
		},
		{
			// create defaults to true, as it did for automations.
			name: "add_to_pile",
			flows: oneTrigger(`{entity: ticket, created: true}`,
				`{add_to_pile: {pile: Inbox, owner: "{{new.title}}"}}`, `{add_to_pile: {pile: Later, create: false}}`),
			want: Rule{Name: "a", On: Trigger{Entity: []string{"ticket"}, Created: true}, Do: []Action{
				{AddToPile: &AddToPileAction{Pile: "Inbox", Owner: "{{new.title}}", Create: true}},
				{AddToPile: &AddToPileAction{Pile: "Later", Create: false}},
			}},
		},
		{
			name: "notify",
			flows: oneTrigger(`{entity: ticket, created: true}`,
				`{notify: {pile: inbox, reason: done, to: {via: owned-by, property: name}}}`,
				`{notify: {pile: inbox, to: {via: [owned-by], type: person, property: name}}}`),
			want: Rule{Name: "a", On: Trigger{Entity: []string{"ticket"}, Created: true}, Do: []Action{
				{Notify: &NotifyAction{Pile: "inbox", Reason: "done", Via: []string{"owned-by"}, Property: "name"}},
				{Notify: &NotifyAction{Pile: "inbox", Via: []string{"owned-by"}, Type: "person", Property: "name"}},
			}},
		},
		{
			name: "script",
			flows: oneTrigger(`{entity: task, created: true}`,
				`{script: {lua: "rela.log(1)", allow_acl_bypass: read+write, capabilities: {http: true}}}`,
				`{script: {file: sync.lua, background: true, run_as: bot, retry: persistent}}`),
			want: Rule{Name: "a", On: Trigger{Entity: []string{"task"}, Created: true}, Do: []Action{
				{Lua: "rela.log(1)", AllowACLBypass: metamodel.ACLBypassReadWrite,
					Capabilities: metamodel.Capabilities{HTTP: true}},
				{LuaFile: "sync.lua", Background: true, RunAs: "bot", Retry: metamodel.JobRetryPersistent},
			}},
		},
		{
			name: "check",
			flows: oneTrigger(`{entity: task, updated: true}`, `{set: {properties: {title: x}}}`,
				`{check: {where: "title!=", severity: error, message: "{{new.title}} needs a title"}}`,
				`{check: {where: "title=x", message: plain}}`),
			want: Rule{Name: "a", On: Trigger{Entity: []string{"task"}, Updated: true},
				Do: []Action{{Set: "title", Value: "x"}},
				Validate: []Validation{
					{Check: "title!=", Severity: "error", Message: "{{new.title}} needs a title"},
					{Check: "title=x", Message: "plain"},
				}},
		},
		{
			// An abstract type expands to its variants, in place and
			// without duplicates: an event always names a concrete type.
			name:  "abstract on.entity",
			flows: oneTrigger(`{entity: [task, work, bug], created: true}`),
			want:  Rule{Name: "a", On: Trigger{Entity: []string{"task", "bug", "chore"}, Created: true}, Do: defaultDo},
		},
		{
			name: "known placeholders",
			flows: oneTrigger(`{entity: task, created: true}`,
				`{set: {properties: {title: "{{new.title}} {{old.title}} {{entity.id}} {{entity.type}} `+
					`{{user.name}} {{user.email}} {{current_user.id}} {{today}} {{now}} {{unclosed"}}}`),
			want: Rule{Name: "a", On: Trigger{Entity: []string{"task"}, Created: true}, Do: []Action{
				{Set: "title", Value: "{{new.title}} {{old.title}} {{entity.id}} {{entity.type}} " +
					"{{user.name}} {{user.email}} {{current_user.id}} {{today}} {{now}} {{unclosed"},
			}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rules, err := Parse([]byte(tc.flows), meta)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(rules) != 1 {
				t.Fatalf("rules = %+v, want one", rules)
			}
			if got := emptyAsNil(rules[0]); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rule =\n  %#v\nwant\n  %#v", got, tc.want)
			}
		})
	}
}

// emptyAsNil returns r with every empty slice set to nil, so a decoded rule
// compares equal to a literal that leaves the slice out.
func emptyAsNil(r Rule) Rule {
	if len(r.On.Entity) == 0 {
		r.On.Entity = nil
	}
	if len(r.On.Faces) == 0 {
		r.On.Faces = nil
	}
	if len(r.On.Copy) == 0 {
		r.On.Copy = nil
	}
	if len(r.On.When) == 0 {
		r.On.When = nil
	}
	if len(r.Do) == 0 {
		r.Do = nil
	}
	if len(r.Validate) == 0 {
		r.Validate = nil
	}
	return r
}

// defaultDo is the action [defaultOutput] decodes to.
var defaultDo = []Action{{Set: "title", Value: "x"}}

// TestParse_DocumentOrder pins that rules keep their document order, which
// is the order they fire in.
func TestParse_DocumentOrder(t *testing.T) {
	t.Parallel()
	doc := "triggers:\n" +
		"  zulu:\n    on: {entity: task, created: true}\n    outputs: [{set: {properties: {title: x}}}]\n" +
		"  alpha:\n    on: {entity: task, created: true}\n    outputs: [{set: {properties: {title: x}}}]\n" +
		"  mike:\n    on: {entity: task, created: true}\n    outputs: [{set: {properties: {title: x}}}]\n"
	rules := mustParse(t, mustMeta(t, checkSchema), doc)
	var names []string
	for _, r := range rules {
		names = append(names, r.Name)
	}
	if want := []string{"zulu", "alpha", "mike"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

// TestParse_ACLBypass pins that a script without `allow_acl_bypass` is not
// elevated, and that the key reaches the action when present.
func TestParse_ACLBypass(t *testing.T) {
	t.Parallel()
	rules := mustParse(t, mustMeta(t, checkSchema), oneTrigger(`{entity: task, created: true}`,
		`{script: {lua: "print('x')"}}`, `{script: {lua: "print('y')", allow_acl_bypass: read}}`))
	if rules[0].Do[0].AllowACLBypass.Enabled() {
		t.Errorf("a script with no allow_acl_bypass key is elevated (%q)", rules[0].Do[0].AllowACLBypass)
	}
	if b := rules[0].Do[1].AllowACLBypass; !b.AllowsRead() || b.AllowsWrite() {
		t.Errorf("allow_acl_bypass: read decoded as %q", b)
	}
}

// TestParse_WhereClausesAreFilters pins that `where:` holds filter syntax:
// each clause is parsed by internal/filter into the rule's When.
func TestParse_WhereClausesAreFilters(t *testing.T) {
	t.Parallel()
	rules := mustParse(t, mustMeta(t, checkSchema), oneTrigger(`{entity: task, created: true, where: ["title!=done"]}`))
	when := rules[0].On.When
	if len(when) != 1 || when[0].Property != "title" || when[0].Operator != filter.OpNotEqual || when[0].Value != "done" {
		t.Errorf("When = %+v, want one title != done filter", when)
	}
}

func TestValidation_GetSeverity(t *testing.T) {
	t.Parallel()
	if got := (&Validation{}).GetSeverity(); got != "warning" {
		t.Errorf("default severity = %q, want warning", got)
	}
	if got := (&Validation{Severity: "error"}).GetSeverity(); got != "error" {
		t.Errorf("severity = %q, want error", got)
	}
}
