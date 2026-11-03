package automation

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const notifySchema = `
version: "1"
piles:
  inbox: {name: Inbox, read_state: true, reasons: {done: done}}
entities:
  task:
    label: Task
    id_type: manual
    properties:
      status: {type: string}
      watchers: {type: string, list: true}
  project:
    label: Project
    id_type: manual
    properties:
      owner: {type: string}
relations:
  part-of:
    from: [task]
    to: [project]
    inverse: has-task
`

func notifyEngine(t *testing.T, action string) (*Engine, error) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(notifySchema + `
automations:
  - name: n
    on: {entity: task, created: true}
    do:
      - ` + action + `
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return NewEngineFromMetamodel(meta, meta.Automations)
}

func TestEngine_NotifyPlans(t *testing.T) {
	ctx := context.Background()
	ent := &entity.Entity{ID: "T-1", Type: "task", Properties: map[string]any{
		"watchers": []any{"P-1", " P-2 ", "P-1", ""},
	}}

	e, err := notifyEngine(t, `notify: {pile: inbox, reason: done, to: {via: part-of, property: owner}}`)
	if err != nil {
		t.Fatal(err)
	}
	got := e.Process(ctx, Event{Type: EventEntityCreated, Entity: ent}).Notifications
	want := []NotificationToSend{{
		Pile: "inbox", Reason: "done", Property: "owner", AutomationName: "n",
		Hops: []NotifyHop{{Relation: "part-of", Target: "project"}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Notifications = %+v, want %+v", got, want)
	}

	e, err = notifyEngine(t, `notify: {pile: inbox, to: {property: watchers}}`)
	if err != nil {
		t.Fatal(err)
	}
	got = e.Process(ctx, Event{Type: EventEntityCreated, Entity: ent}).Notifications
	if len(got) != 1 || !reflect.DeepEqual(got[0].Owners, []string{"P-1", "P-2"}) || len(got[0].Hops) != 0 {
		t.Fatalf("Notifications = %+v, want owners P-1, P-2 read off the task", got)
	}

	empty := &entity.Entity{ID: "T-2", Type: "task"}
	if got := e.Process(ctx, Event{Type: EventEntityCreated, Entity: empty}).Notifications; len(got) != 0 {
		t.Fatalf("no recipients must plan nothing, got %+v", got)
	}
}

func TestEngine_NotifyLoadErrors(t *testing.T) {
	for _, tc := range []struct{ name, action, want string }{
		{"unknown relation", `notify: {pile: inbox, to: {via: nope, property: owner}}`, "notify from \"task\""},
		{"wrong direction", `notify: {pile: inbox, to: {via: has-task, property: owner}}`, "notify from \"task\""},
		{"missing property", `notify: {pile: inbox, to: {via: part-of, property: lead}}`, `"project" has no property "lead"`},
		{"missing own property", `notify: {pile: inbox, to: {property: lead}}`, `"task" has no property "lead"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := notifyEngine(t, tc.action)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
