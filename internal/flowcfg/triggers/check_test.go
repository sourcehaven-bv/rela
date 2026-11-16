package triggers

import (
	"strings"
	"testing"
)

// checkSchema declares what every [Check] case refers to: a faced type and
// copies between its faces, an external ref, piles with and without read
// state, an abstract type with two variants, and plain relations.
const checkSchema = `
version: "1"
piles:
  favorites: {name: Favorites}
  inbox: {name: Inbox, read_state: true, reasons: {done: completed a task}}
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties:
      title: {type: string}
      basecamp: {type: external_ref, system: basecamp}
  person:
    label: Person
    id_prefix: "P-"
    properties: {name: {type: string}}
  task:
    label: Task
    id_prefix: "TASK-"
    properties: {title: {type: string}}
  page:
    label: Page
    id_prefix: "PAGE-"
    faces: {draft: {}, published: {}}
    properties: {title: {type: string}, status: {type: string}}
  work:
    abstract: true
    label: Work
    properties: {title: {type: string}}
  bug:
    extends: work
    label: Bug
    id_prefix: "BUG-"
  chore:
    extends: work
    label: Chore
    id_prefix: "CHR-"
relations:
  owned-by: {from: [ticket], to: [person]}
  blocks: {from: [task], to: [task]}
  implements: {from: [page], to: [ticket]}
copies:
  promote:
    from: page@draft
    to: page@published
    fields: all
    guard: {permission: promote-page}
  spawn:
    from: ticket
    to: new ticket
    fields: {title: "{{new.title}}"}
`

// oneTrigger is a flows.yaml document with one trigger named "a". Each
// output is one flow-style list entry. Without outputs the trigger gets
// [defaultOutput], because a rule with no outputs does not load. An
// `on.deleted` trigger is left without, for the tests of what it needs.
// defaultOutput is the output [oneTrigger] adds when a test names none.
const defaultOutput = `{set: {properties: {title: x}}}`

func oneTrigger(on string, outputs ...string) string {
	if len(outputs) == 0 && !strings.Contains(on, "deleted: true") {
		outputs = []string{defaultOutput}
	}
	var b strings.Builder
	b.WriteString("triggers:\n  a:\n    on: " + on + "\n")
	if len(outputs) > 0 {
		b.WriteString("    outputs:\n")
		for _, o := range outputs {
			b.WriteString("      - " + o + "\n")
		}
	}
	return b.String()
}

// problems splits a [Parse] error into its problem lines.
func problems(err error) []string {
	if err == nil {
		return nil
	}
	lines := strings.Split(err.Error(), "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines[1:] {
		out = append(out, strings.TrimSpace(l))
	}
	return out
}

