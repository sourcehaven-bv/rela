package entitymanager_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// copyAutomationMetaYAML declares a promote copy, a second copy into the
// same face, and automations on the target face: one per event key, one
// scoped to a copy, and a background action scoped to a copy.
const copyAutomationMetaYAML = `
version: "1"
types:
  doc_stage:
    values: [draft, review, final]
    initial: draft
    transitions:
      - {move: submit, from: draft, to: review}
      - {move: finish, from: review, to: final}
entities:
  doc:
    label: Doc
    id_prefix: DOC
    faces:
      draft: {}
      adopted: {}
    properties:
      title: {type: string}
      stamped_by: {type: string}
      created_mark: {type: string}
      updated_mark: {type: string}
      adopted_on: {type: string}
      stage: {type: doc_stage}
copies:
  adopt:
    from: doc@draft
    to: doc@adopted
    fields: all
    set:
      adopted_on: "{{today}}"
    guard:
      permission: adopt
  restore:
    from: doc@draft
    to: doc@adopted
    fields: all
    guard:
      permission: adopt
  finalize:
    from: doc@draft
    to: doc@adopted
    fields: all
    guard:
      permission: adopt
automations:
  - name: on-created
    on: {entity: doc, faces: [adopted], created: true}
    do: [{set: created_mark, value: "yes"}]
  - name: on-updated
    on: {entity: doc, faces: [adopted], updated: true}
    do: [{set: updated_mark, value: "yes"}]
  - name: on-adopt
    on: {entity: doc, copy: adopt}
    do: [{set: stamped_by, value: "adopt"}]
  - name: push-adopted
    on: {entity: doc, copy: [adopt]}
    do: [{lua_file: push.lua, background: true}]
  - name: skip-to-final
    on: {entity: doc, copy: finalize}
    do: [{set: stage, value: final}]
`

type copyBackground struct {
	got []autocascade.BackgroundScript
}

func (b *copyBackground) EnqueueScript(_ context.Context, s autocascade.BackgroundScript) error {
	b.got = append(b.got, s)
	return nil
}

// TestCopyRunsAutomations pins TKT-50DZEHI: a copy runs the target face's
// `created` automations when it creates the face and its `updated`
// automations when it overwrites it, `on.copy:` selects one copy, and a
// background action scoped to a copy is scheduled for the target face.
func TestCopyRunsAutomations(t *testing.T) {
	t.Parallel()
	today := time.Now().Format("2006-01-02")
	tests := []struct {
		name        string
		definition  string
		seedAdopted bool
		want        map[string]any
		wantJobs    int
	}{
		{
			name: "adopt creates the face", definition: "adopt",
			want: map[string]any{
				"created_mark": "yes", "updated_mark": nil, "stamped_by": "adopt", "adopted_on": today,
			},
			wantJobs: 1,
		},
		{
			name: "adopt overwrites the face", definition: "adopt", seedAdopted: true,
			want: map[string]any{
				"created_mark": nil, "updated_mark": "yes", "stamped_by": "adopt", "adopted_on": today,
			},
			wantJobs: 1,
		},
		{
			name: "another copy skips on.copy automations", definition: "restore", seedAdopted: true,
			want: map[string]any{
				"created_mark": nil, "updated_mark": "yes", "stamped_by": nil, "adopted_on": nil,
			},
			wantJobs: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mgr, st, bg, sink := newCopyAutomationManager(t)
			ctx := context.Background()
			seed := func(face entity.Face, props map[string]any) {
				t.Helper()
				if err := st.CreateEntity(ctx, &entity.Entity{ID: "DOC-1", Type: "doc", Face: face, Properties: props}); err != nil {
					t.Fatalf("seed %s: %v", face, err)
				}
			}
			seed("draft", map[string]any{"title": "new"})
			if tc.seedAdopted {
				seed("adopted", map[string]any{"title": "old"})
			}

			res, err := mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: tc.definition, SourceID: "DOC-1"})
			if err != nil {
				t.Fatalf("copy: %v", err)
			}
			if len(res.AutomationErrors) > 0 {
				t.Fatalf("automation errors: %v", res.AutomationErrors)
			}
			got, err := st.GetEntity(ctx, entity.Ref{ID: "DOC-1", Face: "adopted"})
			if err != nil {
				t.Fatalf("read adopted: %v", err)
			}
			if got.Properties["title"] != "new" {
				t.Errorf("title = %v, want the source's value", got.Properties["title"])
			}
			for prop, want := range tc.want {
				if v := got.Properties[prop]; v != want {
					t.Errorf("%s = %v, want %v", prop, v, want)
				}
			}
			if res.Entity.Properties["created_mark"] != got.Properties["created_mark"] {
				t.Errorf("result entity is not the stored row after the automations")
			}
			if len(bg.got) != tc.wantJobs {
				t.Fatalf("background jobs = %v, want %d", bg.got, tc.wantJobs)
			}
			if tc.wantJobs > 0 && bg.got[0].Ref != (entity.Ref{ID: "DOC-1", Face: "adopted"}) {
				t.Errorf("background job ref = %v, want DOC-1@adopted", bg.got[0].Ref)
			}
			var ops []string
			for _, r := range sink.Records() {
				ops = append(ops, r.Op)
			}
			if !slices.Contains(ops, audit.OpCopyState) {
				t.Errorf("audit ops = %v, want a copy record", ops)
			}
			// The automations' properties are part of the copy's one write.
			if slices.Contains(ops, audit.OpUpdateEntity) || slices.Contains(ops, audit.OpCreateEntity) {
				t.Errorf("audit ops = %v, want the copy record alone", ops)
			}
		})
	}
}

