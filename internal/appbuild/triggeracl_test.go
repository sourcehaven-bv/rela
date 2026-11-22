package appbuild

import (
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/configedit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

const triggerACLSchema = `version: "1.0"
entities:
  person:
    label: Person
    id_prefix: PER
    properties:
      login: {type: string, unique: true}
  group:
    label: Group
    id_prefix: GRP
    properties:
      name: {type: string}
  task:
    label: Task
    id_prefix: TSK
    properties:
      title: {type: string}
      notes: {type: string}
      state: {type: string}
      owner: {type: string}
  note:
    label: Note
    id_prefix: NTE
    properties:
      body: {type: string}
  feed:
    label: Feed
    id_prefix: FED
    properties:
      text: {type: string}
  doc:
    label: Doc
    id_prefix: DOC
    faces:
      draft: {label: Draft}
      published: {label: Published}
    properties:
      x: {type: string}
      y: {type: string}
relations:
  member-of:
    from: [person]
    to: [group]
  about:
    from: [task]
    to: [note]
`

// triggerACLBase is the policy every case starts from: the editor may read
// everything, create and update tasks and notes.
const triggerACLBase = `user_entity_type: person
principal_property: login
roles:
  editor:
    read: ["*"]
    create: [task, note, feed]
    update: [task, note, doc@draft, doc@published]
    permissions: [config:edit]
`

// triggerACLFlow is a flow over task that freezes notes against outside
// edits.
const triggerACLFlow = `flows:
  review:
    label: Review
    subject: task
    initial: open
    outside_edits:
      properties:
        notes: deny
    states:
      open:
        on:
          done: {to: closed}
      closed:
        final: true
`

const triggerACLFlows = `triggers:
  stamp:
    on: {entity: task, created: true}
    outputs:
      - set: {properties: {state: open}}
`

// triggerACLStepFlow is a flow over task whose open state waits for an
// editor's step through the task_check form of data-entry.yaml.
const triggerACLStepFlow = `flows:
  check:
    label: Check
    subject: task
    initial: open
    states:
      open:
        waiting_for: [editor]
        form: task_check
        on:
          change: {to: closed, roles: [editor]}
      closed:
        final: true
`

// stepForm returns data-entry.yaml with the task_check form editing fields.
func stepForm(fields ...string) string {
	if len(fields) == 0 {
		return "forms:\n  task_check:\n    entity_type: task\n    fields: []\n"
	}
	var b strings.Builder
	b.WriteString("forms:\n  task_check:\n    entity_type: task\n    fields:\n")
	for _, f := range fields {
		b.WriteString("      - property: " + f + "\n")
	}
	return b.String()
}

// fakeEditor holds the editor role; deny names the types the live request
// refuses it.
type fakeEditor struct{ deny []string }

func (fakeEditor) Roles() []string { return []string{"editor"} }

func (e fakeEditor) Grants(_ string, _ acl.Verb, entityType string, _ entity.Face) bool {
	return !slices.Contains(e.deny, entityType)
}

// trigger returns flows.yaml with the stamp trigger and one more, named
// added, written as the YAML under it.
func trigger(body string) string {
	return triggerACLFlows + "  added:\n" + body
}

func TestCheckTriggerACL(t *testing.T) {
	tests := []struct {
		name          string
		acl           string // appended to triggerACLBase
		schemaAfter   string // replaces the schema after the edit when set
		before, after string
		// dataEntryBefore and dataEntry are data-entry.yaml before and after;
		// dataEntryBefore defaults to dataEntry.
		dataEntryBefore, dataEntry string
		editor                     fakeEditor
		want                       []string // message parts, one per expected problem; none means the edit passes
	}{
		{name: "a plain set passes",
			after: strings.Replace(triggerACLFlows, "state: open", "state: done", 1)},
		{name: "a description edit of a scripted trigger passes",
			before: trigger("    on: {entity: task, updated: true}\n    outputs:\n      - script: {file: scripts/x.lua, background: true}\n"),
			after: trigger("    description: Note it\n    on: {entity: task, updated: true}\n" +
				"    outputs:\n      - script: {file: scripts/x.lua, background: true}\n")},
		{name: "an added script",
			after: trigger("    on: {entity: task, updated: true}\n    outputs:\n      - script: {file: scripts/x.lua, background: true}\n"),
			want:  []string{"runs a script"}},
		{name: "replace on create",
			after: trigger("    on: {entity: task, created: true}\n    outputs:\n" +
				"      - create: {type: note, relate: {type: about, from: subject}, if_exists: replace}\n"),
			want: []string{"if_exists: replace"}},
		{name: "a placeholder reads a hidden field",
			acl:   "  viewer:\n    read: [task]\n    visible:\n      task: [{field: title}]\n",
			after: trigger("    on: {entity: task, updated: true}\n    outputs:\n      - set: {properties: {title: \"{{new.notes}}\"}}\n"),
			want:  []string{"reads task.notes"}},
		{name: "a client baseline hides the field",
			acl: "client_baselines:\n  apps:\n    applies_to: [app]\n    redact:\n      task: [notes]\n",
			after: trigger("    on: {entity: task, updated: true}\n" +
				"    outputs:\n      - set: {properties: {title: \"{{old.notes}}\"}}\n"),
			want: []string{"reads task.notes"}},
		{name: "a when condition reads a hidden field",
			acl: "client_baselines:\n  apps:\n    applies_to: [app]\n    redact:\n      task: [notes]\n",
			after: trigger("    on: {entity: task, updated: true, when: \"entity.notes == 'x'\"}\n" +
				"    outputs:\n      - set: {properties: {state: flagged}}\n"),
			want: []string{"reads task.notes"}},
		{name: "a when condition follows relations",
			after: trigger("    on: {entity: task, updated: true, when: \"related(entity, 'about')\"}\n" +
				"    outputs:\n      - set: {properties: {state: noted}}\n"),
			want: []string{"follows relations"}},
		{name: "a relation target from a placeholder",
			after: trigger("    on: {entity: task, updated: true}\n" +
				"    outputs:\n      - relate: {type: about, to: \"{{new.notes}}\"}\n"),
			want: []string{"relation target with a placeholder"}},
		{name: "a relation target that is the acting user passes",
			after: trigger("    on: {entity: task, created: true}\n" +
				"    outputs:\n      - relate: {type: about, to: \"{{current_user.id}}\"}\n")},
		{name: "the acting user on the membership relation",
			after: trigger("    on: {entity: person, created: true}\n" +
				"    outputs:\n      - relate: {type: member-of, to: \"{{current_user.id}}\"}\n"),
			want: []string{"creates member-of relations, which acl.yaml decides access by",
				"links person records, which you may not change yourself"}},
		{name: "the acting user on a record the editor may not change",
			after: trigger("    on: {entity: task, created: true}\n" +
				"    outputs:\n      - relate: {type: about, to: \"{{current_user.id}}\"}\n"),
			editor: fakeEditor{deny: []string{"task"}},
			want:   []string{"links task records, which you may not change yourself"}},
		{name: "the acting user inside a longer target",
			after: trigger("    on: {entity: task, created: true}\n" +
				"    outputs:\n      - relate: {type: about, to: \"x-{{current_user.id}}\"}\n"),
			want: []string{"relation target with a placeholder"}},
		{name: "a pile name from a placeholder",
			after: trigger("    on: {entity: task, updated: true}\n" +
				"    outputs:\n      - add_to_pile: {pile: \"{{new.title}}\", owner: \"{{new.owner}}\"}\n"),
			want: []string{"names its pile with a placeholder"}},
		{name: "a property a grant condition reads",
			acl: "  viewer:\n    read: [task]\n    update: [task]\n    fields:\n" +
				"      task: [{field: notes, when: \"entity.owner == 'me'\"}]\n",
			after: trigger("    on: {entity: task, created: true}\n    outputs:\n      - set: {properties: {owner: me}}\n"),
			want:  []string{"sets task.owner, which acl.yaml decides access by"}},
		{name: "an options field",
			acl:   "  viewer:\n    read: [task]\n    options:\n      task: [{field: state, option: done}]\n",
			after: trigger("    on: {entity: task, updated: true}\n    outputs:\n      - set: {properties: {state: done}}\n"),
			want:  []string{"sets task.state, which acl.yaml decides access by"}},
		{name: "the membership relation",
			after: trigger("    on: {entity: person, created: true}\n" +
				"    outputs:\n      - relate: {type: member-of, to: admins}\n"),
			want: []string{"creates member-of relations", "links person records, which you may not change"}},
		{name: "a principal record",
			acl: "  maker:\n    create: [person]\n",
			after: trigger("    on: {entity: task, created: true}\n" +
				"    outputs:\n      - create: {type: person, properties: {login: x}}\n"),
			want: []string{"uses as principals", "decides access by", "may not do yourself"}},
		{name: "a field the editor's role does not grant",
			acl:   "    fields:\n      task: [{field: title}]\n",
			after: trigger("    on: {entity: task, updated: true}\n    outputs:\n      - set: {properties: {state: x}}\n"),
			want:  []string{"sets task.state, which you may not change yourself"}},
		{name: "the live request refuses the editor",
			after:  strings.Replace(triggerACLFlows, "state: open", "state: done", 1),
			editor: fakeEditor{deny: []string{"task"}},
			want:   []string{"sets task.state, which you may not change yourself"}},
		{name: "a created record more widely read than its source",
			acl: "  noter:\n    read: [note]\n",
			after: trigger("    on: {entity: task, created: true}\n" +
				"    outputs:\n      - create: {type: note, relate: {type: about, from: subject}}\n"),
			want: []string{"some readers of note may not read task"}},
		{name: "a client ceiling that lets through the created record only",
			acl: "client_baselines:\n  apps:\n    applies_to: [app]\n    read: [note]\n",
			after: trigger("    on: {entity: task, created: true}\n" +
				"    outputs:\n      - create: {type: note, relate: {type: about, from: subject}}\n"),
			want: []string{"some readers of note may not read task"}},
		{name: "a created record every reader of the source may read",
			after: trigger("    on: {entity: task, created: true}\n" +
				"    outputs:\n      - create: {type: note, relate: {type: about, from: subject}}\n")},
		{name: "removing a stamp the policy reads",
			acl:   "  viewer:\n    read: [task]\n    visible:\n      task: [{field: title, when: \"entity.state == 'open'\"}]\n",
			after: "triggers: {}\n",
			want:  []string{"sets task.state, which acl.yaml decides access by. Changing or removing it"}},
		{name: "a created record fires an unchanged scripted trigger",
			before: trigger("    on: {entity: note, created: true}\n    outputs:\n      - script: {file: scripts/x.lua, background: true}\n"),
			after: trigger("    on: {entity: note, created: true}\n    outputs:\n      - script: {file: scripts/x.lua, background: true}\n") +
				"  spawn:\n    on: {entity: task, created: true}\n" +
				"    outputs:\n      - create: {type: note, relate: {type: about, from: subject}}\n",
			want: []string{"fire automation \"added\", which runs a script"}},
		{name: "a fixed relation target",
			after: trigger("    on: {entity: task, updated: true}\n" +
				"    outputs:\n      - relate: {type: about, to: NTE-1}\n"),
			want: []string{"links to the fixed record \"NTE-1\""}},
		{name: "a template variant",
			after: trigger("    on: {entity: task, created: true}\n" +
				"    outputs:\n      - create: {type: note, template: memo}\n"),
			want: []string{"applies a template variant"}},
		{name: "a property a flow locks",
			before: triggerACLFlow + triggerACLFlows,
			after: triggerACLFlow + trigger("    on: {entity: task, updated: true}\n"+
				"    outputs:\n      - set: {properties: {notes: x}}\n"),
			want: []string{"sets task.notes, which flow \"review\" locks against outside edits"}},
		{name: "a property a step form edits",
			before: triggerACLStepFlow + triggerACLFlows, dataEntry: stepForm("title"),
			after: triggerACLStepFlow + trigger("    on: {entity: task, updated: true}\n"+
				"    outputs:\n      - set: {properties: {title: x}}\n"),
			want: []string{"sets task.title, which flow \"check\" locks against outside edits"}},
		{name: "a property the step form does not edit",
			before: triggerACLStepFlow + triggerACLFlows, dataEntry: stepForm("notes"),
			after: triggerACLStepFlow + trigger("    on: {entity: task, updated: true}\n"+
				"    outputs:\n      - set: {properties: {title: x}}\n")},
		{name: "a form edit locks what an unchanged trigger sets",
			before: triggerACLStepFlow + trigger("    on: {entity: task, updated: true}\n"+
				"    outputs:\n      - set: {properties: {title: x}}\n"),
			after: triggerACLStepFlow + trigger("    on: {entity: task, updated: true}\n"+
				"    outputs:\n      - set: {properties: {title: x}}\n"),
			dataEntryBefore: stepForm(), dataEntry: stepForm("title"),
			want: []string{"sets task.title, which flow \"check\" locks against outside edits"}},
		{name: "a field granted on another face only",
			acl:   "    fields:\n      doc@draft: [{field: x}]\n      doc@published: [{field: y}]\n",
			after: trigger("    on: {entity: doc, updated: true}\n    outputs:\n      - set: {properties: {x: v}}\n"),
			want:  []string{"sets doc.x, which you may not change yourself"}},
		{name: "a field granted on every face",
			acl:   "    fields:\n      doc@draft: [{field: x}]\n      doc@published: [{field: x}]\n",
			after: trigger("    on: {entity: doc, updated: true}\n    outputs:\n      - set: {properties: {x: v}}\n")},
		{name: "a second hop creates a record read more widely",
			acl: "  public:\n    read: [feed]\n",
			before: trigger("    on: {entity: note, created: true}\n" +
				"    outputs:\n      - create: {type: feed, properties: {text: \"{{new.body}}\"}}\n"),
			after: trigger("    on: {entity: note, created: true}\n"+
				"    outputs:\n      - create: {type: feed, properties: {text: \"{{new.body}}\"}}\n") +
				"  spawn:\n    on: {entity: task, created: true}\n" +
				"    outputs:\n      - create: {type: note, relate: {type: about, from: subject}}\n",
			want: []string{"fire automation \"added\", which creates feed records from task records"}},
		{name: "a schema edit keeps an unchanged trigger's old problem",
			acl: "client_baselines:\n  apps:\n    applies_to: [app]\n    redact:\n      task: [notes]\n",
			before: trigger("    on: {entity: task, updated: true}\n" +
				"    outputs:\n      - set: {properties: {title: \"{{new.notes}}\"}}\n"),
			after: trigger("    on: {entity: task, updated: true}\n" +
				"    outputs:\n      - set: {properties: {title: \"{{new.notes}}\"}}\n"),
			schemaAfter: strings.Replace(triggerACLSchema, "    label: Note\n", "    label: Memo\n", 1)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.before
			if before == "" {
				before = triggerACLFlows
			}
			schemaAfter := tc.schemaAfter
			if schemaAfter == "" {
				schemaAfter = triggerACLSchema
			}
			aclText := triggerACLBase
			if strings.HasPrefix(tc.acl, "client_baselines:") {
				aclText += tc.acl
			} else {
				aclText = strings.Replace(aclText, "    permissions: [config:edit]\n",
					"    permissions: [config:edit]\n"+tc.acl, 1)
			}
			dataEntryBefore := tc.dataEntryBefore
			if dataEntryBefore == "" {
				dataEntryBefore = tc.dataEntry
			}
			got, err := CheckTriggerACL(configedit.TriggerEdit{
				ACL:          []byte(aclText),
				SchemaBefore: []byte(triggerACLSchema), SchemaAfter: []byte(schemaAfter),
				FlowsBefore: []byte(before), FlowsAfter: []byte(tc.after),
				DataEntryBefore: []byte(dataEntryBefore), DataEntryAfter: []byte(tc.dataEntry),
			}, tc.editor)
			if err != nil {
				t.Fatal(err)
			}
			var all []string
			for _, p := range got {
				all = append(all, p.Message)
			}
			joined := strings.Join(all, "\n")
			if len(tc.want) == 0 && len(got) > 0 {
				t.Fatalf("want no problems, got:\n%s", joined)
			}
			for _, w := range tc.want {
				if !strings.Contains(joined, w) {
					t.Errorf("want a problem containing %q, got:\n%s", w, joined)
				}
			}
		})
	}
}