// TestCheck ports the automation checks the schema loader ran while the
// rules lived in schema.yaml (TKT-GWOBH2U). Each case goes through [Parse],
// which runs [Check]; want is a substring of the single expected problem,
// or "" when the rule must load.
func TestCheck(t *testing.T) {
	t.Parallel()
	const bg = `{script: {file: push.lua, background: true}}`
	const created = `{entity: task, created: true}`
	const onTicket = `{entity: ticket, created: true}`
	tests := []struct {
		name  string
		flows string
		want  string
	}{
		// on.copy (TKT-50DZEHI): each error names a scope that could never match.
		// A rule that can never fire or never act does not load.
		{"no event key", oneTrigger(`{entity: task, where: ["title!="]}`), "`on` needs an event key"},
		{"becomes without property", oneTrigger(`{entity: task, becomes: done}`), "`on` needs an event key"},
		{"no outputs", "triggers:\n  a:\n    on: {entity: task, created: true}\n", "`outputs` is empty"},
		{"only a check", oneTrigger(created, `{check: {where: "title!=", message: m}}`), ""},
		{"check severity error", oneTrigger(created, `{check: {where: "title!=", severity: error, message: m}}`), ""},
		{"check severity warning", oneTrigger(created,
			`{check: {where: "title!=", severity: warning, message: m}}`), ""},
		{"unknown check severity", oneTrigger(created, `{check: {where: "title!=", severity: fatal, message: m}}`),
			`check 1: severity "fatal" is not warning or error`},

		{"copy alone", oneTrigger(`{copy: promote}`), ""},
		{"copy with matching entity and face",
			oneTrigger(`{entity: page, faces: [published], copy: promote, created: true}`), ""},
		{"cross-entity copy into the default face", oneTrigger(`{entity: ticket, copy: spawn}`), ""},
		{"undeclared copy", oneTrigger(`{copy: nope}`), "`on.copy: [nope]` names no declared copy"},
		{"entity the copy does not write", oneTrigger(`{entity: ticket, copy: promote}`),
			"writes \"page\", which `on.entity` does not name"},
		{"face the copy does not write", oneTrigger(`{faces: [draft], copy: promote}`),
			"writes face \"published\", which `on.faces` does not name"},
		{"copy with a relation event", oneTrigger(`{copy: promote, relation_created: implements}`),
			"a copy raises only entity events"},

		// on.faces (TKT-4Y6CMV).
		{"declared face", oneTrigger(`{entity: page, created: true, faces: [draft]}`), ""},
		{"misspelled face", oneTrigger(`{entity: page, created: true, faces: [drafts]}`),
			"names no declared face of page"},
		{"face on an unfaced type", oneTrigger(`{entity: task, created: true, faces: [draft]}`),
			"task declares no `faces:`"},
		{"face no type declares", oneTrigger(`{created: true, faces: [nope]}`),
			"names a face no entity type declares"},

		// Background scripts (TKT-2Q4UFI).
		{"background script", oneTrigger(created, bg), ""},
		{"run_as and retry", oneTrigger(created,
			`{script: {file: p.lua, background: true, run_as: bot, retry: never}}`), ""},
		{"background inline lua", oneTrigger(created, `{script: {lua: "x()", background: true}}`),
			"`background: true` needs `file:`"},
		{"background with bypass", oneTrigger(created,
			`{script: {file: p.lua, background: true, allow_acl_bypass: read}}`),
			"`allow_acl_bypass` is not supported on a background script"},
		{"run_as without background", oneTrigger(created, `{script: {file: p.lua, run_as: bot}}`),
			"only with `background: true`"},
		{"retry without background", oneTrigger(created, `{script: {file: p.lua, retry: never}}`),
			"only with `background: true`"},
		{"run_as surrounded by space", oneTrigger(created,
			`{script: {file: p.lua, background: true, run_as: " bot"}}`), "invalid run_as"},
		{"run_as with a control character", oneTrigger(created,
			`{script: {file: p.lua, background: true, run_as: "b\x01ot"}}`), "invalid run_as"},
		{"reserved run_as", oneTrigger(created,
			`{script: {file: p.lua, background: true, run_as: "system:scheduler"}}`), "invalid run_as"},
		{"automation run_as", oneTrigger(created,
			`{script: {file: p.lua, background: true, run_as: "system:automation"}}`), ""},
		{"integration run_as", oneTrigger(created,
			`{script: {file: p.lua, background: true, run_as: "integration:basecamp"}}`), ""},
		{"bad integration run_as", oneTrigger(created,
			`{script: {file: p.lua, background: true, run_as: "integration:Base Camp"}}`), "invalid run_as"},
		{"tokens on a synchronous script", oneTrigger(created,
			`{script: {file: p.lua, capabilities: {tokens: [basecamp]}}}`),
			"`capabilities.tokens` applies only with `background: true`"},
		{"tokens on a background script", oneTrigger(created,
			`{script: {file: p.lua, background: true, capabilities: {tokens: [basecamp]}}}`), ""},
		{"same file twice", oneTrigger(created, bg, bg), "runs in the background twice"},
		{"same file, other spelling", oneTrigger(created, bg,
			`{script: {file: ./push.lua, background: true}}`), "runs in the background twice"},

		// on.updated.
		{"updated with property", oneTrigger(`{entity: task, updated: true, property: title}`),
			"`on.updated` fires on any change"},
		{"updated with becomes", oneTrigger(`{entity: task, updated: true, becomes: x}`),
			"`on.updated` fires on any change"},
		{"updated with a foreground script", oneTrigger(`{entity: task, updated: true}`,
			`{script: {lua: "x = 1"}}`), "must be `background: true`"},
		{"updated with a background script", oneTrigger(`{entity: task, updated: true}`, bg), ""},

		// on.deleted.
		{"deleted with a background script", oneTrigger(`{entity: task, deleted: true}`, bg), ""},
		{"deleted with a foreground script", oneTrigger(`{entity: task, deleted: true}`,
			`{script: {file: p.lua}}`), "an `on.deleted` output must be a `background: true` script"},
		{"deleted with set", oneTrigger(`{entity: task, deleted: true}`,
			`{set: {properties: {title: x}}}`), "an `on.deleted` output must be a `background: true` script"},
		{"deleted without outputs", oneTrigger(`{entity: task, deleted: true}`),
			"`on.deleted` needs a `background: true` script"},
		{"deleted with created", oneTrigger(`{entity: task, deleted: true, created: true}`, bg),
			"cannot be combined"},
		{"deleted with property", oneTrigger(`{entity: task, deleted: true, property: title}`, bg),
			"cannot be combined"},
		{"deleted with relation_removed", oneTrigger(`{entity: task, deleted: true, relation_removed: blocks}`, bg),
			"cannot be combined"},
		{"deleted with a check", oneTrigger(`{entity: task, deleted: true}`, bg,
			`{check: {where: "title=x", message: m}}`), "a `check` output does not apply to `on.deleted`"},

		// add_to_pile.
		{"add_to_pile", oneTrigger(onTicket, `{add_to_pile: {pile: Inbox, owner: "{{new.title}}"}}`), ""},
		{"add_to_pile without pile", oneTrigger(onTicket, `{add_to_pile: {owner: "{{new.title}}"}}`),
			"add_to_pile requires `pile:`"},
		{"add_to_pile with a blank pile", oneTrigger(onTicket, `{add_to_pile: {pile: "  "}}`),
			"add_to_pile requires `pile:`"},

		// notify.
		{"notify", oneTrigger(onTicket, `{notify: {pile: inbox, reason: done, to: {via: owned-by, property: name}}}`), ""},
		{"notify without pile", oneTrigger(onTicket, `{notify: {to: {property: title}}}`), "notify requires `pile:`"},
		{"notify to an undeclared pile", oneTrigger(onTicket, `{notify: {pile: nope, to: {property: title}}}`),
			`pile "nope" is not declared`},
		{"notify to a pile without read state", oneTrigger(onTicket,
			`{notify: {pile: favorites, to: {property: title}}}`), "needs `read_state: true`"},
		{"notify with an unknown reason", oneTrigger(onTicket,
			`{notify: {pile: inbox, reason: other, to: {property: title}}}`), `reason "other" is not one of`},
		{"notify without property", oneTrigger(onTicket, `{notify: {pile: inbox, to: {}}}`),
			"requires `to.property:`"},
		{"notify via without on.entity", oneTrigger(`{created: true}`,
			`{notify: {pile: inbox, to: {via: x, property: title}}}`), "needs the trigger's `on.entity:`"},
		{"notify type without via", oneTrigger(onTicket,
			`{notify: {pile: inbox, to: {type: person, property: title}}}`), "applies only with `to.via:`"},

		// Names rela manages (TKT-BC5G8RP).
		{"managed on.entity", oneTrigger(`{entity: _flow-run, created: true}`), "on.entity: \"_flow-run\" is managed by rela"},
		{"managed on.relation_created", oneTrigger(`{relation_created: _carries}`),
			"on.relation_created: \"_carries\" is managed by rela"},
		{"managed on.relation_removed", oneTrigger(`{relation_removed: _produced}`),
			"on.relation_removed: \"_produced\" is managed by rela"},
		{"managed relate.type", oneTrigger(created, `{relate: {type: _carries, to: "{{new.title}}"}}`),
			"relate.type: \"_carries\" is managed by rela"},
		{"managed create.type", oneTrigger(created, `{create: {type: _flow-run}}`),
			"create.type: \"_flow-run\" is managed by rela"},
		{"managed create.relate.type", oneTrigger(created,
			`{create: {type: task, relate: {type: _carries, from: subject}}}`),
			"create.relate.type: \"_carries\" is managed by rela"},
		{"plain names", oneTrigger(`{entity: task, relation_created: blocks}`,
			`{create: {type: task, relate: {type: blocks, from: subject}}}`), ""},

		// External refs change only through sync.
		{"external ref property trigger", oneTrigger(`{entity: ticket, property: basecamp}`,
			`{set: {properties: {title: x}}}`), "`on.property` \"basecamp\" is an external ref on \"ticket\""},
		{"set an external ref", oneTrigger(onTicket, `{set: {properties: {basecamp: x}}}`),
			"`set` \"basecamp\" is an external ref"},
		{"set an external ref on every type", oneTrigger(`{created: true}`, `{set: {properties: {basecamp: x}}}`),
			"only a script may write a ref"},
		{"create an external ref", oneTrigger(onTicket,
			`{create: {type: ticket, properties: {title: x, basecamp: "{{new.title}}"}}}`),
			"`create` property \"basecamp\" is an external ref"},

		// A create writes one concrete type.
		{"create an abstract type", oneTrigger(created, `{create: {type: work}}`),
			"create.type: \"work\" is abstract and this setting takes one concrete type; name one of bug, chore"},
		{"create a variant", oneTrigger(created, `{create: {type: bug}}`), ""},
	}
	meta := mustMeta(t, checkSchema)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse([]byte(tc.flows), meta)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("want the rule to load, got: %v", err)
				}
				return
			}
			got := problems(err)
			if len(got) != 1 || !strings.Contains(got[0], tc.want) {
				t.Fatalf("problems = %q, want one containing %q", got, tc.want)
			}
			if !strings.Contains(got[0], `trigger "a"`) {
				t.Errorf("problem %q does not name the trigger", got[0])
			}
		})
	}
}

