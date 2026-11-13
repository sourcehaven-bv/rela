package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestSyntheticAutomations_Golden replays hand-picked events against a
// schema built to reach every branch of the matcher: typed, untranspilable
// and undeclared `when:` clauses, type-less automations on unknown and alias
// types, conditions that hold, fail or cannot be evaluated, faces, copies,
// `validate:` checks, piles, notify, create and the script actions. Three
// engines run it: with a metamodel and a working traversal binder, with a
// failing binder, and string-only (NewEngine). The golden file was written
// by the engine before automations compiled onto flow triggers
// (TKT-GS7THPA). Run with -update to rewrite it on purpose.
func TestSyntheticAutomations_Golden(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "golden", "synthetic-schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := metamodel.Parse(raw)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	now := time.Date(2026, 11, 13, 9, 30, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	vars := TemplateVars{Now: clock, User: UserVars{Name: "Golden", Email: "golden@example.com"}}

	withMeta := func(b TraversalBinder) *Engine {
		e, buildErr := NewEngineFromMetamodel(meta, meta.Automations, WithTraversals(b), WithClock(clock))
		if buildErr != nil {
			t.Fatalf("build engine: %v", buildErr)
		}
		e.SetTemplateVars(vars)
		return e
	}
	plain := make([]Automation, 0, len(meta.Automations))
	for _, def := range meta.Automations {
		a, convErr := convertFromMetamodel(def)
		if convErr != nil {
			t.Fatal(convErr)
		}
		plain = append(plain, a)
	}
	stringOnly := NewEngine(plain)
	stringOnly.SetTemplateVars(vars)

	engines := []struct {
		name   string
		engine *Engine
	}{
		{"binder-true", withMeta(&stubBinder{answer: true})},
		{"binder-false", withMeta(&stubBinder{answer: false})},
		{"binder-error", withMeta(&stubBinder{err: errors.New("store down")})},
		{"string-only", stringOnly},
	}

	var buf bytes.Buffer
	for _, eng := range engines {
		for _, ev := range syntheticEvents() {
			res := eng.engine.Process(context.Background(), ev.event)
			line, mErr := json.Marshal(struct {
				Engine string  `json:"engine"`
				Event  string  `json:"event"`
				Result *Result `json:"result"`
			}{eng.name, ev.name, res})
			if mErr != nil {
				t.Fatalf("marshal: %v", mErr)
			}
			buf.Write(line)
			buf.WriteByte('\n')
		}
	}

	golden := filepath.Join("testdata", "golden", "synthetic.jsonl")
	if *updateGolden {
		if wErr := os.WriteFile(golden, buf.Bytes(), 0o600); wErr != nil {
			t.Fatal(wErr)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create it): %v", err)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		diffLines(t, string(want), buf.String())
	}
}

func syntheticEvents() []goldenEvent {
	task := func(typ string, props map[string]any) *entity.Entity {
		p := map[string]any{"title": "Golden task"}
		maps.Copy(p, props)
		return &entity.Entity{ID: "TASK-1", Type: typ, Properties: p, Content: "body"}
	}
	doc := func(face entity.Face) *entity.Entity {
		return &entity.Entity{ID: "DOC-1", Type: "doc", Face: face, Properties: map[string]any{"title": "Doc"}}
	}
	full := map[string]any{
		"status": "open", "count": 10, "due": "2026-11-20", "owner": "PROJ-1", "kind": "bug",
		"watchers": []any{"P-1", " P-2 ", "P-1", ""}, "nickname": "bob",
	}
	small := map[string]any{"status": "open", "count": 2, "due": "2027-01-01", "owner": "", "kind": "a b"}
	with := func(base map[string]any, k string, v any) map[string]any {
		out := maps.Clone(base)
		out[k] = v
		return out
	}
	var out []goldenEvent
	for _, typ := range []string{"task", "todo", "Task", "ghost"} {
		for _, variant := range []struct {
			name  string
			props map[string]any
		}{{"full", full}, {"small", small}} {
			props := variant.props
			p := typ + "/" + variant.name + "/"
			out = append(out,
				goldenEvent{p + "created", Event{Type: EventEntityCreated, Entity: task(typ, props)}},
				goldenEvent{p + "updated-title", Event{Type: EventEntityUpdated,
					OldEntity: task(typ, props), Entity: task(typ, with(props, "title", "New title"))}},
				goldenEvent{p + "updated-nothing", Event{Type: EventEntityUpdated,
					OldEntity: task(typ, props), Entity: task(typ, props)}},
				goldenEvent{p + "status-done", Event{Type: EventEntityUpdated,
					OldEntity: task(typ, props), Entity: task(typ, with(props, "status", "done"))}},
				goldenEvent{p + "status-review", Event{Type: EventEntityUpdated,
					OldEntity: task(typ, props), Entity: task(typ, with(props, "status", "review"))}},
				goldenEvent{p + "kind-gated", Event{Type: EventEntityUpdated,
					OldEntity: task(typ, props), Entity: task(typ, with(props, "kind", "gated"))}},
				goldenEvent{p + "owner-changed", Event{Type: EventEntityUpdated,
					OldEntity: task(typ, props), Entity: task(typ, with(props, "owner", "alice"))}},
				goldenEvent{p + "owner-cleared", Event{Type: EventEntityUpdated,
					OldEntity: task(typ, with(props, "owner", "alice")), Entity: task(typ, with(props, "owner", ""))}},
				goldenEvent{p + "relation-created", Event{Type: EventRelationCreated, Entity: task(typ, props),
					Relation: entity.NewRelation("TASK-1", "part-of", "PROJ-1")}},
				goldenEvent{p + "relation-removed", Event{Type: EventRelationRemoved, Entity: task(typ, props),
					Relation: entity.NewRelation("TASK-1", "part-of", "PROJ-1")}},
				goldenEvent{p + "relation-other", Event{Type: EventRelationCreated, Entity: task(typ, props),
					Relation: entity.NewRelation("TASK-1", "has-checklist", "CHK-1")}},
				goldenEvent{p + "renamed", Event{Type: EventEntityRenamed, Entity: task(typ, props)}},
				goldenEvent{p + "deleted", Event{Type: EventEntityDeleted, Entity: task(typ, props)}},
				goldenEvent{p + "managed", Event{Type: EventEntityCreated,
					Entity: &entity.Entity{ID: "R-1", Type: "_flow-run", Properties: props}}},
			)
		}
	}
	for _, face := range []entity.Face{"", "nl", "draft", "adopted"} {
		p := "doc/" + string(face) + "/"
		out = append(out,
			goldenEvent{p + "created", Event{Type: EventEntityCreated, Entity: doc(face)}},
			goldenEvent{p + "copy-adopt", Event{Type: EventEntityUpdated, Copy: "adopt",
				OldEntity: doc(face), Entity: doc(face)}},
			goldenEvent{p + "copy-other", Event{Type: EventEntityCreated, Copy: "other", Entity: doc(face)}},
			goldenEvent{p + "renamed", Event{Type: EventEntityRenamed, Entity: doc(face)}},
		)
	}
	out = append(out, goldenEvent{"nil-entity/relation-created", Event{Type: EventRelationCreated,
		Relation: entity.NewRelation("TASK-1", "part-of", "PROJ-1")}})
	return out
}
