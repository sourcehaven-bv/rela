package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// convSchema converts a note to a task. The task needs an owner the mapping
// does not fill, and the note's stored-in relation is dropped.
const convSchema = `version: "1"
entities:
  note:
    label: Note
    id_prefix: "NOTE-"
    id_type: manual
    properties:
      title: {type: string, required: true}
  task:
    label: Task
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      owner: {type: string, required: true}
  peer:
    label: Peer
    id_prefix: "PEER-"
    id_type: manual
    properties: {title: {type: string}}
relations:
  relates-to:
    label: Relates to
    from: [note, task]
    to: [peer]
  stored-in:
    label: Stored in
    from: [note]
    to: [peer]
conversions:
  note-to-task:
    label: Make task
    from: note
    to: task
    fields:
      title: "{{source.title}}"
    relations:
      relates-to: relates-to
      stored-in: drop
`

// conversionServer serves the conversion tools over a real Converter. alice
// may convert notes; bob may read and write but holds no convert: grant;
// carol has no assignment and sees nothing.
func conversionServer(t *testing.T) (h conversionHandler, st store.Store, as func(string) context.Context) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(convSchema))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	mem := memstore.New()
	ctx := context.Background()
	note := entity.New("NOTE-1", "note")
	note.SetString("title", "Call the vendor")
	for _, e := range []*entity.Entity{note, entity.New("PEER-1", "peer"), entity.New("PEER-2", "peer")} {
		if cErr := mem.CreateEntity(ctx, e); cErr != nil {
			t.Fatalf("seed %s: %v", e.ID, cErr)
		}
	}
	for _, k := range []entity.RelationKey{
		{From: "NOTE-1", Type: "relates-to", To: "PEER-1"},
		{From: "NOTE-1", Type: "stored-in", To: "PEER-2"},
	} {
		if _, rErr := mem.CreateRelation(ctx, k, nil); rErr != nil {
			t.Fatalf("seed relation: %v", rErr)
		}
	}
	all := []string{"note", "task", "peer"}
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles: map[string]acl.RoleDef{
			"converter": {Read: all, Create: all, Update: all, Delete: all, Convert: []string{"note"}},
			"editor":    {Read: all, Create: all, Update: all, Delete: all},
		},
		Assignments: map[string]string{"alice": "converter", "bob": "editor"},
	}, aclstore.MustNewGraph(mem), mem)
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}
	svc := appbuildtest.New(meta, appbuildtest.WithStore(mem), appbuildtest.WithDeclarative(d))
	t.Cleanup(func() { _ = svc.Close() })
	conv, err := entitymanager.NewConverter(entitymanager.FieldGated(svc.EntityManager()))
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	deps := newTestDeps(t, meta, mem)
	deps.Store = svc.GatedReads().Reader
	deps.Conversions = conv
	srv := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(srv, deps)
	return group(srv, selConversions), mem, func(user string) context.Context {
		return acltest.Bind(principal.With(ctx, principal.Principal{User: user, Tool: principal.ToolMCP}), t, d)
	}
}

func convert(ctx context.Context, t *testing.T, h conversionHandler, args map[string]any) (string, bool) {
	t.Helper()
	res, err := h.handleConvertEntity(ctx, makeToolRequest(args))
	if err != nil {
		t.Fatalf("convert_entity: %v", err)
	}
	return getResultText(t, res), res.IsError
}