// TestCheck_Direct pins that [Check] reports every problem of every rule
// when called on rules that did not come from [Parse].
func TestCheck_Direct(t *testing.T) {
	t.Parallel()
	meta := mustMeta(t, checkSchema)
	rules := []Rule{
		{Name: "ok", On: Trigger{Entity: []string{"task"}, Created: true}, Do: []Action{{Set: "title", Value: "x"}}},
		{Name: "bad-copy", On: Trigger{Copy: []string{"nope"}}, Do: []Action{{Set: "title", Value: "x"}}},
		{Name: "bad-pile", On: Trigger{Entity: []string{"task"}, Created: true},
			Do: []Action{{AddToPile: &AddToPileAction{}}, {LuaFile: "p.lua", RunAs: "bot"}}},
	}
	got := Check(meta, rules)
	want := []string{
		`trigger "bad-copy": ` + "`on.copy: [nope]` names no declared copy",
		`trigger "bad-pile" action 2: ` + "`run_as` and `retry` apply only with `background: true`",
		`trigger "bad-pile" action 1: add_to_pile requires ` + "`pile:` (the pile name)",
	}
	if len(got) != len(want) {
		t.Fatalf("Check = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Check[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if errs := Check(meta, nil); len(errs) != 0 {
		t.Errorf("Check(nil rules) = %q, want none", errs)
	}
}
