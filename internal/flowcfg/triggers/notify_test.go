package triggers

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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
	meta := mustMeta(t, notifySchema)
	rules := mustParse(t, meta, `
triggers:
  n:
    on: {entity: task, created: true}
    outputs:
      - `+action+`
`)
	return New(meta, rules)
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

// sharedRulesFlows declares one output of every kind that points to a
// struct, so a write through any of those pointers would show.
const sharedRulesFlows = `
triggers:
  n:
    on: {entity: task, created: true}
    outputs:
      - notify: {pile: inbox, reason: done, to: {via: part-of, property: owner}}
      - notify: {pile: inbox, to: {property: watchers}}
      - relate: {type: part-of, to: P-1}
      - create: {type: project, properties: {owner: "{{new.status}}"}}
      - add_to_pile: {pile: inbox}
`

// TestNew_SharedRules pins that engines built from one []Rule share nothing
// they write. The wiring site parses the rules once and builds one engine per
// tenant, possibly at the same time. Under -race, a write into the shared
// rules fails this test.
func TestNew_SharedRules(t *testing.T) {
	t.Parallel()
	meta := mustMeta(t, notifySchema)
	rules := mustParse(t, meta, sharedRulesFlows)
	ent := &entity.Entity{ID: "T-1", Type: "task", Properties: map[string]any{"watchers": "P-2"}}

	const engines = 4
	errs := make(chan error, engines)
	var wg sync.WaitGroup
	for range engines {
		wg.Go(func() {
			e, err := New(meta, rules)
			if err != nil {
				errs <- err
				return
			}
			got := e.Process(context.Background(), Event{Type: EventEntityCreated, Entity: ent}).Notifications
			if len(got) != 2 || len(got[0].Hops) != 1 {
				errs <- fmt.Errorf("Notifications = %+v, want the resolved path and the direct owners", got)
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// TestNew_LeavesRulesUnchanged pins that [New] does not write into the rules
// it is given, and that an engine does not see later changes to them.
func TestNew_LeavesRulesUnchanged(t *testing.T) {
	t.Parallel()
	meta := mustMeta(t, notifySchema)
	rules := mustParse(t, meta, sharedRulesFlows)
	before := mustParse(t, meta, sharedRulesFlows)

	e, err := New(meta, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rules, before) {
		t.Fatalf("New changed its input:\n got  %+v\n want %+v", rules, before)
	}

	// Changes to the caller's rules after New do not reach the engine.
	rules[0].Do[0].Notify.Pile = "changed"
	rules[0].Do[2].CreateRelation.To = "P-9"
	rules[0].Do[3].CreateEntity.Properties["owner"] = "changed"
	ent := &entity.Entity{ID: "T-1", Type: "task", Properties: map[string]any{"watchers": "P-2"}}
	res := e.Process(context.Background(), Event{Type: EventEntityCreated, Entity: ent})
	if len(res.Notifications) == 0 || res.Notifications[0].Pile != "inbox" {
		t.Errorf("Notifications = %+v, want pile inbox", res.Notifications)
	}
	if len(res.RelationsToCreate) != 1 || res.RelationsToCreate[0].Relation.To != "P-1" {
		t.Errorf("RelationsToCreate = %+v, want one to P-1", res.RelationsToCreate)
	}
	if len(res.EntitiesToCreate) != 1 || res.EntitiesToCreate[0].Properties["owner"] != "" {
		t.Errorf("EntitiesToCreate = %+v, want owner as parsed", res.EntitiesToCreate)
	}
}