func TestConversionTools_Registration(t *testing.T) {
	t.Parallel()
	meta, st := makeTestFixture(t)
	deps := newTestDeps(t, meta, st)
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if names := listedTools(t, srv); slices.Contains(names, "convert_entity") {
		t.Error("convert_entity registered without a converter")
	}
	conv, err := entitymanager.NewConverter(appbuildtest.New(meta, appbuildtest.WithStore(st)).EntityManager())
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	deps.Conversions = conv
	srv, err = NewServer(deps, "test", WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	names := listedTools(t, srv)
	for _, name := range []string{"list_conversions", "convert_entity"} {
		if !slices.Contains(names, name) {
			t.Errorf("%s not registered with a converter", name)
		}
	}
}

func TestListConversions(t *testing.T) {
	t.Parallel()
	h, _, as := conversionServer(t)
	for _, tc := range []struct {
		user    string
		allowed bool
	}{{"alice", true}, {"bob", false}} {
		res, err := h.handleListConversions(as(tc.user), makeToolRequest(map[string]any{"id": "NOTE-1"}))
		if err != nil || res.IsError {
			t.Fatalf("%s: list_conversions: %v %s", tc.user, err, getResultText(t, res))
		}
		var out struct {
			Conversions []conversionOfferView `json:"conversions"`
		}
		if uErr := json.Unmarshal([]byte(getResultText(t, res)), &out); uErr != nil {
			t.Fatal(uErr)
		}
		if len(out.Conversions) != 1 || out.Conversions[0].Name != "note-to-task" ||
			out.Conversions[0].To != "task" || out.Conversions[0].Allowed != tc.allowed {

			t.Errorf("%s: offers = %+v", tc.user, out.Conversions)
		}
	}
	res, _ := h.handleListConversions(as("carol"), makeToolRequest(map[string]any{"id": "NOTE-1"}))
	if !res.IsError || !strings.Contains(getResultText(t, res), "not found") {
		t.Errorf("carol: %s", getResultText(t, res))
	}
}

func TestConvertEntity_Flow(t *testing.T) {
	t.Parallel()
	h, st, as := conversionServer(t)
	ctx := as("alice")
	base := map[string]any{"id": "NOTE-1", "conversion": "note-to-task"}
	with := func(extra map[string]any) map[string]any {
		out := map[string]any{}
		maps.Copy(out, base)
		maps.Copy(out, extra)
		return out
	}

	// The owner is required and not mapped.
	text, isErr := convert(ctx, t, h, base)
	if !isErr || !strings.Contains(text, `"owner"`) || !strings.Contains(text, `"blocked": true`) {
		t.Fatalf("missing owner: %v %s", isErr, text)
	}

	// Dry run names the dropped relation.
	text, isErr = convert(ctx, t, h, with(map[string]any{"properties": map[string]any{"owner": "alice"}, "dry_run": true}))
	if isErr || !strings.Contains(text, `"stored-in"`) {
		t.Fatalf("dry run: %v %s", isErr, text)
	}

	// Without confirm the drop is refused and nothing is written.
	text, isErr = convert(ctx, t, h, with(map[string]any{"properties": map[string]any{"owner": "alice"}}))
	if !isErr || !strings.Contains(text, `"needs_confirm": true`) {
		t.Fatalf("unconfirmed: %v %s", isErr, text)
	}
	if _, err := st.GetEntity(context.Background(), entity.Ref{ID: "NOTE-1"}); err != nil {
		t.Fatalf("unconfirmed conversion wrote: %v", err)
	}

	text, isErr = convert(ctx, t, h, with(map[string]any{"properties": map[string]any{"owner": "alice"}, "confirm": true}))
	if isErr {
		t.Fatalf("confirmed: %s", text)
	}
	var res conversionResultView
	if err := json.Unmarshal([]byte(text), &res); err != nil {
		t.Fatal(err)
	}
	if res.OldID != "NOTE-1" || !strings.HasPrefix(res.ID, "TASK-") || res.RelationsDropped != 1 {
		t.Errorf("result = %+v", res)
	}
	got, err := st.GetEntity(context.Background(), entity.Ref{ID: res.ID})
	if err != nil || got.Type != "task" || got.GetString("owner") != "alice" || got.GetString("title") != "Call the vendor" {
		t.Errorf("converted entity = %+v, %v", got, err)
	}
	if _, err := st.GetEntity(context.Background(), entity.Ref{ID: "NOTE-1"}); err == nil {
		t.Error("NOTE-1 still stored after conversion")
	}
}

func TestConvertEntity_ACL(t *testing.T) {
	t.Parallel()
	h, st, as := conversionServer(t)
	args := map[string]any{
		"id": "NOTE-1", "conversion": "note-to-task",
		"properties": map[string]any{"owner": "x"}, "confirm": true,
	}
	text, isErr := convert(as("bob"), t, h, args)
	if !isErr || !strings.Contains(text, "not permitted") {
		t.Errorf("bob: %v %s", isErr, text)
	}
	text, isErr = convert(as("carol"), t, h, args)
	if !isErr || text != "entity not found: NOTE-1" {
		t.Errorf("carol: %v %s", isErr, text)
	}
	if _, err := st.GetEntity(context.Background(), entity.Ref{ID: "NOTE-1"}); err != nil {
		t.Errorf("refused conversion wrote: %v", err)
	}
	text, isErr = convert(as("alice"), t, h, map[string]any{"id": "NOTE-1", "conversion": "nope"})
	if !isErr {
		t.Errorf("unknown conversion: %s", text)
	}
}