// TestCopyAutomationTransitionChecked: a property an automation sets on a
// copy's target face makes a state-machine move, which must be legal. The
// illegal move refuses the copy before anything is written.
func TestCopyAutomationTransitionChecked(t *testing.T) {
	t.Parallel()
	mgr, st, bg, _ := newCopyAutomationManager(t)
	ctx := context.Background()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "DOC-1", Type: "doc", Face: "draft", Properties: map[string]any{"title": "new", "stage": "draft"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: "finalize", SourceID: "DOC-1"})
	if err == nil {
		t.Fatal("copy succeeded; want the illegal draft -> final move refused")
	}
	if !strings.Contains(err.Error(), "final") {
		t.Errorf("error = %v, want it to name the refused move", err)
	}
	if _, gerr := st.GetEntity(ctx, entity.Ref{ID: "DOC-1", Face: "adopted"}); gerr == nil {
		t.Error("the adopted face was written; a refused copy must write nothing")
	}
	if len(bg.got) != 0 {
		t.Errorf("background jobs = %v, want none for a refused copy", bg.got)
	}
}

// TestCopyAutomationsNeedTheGuard: a caller the copy's guard refuses gets no
// automation either. The guard is checked before the automations are
// evaluated, so a refused copy writes nothing, records no copy and schedules
// no background action.
func TestCopyAutomationsNeedTheGuard(t *testing.T) {
	t.Parallel()
	mgr, st, bg, sink := newCopyAutomationManagerWith(t, allowGuard{allow: false})
	ctx := context.Background()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "DOC-1", Type: "doc", Face: "draft", Properties: map[string]any{"title": "new"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: "adopt", SourceID: "DOC-1"}); err == nil {
		t.Fatal("copy succeeded; want the guard to refuse it")
	}
	if _, gerr := st.GetEntity(ctx, entity.Ref{ID: "DOC-1", Face: "adopted"}); gerr == nil {
		t.Error("the adopted face was written; a refused copy must write nothing")
	}
	if len(bg.got) != 0 {
		t.Errorf("background jobs = %v, want none for a refused copy", bg.got)
	}
	for _, r := range sink.Records() {
		if r.Op == audit.OpCopyState {
			t.Errorf("audit has a copy record %+v for a refused copy", r)
		}
	}
}

// TestCopyWithoutAutomationsIsUnchanged: a manager with no automation engine
// copies as before and reports no automation output.
func TestCopyWithoutAutomationsIsUnchanged(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(copyAutomationMetaYAML))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: transitions.NoTransitions(), FieldGate: entitymanager.AllowAllFieldGate{},
		CopyGuard: allowGuard{allow: true}, AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := context.Background()
	if serr := st.CreateEntity(ctx, &entity.Entity{
		ID: "DOC-1", Type: "doc", Face: "draft", Properties: map[string]any{"title": "new"},
	}); serr != nil {
		t.Fatalf("seed: %v", serr)
	}
	res, err := mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: "adopt", SourceID: "DOC-1"})
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if res.Entity.Properties["created_mark"] != nil || len(res.AutomationErrors) > 0 {
		t.Errorf("result = %+v, want no automation output", res)
	}
}

func newCopyAutomationManager(t *testing.T) (*entitymanager.Manager, store.Store, *copyBackground, *audit.Memory) {
	t.Helper()
	return newCopyAutomationManagerWith(t, allowGuard{allow: true})
}

// newCopyAutomationManagerWith is newCopyAutomationManager with guard as the
// copy guard.
func newCopyAutomationManagerWith(
	t *testing.T, guard entitymanager.CopyGuard,
) (*entitymanager.Manager, store.Store, *copyBackground, *audit.Memory) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(copyAutomationMetaYAML))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	engine, err := automation.NewEngineFromMetamodel(meta, meta.Automations)
	if err != nil {
		t.Fatalf("automation engine: %v", err)
	}
	bg := &copyBackground{}
	runner, err := autocascade.New(autocascade.Deps{Engine: engine, Background: bg})
	if err != nil {
		t.Fatalf("autocascade.New: %v", err)
	}
	machines, err := transitions.CompileTransitions(meta)
	if err != nil {
		t.Fatalf("transitions.CompileTransitions: %v", err)
	}
	st := memstore.New()
	sink := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        meta,
		Templater:   nopTemplater{},
		Audit:       sink,
		ACL:         acl.NopACL{},
		Transitions: machines,
		FieldGate:   entitymanager.AllowAllFieldGate{},
		CopyGuard:   guard,
		AnchorGuard: entitymanager.NopAnchorGuard{},
		Automations: engine,
		Cascade:     runner,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st, bg, sink
}
