package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

var updateGolden = flag.Bool("update", false, "rewrite the automation golden files")

// goldenSchemas are the in-repo schemas whose automations the golden test
// replays. tickets/ and docs-project/ are the acceptance of TKT-GS7THPA; the
// others widen the matrix.
var goldenSchemas = []string{
	"tickets",
	"docs-project",
	"examples/basecamp",
	"internal/dataentry/testdata/golden/fixture",
}

// TestSchemaAutomations_Golden replays a matrix of events against every
// automation of the in-repo schemas and compares what the engine plans with
// a golden file. The golden files were written by the engine before
// automations compiled onto flow triggers (TKT-GS7THPA), so a difference is
// a behavior change. Run with -update to rewrite them on purpose.
func TestSchemaAutomations_Golden(t *testing.T) {
	for _, dir := range goldenSchemas {
		t.Run(dir, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", dir, "schema.yaml"))
			if err != nil {
				t.Fatalf("read schema: %v", err)
			}
			meta, err := metamodel.Parse(raw)
			if err != nil {
				t.Fatalf("parse schema: %v", err)
			}
			now := time.Date(2026, 11, 13, 9, 30, 0, 0, time.UTC)
			engine, err := NewEngineFromMetamodel(meta, meta.Automations,
				WithTraversals(&stubBinder{answer: true}), WithClock(func() time.Time { return now }))
			if err != nil {
				t.Fatalf("build engine: %v", err)
			}
			engine.SetTemplateVars(TemplateVars{
				Now: func() time.Time { return now }, User: UserVars{Name: "Golden", Email: "golden@example.com"},
			})

			var buf bytes.Buffer
			for _, ev := range goldenEvents(meta) {
				res := engine.Process(context.Background(), ev.event)
				line, mErr := json.Marshal(struct {
					Event  string  `json:"event"`
					Result *Result `json:"result"`
				}{ev.name, res})
				if mErr != nil {
					t.Fatalf("marshal: %v", mErr)
				}
				buf.Write(line)
				buf.WriteByte('\n')
			}

			golden := filepath.Join("testdata", "golden", filepath.Base(dir)+".jsonl")
			if *updateGolden {
				if mkErr := os.MkdirAll(filepath.Dir(golden), 0o755); mkErr != nil {
					t.Fatal(mkErr)
				}
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
		})
	}
}

func diffLines(t *testing.T, want, got string) {
	t.Helper()
	wl := bytes.Split([]byte(want), []byte("\n"))
	gl := bytes.Split([]byte(got), []byte("\n"))
	shown := 0
	for i := 0; i < max(len(wl), len(gl)) && shown < 5; i++ {
		var w, g []byte
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if !bytes.Equal(w, g) {
			t.Errorf("line %d differs:\nwant %s\ngot  %s", i+1, w, g)
			shown++
		}
	}
	if len(wl) != len(gl) {
		t.Errorf("want %d lines, got %d", len(wl), len(gl))
	}
}

type goldenEvent struct {
	name  string
	event Event
}

// goldenEvents builds, for every automation and every type it triggers on,
// the events that could fire it: created, a change of its property from
// `from` to `becomes`, any update, its relation created and removed, a
// rename and a delete. Each entity carries the values its `when:` equalities
// name, so the guarded automations fire too.
func goldenEvents(meta *metamodel.Metamodel) []goldenEvent {
	var out []goldenEvent
	for _, def := range meta.Automations {
		types := []string(def.On.Entity)
		if len(types) == 0 {
			types = slices.Sorted(maps.Keys(meta.Entities))
		}
		for _, typ := range types {
			base := map[string]any{"title": "Golden " + typ, "kind": "enhancement", "owner": "alice"}
			for _, w := range def.On.When {
				if f, err := filter.Parse(w); err == nil && f.Operator == filter.OpEqual {
					base[f.Property] = f.Value
				}
			}
			mk := func(id string, extra map[string]any) *entity.Entity {
				props := maps.Clone(base)
				maps.Copy(props, extra)
				return &entity.Entity{ID: id, Type: typ, Properties: props, Content: "body"}
			}
			prefix := fmt.Sprintf("%s/%s/", def.Name, typ)
			oldV, newV := "golden-old", "golden-new"
			if def.On.From != "" {
				oldV = def.On.From
			}
			if def.On.Becomes != "" {
				newV = def.On.Becomes
			}
			created := map[string]any{}
			if def.On.Property != "" {
				created[def.On.Property] = newV
			}
			out = append(out, goldenEvent{prefix + "created",
				Event{Type: EventEntityCreated, Entity: mk("G-1", created)}})
			if def.On.Property != "" {
				out = append(out, goldenEvent{prefix + "property",
					Event{Type: EventEntityUpdated,
						OldEntity: mk("G-1", map[string]any{def.On.Property: oldV}),
						Entity:    mk("G-1", map[string]any{def.On.Property: newV})}})
			}
			out = append(out, goldenEvent{prefix + "updated",
				Event{Type: EventEntityUpdated, OldEntity: mk("G-1", nil),
					Entity: mk("G-1", map[string]any{"title": "Changed"})}})
			for _, rel := range []struct {
				name string
				typ  EventType
				rel  string
			}{{"relation_created", EventRelationCreated, def.On.RelationCreated},
				{"relation_removed", EventRelationRemoved, def.On.RelationRemoved}} {
				if rel.rel == "" {
					continue
				}
				out = append(out, goldenEvent{prefix + rel.name, Event{Type: rel.typ, Entity: mk("G-1", nil),
					Relation: entity.NewRelation("G-1", rel.rel, "G-2")}})
			}
			out = append(out,
				goldenEvent{prefix + "renamed", Event{Type: EventEntityRenamed, Entity: mk("G-3", created)}},
				goldenEvent{prefix + "deleted", Event{Type: EventEntityDeleted, Entity: mk("G-1", nil)}})
			for _, c := range def.On.Copy {
				out = append(out, goldenEvent{prefix + "copy:" + c,
					Event{Type: EventEntityUpdated, Copy: c, OldEntity: mk("G-1", nil), Entity: mk("G-1", nil)}})
			}
		}
	}
	return out
}