func TestCheckTriggerACL_FailsClosed(t *testing.T) {
	type input struct {
		configedit.TriggerEdit
		Editor TriggerEditor
	}
	good := input{
		TriggerEdit: configedit.TriggerEdit{
			ACL:          []byte(triggerACLBase),
			SchemaBefore: []byte(triggerACLSchema), SchemaAfter: []byte(triggerACLSchema),
			FlowsBefore: []byte(triggerACLFlows), FlowsAfter: []byte(triggerACLFlows),
		},
		Editor: fakeEditor{},
	}
	tests := []struct {
		name string
		edit func(*input)
		want string
	}{
		{"no editor", func(in *input) { in.Editor = nil }, "no editor"},
		{"no acl.yaml", func(in *input) { in.ACL = nil }, "acl.yaml is absent"},
		{"a broken acl.yaml", func(in *input) { in.ACL = []byte("roles: [") }, "acl"},
		{"acl.yaml names an unknown user type", func(in *input) {
			in.ACL = []byte(strings.Replace(triggerACLBase, "user_entity_type: person", "user_entity_type: ghost", 1))
		}, "ghost"},
		{"a broken trigger", func(in *input) {
			in.FlowsAfter = []byte("triggers:\n  x:\n    on: {entity: ghost, created: true}\n")
		}, "as saved"},
		{"a broken schema", func(in *input) { in.SchemaBefore = []byte("entities: [") }, "as read"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := good
			tc.edit(&in)
			_, err := CheckTriggerACL(in.TriggerEdit, in.Editor)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestRequestEditor_RejectsNil(t *testing.T) {
	if _, err := RequestEditor(nil); err == nil {
		t.Fatal("want error")
	}
}

// TestCheckTriggerACL_StepFormEdits pins how an edit of data-entry.yaml
// alone is judged: only a change to what a step may send is judged, and a
// form may not add a property the flow locks in the step's state
// (RR-3IMPOH2, RR-BZASHNZ).
func TestCheckTriggerACL_StepFormEdits(t *testing.T) {
	lockedFlow := strings.Replace(triggerACLStepFlow, "    initial: open\n",
		"    initial: open\n    outside_edits:\n      properties:\n        owner: deny\n", 1)
	tests := []struct {
		name          string
		acl           string
		noFlows       bool
		before, after string
		want          string
		wantErr       string
	}{
		{name: "a form edit with triggers only and no acl.yaml", noFlows: true,
			before: stepForm("notes"), after: stepForm("notes", "title")},
		{name: "an unrelated form edit", acl: triggerACLBase,
			before: stepForm("notes"), after: stepForm("notes") + "  other:\n    entity_type: note\n    fields: []\n"},
		{name: "reordering the step's fields", acl: triggerACLBase,
			before: stepForm("notes", "title"), after: stepForm("title", "notes")},
		{name: "adding a locked property", acl: triggerACLBase,
			before: stepForm("notes"), after: stepForm("notes", "owner"),
			want: `Form "task_check" adds "owner" to the step of flow "check" in state "open"`},
		{name: "adding an unlocked property is judged against acl.yaml", acl: triggerACLBase,
			before: stepForm("notes"), after: stepForm("notes", "title")},
		{name: "a step form made unusable", acl: triggerACLBase,
			before: stepForm("notes"), after: "forms: {}\n", wantErr: "as saved"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flows := lockedFlow + triggerACLFlows
			if tc.noFlows {
				flows = triggerACLFlows
			}
			got, err := CheckTriggerACL(configedit.TriggerEdit{
				ACL:          []byte(tc.acl),
				SchemaBefore: []byte(triggerACLSchema), SchemaAfter: []byte(triggerACLSchema),
				FlowsBefore: []byte(flows), FlowsAfter: []byte(flows),
				DataEntryBefore: []byte(tc.before), DataEntryAfter: []byte(tc.after),
			}, fakeEditor{})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if len(got) > 0 {
					t.Fatalf("want no problem, got %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Form != "task_check" || !strings.Contains(got[0].Message, tc.want) {
				t.Fatalf("got %+v, want one task_check problem containing %q", got, tc.want)
			}
		})
	}
}
