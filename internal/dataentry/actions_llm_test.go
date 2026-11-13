package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/ai"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// LLM actions (TKT-4USH1HO): the action reads as the caller, calls the
// provider, and returns a proposal. It never writes.

// fakeChat is an ai.Provider that records the request and returns a fixed
// reply.
type fakeChat struct {
	mu    sync.Mutex
	reply string
	// finish is the reply's finish reason; empty means "stop".
	finish string
	err    error
	got    []ai.ChatRequest
}

func (f *fakeChat) Chat(_ context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, req)
	if f.err != nil {
		return nil, f.err
	}
	finish := f.finish
	if finish == "" {
		finish = "stop"
	}
	return &ai.ChatResponse{Content: f.reply, Model: "fake-1", FinishReason: finish}, nil
}

func (*fakeChat) Embed(context.Context, ai.EmbedRequest) (*ai.EmbedResponse, error) {
	return nil, &ai.Error{Kind: ai.ErrBadRequest, Message: "not supported"}
}

// sent is everything the model was sent, joined, for leak assertions.
func (f *fakeChat) sent(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.got) != 1 {
		t.Fatalf("provider called %d times, want 1", len(f.got))
	}
	var b strings.Builder
	for _, m := range f.got[0].Messages {
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func llmMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(`
entities:
  ticket:
    label: Ticket
    display_property: title
    id_prefix: TKT
    properties:
      title: { type: string }
      priority: { type: enum, values: [low, high] }
      secret: { type: string }
  concept:
    label: Concept
    display_property: title
    id_prefix: CON
    properties:
      title: { type: string }
      area: { type: string }
relations:
  affects:
    from: [ticket]
    to: [concept]
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	return m
}

func triageAction() dataentryconfig.Action {
	return dataentryconfig.Action{
		Label:       "Triage",
		Permission:  "ticket.triage",
		AvailableOn: &dataentryconfig.ActionScope{EntityTypes: []string{"ticket"}},
		LLM: &dataentryconfig.LLMAction{
			Prompt: "Triage the ticket.",
			Inputs: dataentryconfig.LLMInputs{
				Properties: []string{"title", "secret"},
				Content:    &dataentryconfig.LLMContentInput{},
			},
			Outputs: dataentryconfig.LLMOutputs{
				Set: []string{"priority"},
				Relate: []dataentryconfig.LLMRelateOutput{{
					Relation: "affects", Candidates: "entity.area == 'auth'", Fields: []string{"area"}, Max: 2,
				}},
			},
		},
	}
}

type llmApp struct {
	app   *App
	decl  *acl.Declarative
	store store.Store
	chat  *fakeChat
}

func newLLMApp(t *testing.T, reply string) llmApp {
	t.Helper()
	return buildLLMApp(t, llmFixture{
		meta: llmMeta(t),
		seed: []*entity.Entity{
			{ID: "TKT-1", Type: "ticket", Content: "Users cannot log in.",
				Properties: map[string]any{"title": "Login broken", "priority": "low", "secret": "S3CRET"}},
			{ID: "CON-1", Type: "concept", Properties: map[string]any{"title": "Sessions", "area": "auth"}},
			{ID: "CON-2", Type: "concept", Properties: map[string]any{"title": "Invoices", "area": "billing"}},
			{ID: "CON-3", Type: "concept", Properties: map[string]any{"title": "Passwords", "area": "auth"}},
		},
		edges:  []entity.RelationKey{{From: "TKT-1", Type: "affects", To: "CON-3"}},
		read:   []string{"ticket", "concept"},
		action: triageAction(),
		reply:  reply,
	})
}

// llmFixture describes an LLM action app: its schema, seed data, the types
// alice and bob may read, the "triage" action, and an optional store wrapper.
type llmFixture struct {
	meta   *metamodel.Metamodel
	seed   []*entity.Entity
	edges  []entity.RelationKey
	read   []string
	action dataentryconfig.Action
	reply  string
	// wrap, when set, wraps the seeded store; every read the app makes goes
	// through the wrapper.
	wrap func(store.Store) store.Store
}

func buildLLMApp(t *testing.T, fx llmFixture) llmApp {
	t.Helper()
	root := t.TempDir()
	meta := fx.meta
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths := &project.Context{Root: root, CacheDir: filepath.Join(root, ".rela")}
	st := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths)).Store()
	ctx := context.Background()
	for _, e := range fx.seed {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	for _, k := range fx.edges {
		if _, err := st.CreateRelation(ctx, k, nil); err != nil {
			t.Fatalf("seed relation %v: %v", k, err)
		}
	}
	if fx.wrap != nil {
		st = fx.wrap(st)
	}
	role := func(perms ...string) acl.RoleDef {
		return acl.RoleDef{Read: fx.read, Update: []string{"ticket"}, Permissions: perms}
	}
	decl := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"triager": role("ticket.triage"), "reader": role()},
		Assignments: map[string]string{"alice": "triager", "bob": "reader"},
	}, st)
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths), appbuildtest.WithStore(st),
		appbuildtest.WithDeclarative(decl))
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	setTestACL(t, app, decl)
	app.schema.Publish(&Schema{Cfg: &Config{Actions: map[string]dataentryconfig.Action{"triage": fx.action}}, Meta: meta})
	if err := app.SetViewConditions(AdaptViewConditions(appbuild.ViewConditions)); err != nil {
		t.Fatal(err)
	}
	chat := &fakeChat{reply: fx.reply}
	base := app.write.luaDeps
	app.write.luaDeps = func() lua.WriteDeps {
		d := base()
		d.Seams.AI = chat
		return d
	}
	return llmApp{app: app, decl: decl, store: st, chat: chat}
}

func (a llmApp) run(t *testing.T, user string) (status int, resp v1.ActionResponse, body string) {
	t.Helper()
	rec := postAction(principalCtx(user), t, a.app, a.decl, "triage", `{"entity_id":"TKT-1"}`)
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v: %s", err, rec.Body)
		}
	}
	return rec.Code, resp, rec.Body.String()
}

// unchanged asserts the run wrote nothing: TKT-1 keeps its priority and has
// only its seeded relation.
func (a llmApp) unchanged(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	e, err := a.store.GetEntity(ctx, entity.Ref{ID: "TKT-1"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Properties["priority"] != "low" {
		t.Errorf("priority written: %v", e.Properties["priority"])
	}
	n := 0
	for _, err := range a.store.ListRelations(ctx, store.RelationQuery{EntityID: "TKT-1", Direction: store.DirectionOutgoing}) {
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n != 1 {
		t.Errorf("TKT-1 has %d outgoing relations, want the 1 seeded", n)
	}
}

func TestLLMAction_ProposesWithoutWriting(t *testing.T) {
	a := newLLMApp(t, `{"set":{"priority":"high"},"relate":{"affects":["CON-1","CON-3"]}}`)
	code, resp, body := a.run(t, "alice")
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	p := resp.Proposal
	if p == nil {
		t.Fatalf("no proposal: %s", body)
	}
	if p.Entity != "TKT-1" || p.Model != "fake-1" {
		t.Errorf("entity/model = %q/%q", p.Entity, p.Model)
	}
	if len(p.Set) != 1 || p.Set[0].Property != "priority" || p.Set[0].Current != "low" || p.Set[0].Proposed != "high" {
		t.Errorf("set = %+v", p.Set)
	}
	// CON-3 is already related, so only CON-1 is proposed.
	if len(p.Relate) != 1 || p.Relate[0].To != "CON-1" || p.Relate[0].ToTitle != "Sessions" {
		t.Errorf("relate = %+v", p.Relate)
	}
	sent := a.chat.sent(t)
	if !strings.Contains(sent, "CON-1") || strings.Contains(sent, "CON-2") {
		t.Errorf("candidates not filtered by the condition:\n%s", sent)
	}
	if !strings.Contains(sent, "Users cannot log in.") {
		t.Errorf("content not sent:\n%s", sent)
	}
	a.unchanged(t)
}

func TestLLMAction_HiddenFieldsAreNotSent(t *testing.T) {
	a := newLLMApp(t, `{"set":{"priority":null},"relate":{"affects":[]}}`)
	setTestResolver(t, a.app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"secret": false, "area": false}}})
	code, resp, body := a.run(t, "alice")
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	sent := a.chat.sent(t)
	if strings.Contains(sent, "S3CRET") {
		t.Errorf("hidden subject field sent to the model:\n%s", sent)
	}
	// The candidate condition reads the hidden area, so no candidate passes.
	for _, id := range []string{"CON-1", "CON-2", "CON-3"} {
		if strings.Contains(sent, id) {
			t.Errorf("candidate %s judged on a hidden field:\n%s", id, sent)
		}
	}
	if len(resp.Proposal.Set) != 0 || len(resp.Proposal.Relate) != 0 {
		t.Errorf("proposal = %+v", resp.Proposal)
	}
	a.unchanged(t)
}

func TestLLMAction_Refusals(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		reply    string
		chatErr  error
		resolver *fakeResolver
		noAI     bool
		want     int
		wantCode string
	}{
		{name: "permission missing", user: "bob", want: http.StatusForbidden, wantCode: "permission_required"},
		{
			name: "invalid enum value", user: "alice", reply: `{"set":{"priority":"urgent"},"relate":{"affects":[]}}`,
			want: http.StatusBadGateway, wantCode: "ai_reply_invalid",
		},
		{
			name: "candidate not offered", user: "alice", reply: `{"set":{"priority":null},"relate":{"affects":["CON-2"]}}`,
			want: http.StatusBadGateway, wantCode: "ai_reply_invalid",
		},
		{
			name: "provider error", user: "alice", chatErr: &ai.Error{Kind: ai.ErrTimeout, Message: "secret prompt echo"},
			want: http.StatusBadGateway, wantCode: "ai_failed",
		},
		{name: "no ai config", user: "alice", noAI: true, want: http.StatusServiceUnavailable, wantCode: "ai_not_configured"},
		{
			name: "nothing to propose", user: "alice",
			resolver: &fakeResolver{
				fv: FieldVerdicts{Writable: map[string]bool{"priority": false}},
				rv: RelationVerdicts{Types: map[string]RelationVerdict{"affects": {}}},
			},
			want: http.StatusForbidden, wantCode: "nothing_to_propose",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := newLLMApp(t, tc.reply)
			a.chat.err = tc.chatErr
			if tc.noAI {
				base := a.app.write.luaDeps
				a.app.write.luaDeps = func() lua.WriteDeps {
					d := base()
					d.Seams.AI = nil
					return d
				}
			}
			if tc.resolver != nil {
				setTestResolver(t, a.app, *tc.resolver)
			}
			code, _, body := a.run(t, tc.user)
			if code != tc.want || !strings.Contains(body, tc.wantCode) {
				t.Fatalf("got %d %s, want %d %s", code, body, tc.want, tc.wantCode)
			}
			if strings.Contains(body, "secret prompt echo") {
				t.Errorf("provider message leaked to the caller: %s", body)
			}
			a.unchanged(t)
		})
	}
}

// llmGraphMeta has a ticket that blocks other tickets, and a relation whose
// `to` names two types.
func llmGraphMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(`
entities:
  ticket:
    label: Ticket
    display_property: title
    id_prefix: TKT
    properties:
      title: { type: string }
      priority: { type: enum, values: [low, high] }
  concept:
    label: Concept
    display_property: title
    id_prefix: CON
    properties:
      title: { type: string }
      area: { type: string }
      note: { type: string }
  component:
    label: Component
    display_property: title
    id_prefix: CMP
    properties:
      title: { type: string }
      area: { type: string }
relations:
  affects:
    from: [ticket]
    to: [concept]
  blocks:
    from: [ticket]
    to: [ticket]
  touches:
    from: [ticket]
    to: [concept, component]
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	return m
}

// llmGraphSeed: TKT-1 blocks TKT-2, TKT-3 blocks TKT-1, TKT-1 affects
// CON-1..3 and touches CON-1 and CMP-1.
func llmGraphSeed() ([]*entity.Entity, []entity.RelationKey) {
	ents := []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "Login broken", "priority": "low"}},
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "Blocked one", "priority": "low"}},
		{ID: "TKT-3", Type: "ticket", Properties: map[string]any{"title": "Blocker", "priority": "high"}},
		{ID: "CON-1", Type: "concept", Properties: map[string]any{"title": "Sessions", "area": "auth", "note": "N1"}},
		{ID: "CON-2", Type: "concept", Properties: map[string]any{"title": "Invoices", "area": "billing", "note": "N2"}},
		{ID: "CON-3", Type: "concept", Properties: map[string]any{"title": "Passwords", "area": "auth", "note": "N3"}},
		{ID: "CMP-1", Type: "component", Properties: map[string]any{"title": "Gateway", "area": "auth"}},
	}
	edges := []entity.RelationKey{
		{From: "TKT-1", Type: "blocks", To: "TKT-2"},
		{From: "TKT-3", Type: "blocks", To: "TKT-1"},
		{From: "TKT-1", Type: "affects", To: "CON-1"},
		{From: "TKT-1", Type: "affects", To: "CON-2"},
		{From: "TKT-1", Type: "affects", To: "CON-3"},
		{From: "TKT-1", Type: "touches", To: "CON-1"},
		{From: "TKT-1", Type: "touches", To: "CMP-1"},
	}
	return ents, edges
}

// llmAction is a "triage" action on tickets with the given inputs and
// outputs.
func llmAction(related []dataentryconfig.LLMRelatedInput, set []string,
	relate []dataentryconfig.LLMRelateOutput,
) dataentryconfig.Action {
	return dataentryconfig.Action{
		Label:       "Triage",
		Permission:  "ticket.triage",
		AvailableOn: &dataentryconfig.ActionScope{EntityTypes: []string{"ticket"}},
		LLM: &dataentryconfig.LLMAction{
			Prompt:  "Triage the ticket.",
			Inputs:  dataentryconfig.LLMInputs{Properties: []string{"title"}, Related: related},
			Outputs: dataentryconfig.LLMOutputs{Set: set, Relate: relate},
		},
	}
}

// llmItemDoc is an entity in the document sent to the model.
type llmItemDoc struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	Title          string         `json:"title"`
	Properties     map[string]any `json:"properties"`
	AlreadyRelated bool           `json:"already_related"`
}

// llmDoc is the JSON user message sent to the model.
type llmDoc struct {
	Record  map[string]any `json:"record"`
	Related []struct {
		Relation  string       `json:"relation"`
		Direction string       `json:"direction"`
		Entities  []llmItemDoc `json:"entities"`
	} `json:"related"`
	Candidates map[string]struct {
		Entities  []llmItemDoc `json:"entities"`
		Truncated bool         `json:"truncated"`
	} `json:"candidates"`
}

// doc decodes the user message of the one request the provider received.
func (f *fakeChat) doc(t *testing.T) llmDoc {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.got) != 1 {
		t.Fatalf("provider called %d times, want 1", len(f.got))
	}
	for _, m := range f.got[0].Messages {
		if m.Role != "user" {
			continue
		}
		var d llmDoc
		if err := json.Unmarshal([]byte(m.Content), &d); err != nil {
			t.Fatalf("decode user message: %v: %s", err, m.Content)
		}
		return d
	}
	t.Fatal("request has no user message")
	return llmDoc{}
}

func (f *fakeChat) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.got)
}

func itemIDs(items []llmItemDoc) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func TestLLMAction_RelatedInputs(t *testing.T) {
	tests := []struct {
		name     string
		input    dataentryconfig.LLMRelatedInput
		read     []string
		resolver *fakeResolver
		want     []string
		absent   []string
		check    func(t *testing.T, items []llmItemDoc)
	}{
		{
			name:  "outgoing picks the targets",
			input: dataentryconfig.LLMRelatedInput{Relation: "blocks", Fields: []string{"priority"}},
			want:  []string{"TKT-2"},
		},
		{
			name:  "incoming picks the sources",
			input: dataentryconfig.LLMRelatedInput{Relation: "blocks", Direction: "incoming", Fields: []string{"priority"}},
			want:  []string{"TKT-3"},
			check: func(t *testing.T, items []llmItemDoc) {
				t.Helper()
				if items[0].Properties["priority"] != "high" {
					t.Errorf("TKT-3 fields = %v", items[0].Properties)
				}
			},
		},
		{
			name:  "limit caps the count",
			input: dataentryconfig.LLMRelatedInput{Relation: "affects", Limit: 2},
			want:  []string{"CON-1", "CON-2"},
		},
		{
			name:   "peer hidden by ACL is absent",
			input:  dataentryconfig.LLMRelatedInput{Relation: "touches", Fields: []string{"area"}},
			read:   []string{"ticket", "concept"},
			want:   []string{"CON-1"},
			absent: []string{"CMP-1", "Gateway"},
		},
		{
			name:     "field hidden by visible is absent",
			input:    dataentryconfig.LLMRelatedInput{Relation: "affects", Fields: []string{"area", "note"}, Limit: 1},
			resolver: &fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"note": false}}},
			want:     []string{"CON-1"},
			absent:   []string{"N1"},
			check: func(t *testing.T, items []llmItemDoc) {
				t.Helper()
				if _, ok := items[0].Properties["note"]; ok || items[0].Properties["area"] != "auth" {
					t.Errorf("CON-1 fields = %v, want area only", items[0].Properties)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ents, edges := llmGraphSeed()
			read := tc.read
			if read == nil {
				read = []string{"ticket", "concept", "component"}
			}
			a := buildLLMApp(t, llmFixture{
				meta: llmGraphMeta(t), seed: ents, edges: edges, read: read,
				action: llmAction([]dataentryconfig.LLMRelatedInput{tc.input}, []string{"priority"}, nil),
				reply:  `{"set":{"priority":null},"relate":{}}`,
			})
			if tc.resolver != nil {
				setTestResolver(t, a.app, *tc.resolver)
			}
			code, _, body := a.run(t, "alice")
			if code != http.StatusOK {
				t.Fatalf("status %d: %s", code, body)
			}
			d := a.chat.doc(t)
			if len(d.Related) != 1 {
				t.Fatalf("related = %+v", d.Related)
			}
			items := d.Related[0].Entities
			if got := itemIDs(items); !slices.Equal(got, tc.want) {
				t.Errorf("related ids = %v, want %v", got, tc.want)
			}
			sent := a.chat.sent(t)
			for _, s := range tc.absent {
				if strings.Contains(sent, s) {
					t.Errorf("%q sent to the model:\n%s", s, sent)
				}
			}
			if tc.check != nil && len(items) > 0 {
				tc.check(t, items)
			}
		})
	}
}

func TestLLMAction_Candidates(t *testing.T) {
	tests := []struct {
		name          string
		relate        dataentryconfig.LLMRelateOutput
		want          []string
		wantTruncated bool
	}{
		{
			name:          "limit truncates",
			relate:        dataentryconfig.LLMRelateOutput{Relation: "affects", Candidates: "entity.area == 'auth'", Limit: 1},
			want:          []string{"CON-1"},
			wantTruncated: true,
		},
		{
			name:   "every to type is offered",
			relate: dataentryconfig.LLMRelateOutput{Relation: "touches", Candidates: "entity.area == 'auth'"},
			want:   []string{"CON-1", "CON-3", "CMP-1"},
		},
		{
			name:   "subject is never a candidate",
			relate: dataentryconfig.LLMRelateOutput{Relation: "blocks"},
			want:   []string{"TKT-2", "TKT-3"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ents, edges := llmGraphSeed()
			a := buildLLMApp(t, llmFixture{
				meta: llmGraphMeta(t), seed: ents, edges: edges, read: []string{"ticket", "concept", "component"},
				action: llmAction(nil, []string{"priority"}, []dataentryconfig.LLMRelateOutput{tc.relate}),
				reply:  `{"set":{"priority":null},"relate":{"` + tc.relate.Relation + `":[]}}`,
			})
			code, _, body := a.run(t, "alice")
			if code != http.StatusOK {
				t.Fatalf("status %d: %s", code, body)
			}
			c, ok := a.chat.doc(t).Candidates[tc.relate.Relation]
			if !ok {
				t.Fatalf("no candidate list for %s", tc.relate.Relation)
			}
			got := itemIDs(c.Entities)
			if len(got) != len(tc.want) {
				t.Errorf("candidates = %v, want %v", got, tc.want)
			}
			for _, id := range tc.want {
				if !slices.Contains(got, id) {
					t.Errorf("candidates = %v, missing %s", got, id)
				}
			}
			if slices.Contains(got, "TKT-1") {
				t.Errorf("subject offered as a candidate: %v", got)
			}
			if c.Truncated != tc.wantTruncated {
				t.Errorf("truncated = %v, want %v", c.Truncated, tc.wantTruncated)
			}
		})
	}
}

// A target the caller cannot read is neither sent to the model nor
// accepted when the model names it anyway.
func TestLLMAction_HiddenCandidate(t *testing.T) {
	relate := dataentryconfig.LLMRelateOutput{Relation: "touches", Candidates: "entity.area == 'auth'"}
	tests := []struct {
		name, reply string
		wantStatus  int
	}{
		{name: "not offered", reply: `{"set":{"priority":null},"relate":{"touches":[]}}`, wantStatus: http.StatusOK},
		{name: "refused when named", reply: `{"set":{"priority":null},"relate":{"touches":["CMP-1"]}}`, wantStatus: http.StatusBadGateway},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ents, edges := llmGraphSeed()
			a := buildLLMApp(t, llmFixture{
				meta: llmGraphMeta(t), seed: ents, edges: edges, read: []string{"ticket", "concept"},
				action: llmAction(nil, []string{"priority"}, []dataentryconfig.LLMRelateOutput{relate}),
				reply:  tc.reply,
			})
			code, _, body := a.run(t, "alice")
			if code != tc.wantStatus {
				t.Fatalf("status %d, want %d: %s", code, tc.wantStatus, body)
			}
			if tc.wantStatus == http.StatusBadGateway && !strings.Contains(body, "ai_reply_invalid") {
				t.Errorf("body = %s, want ai_reply_invalid", body)
			}
			sent := a.chat.sent(t)
			if strings.Contains(sent, "CMP-1") || strings.Contains(sent, "Gateway") {
				t.Errorf("hidden candidate sent to the model: %s", sent)
			}
		})
	}
}

func TestLLMAction_NoCandidatesSkipsProvider(t *testing.T) {
	tests := []struct {
		name     string
		set      []string
		resolver *fakeResolver
	}{
		{name: "no set output", set: nil},
		{
			name: "set output not writable", set: []string{"priority"},
			resolver: &fakeResolver{fv: FieldVerdicts{Writable: map[string]bool{"priority": false}}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ents, edges := llmGraphSeed()
			a := buildLLMApp(t, llmFixture{
				meta: llmGraphMeta(t), seed: ents, edges: edges, read: []string{"ticket", "concept", "component"},
				action: llmAction(nil, tc.set, []dataentryconfig.LLMRelateOutput{{
					Relation: "affects", Candidates: "entity.area == 'nowhere'",
				}}),
				reply: `{"set":{},"relate":{"affects":[]}}`,
			})
			if tc.resolver != nil {
				setTestResolver(t, a.app, *tc.resolver)
			}
			code, resp, body := a.run(t, "alice")
			if code != http.StatusOK {
				t.Fatalf("status %d: %s", code, body)
			}
			if n := a.chat.calls(); n != 0 {
				t.Errorf("provider called %d times, want 0", n)
			}
			p := resp.Proposal
			if p == nil || p.Entity != "TKT-1" || len(p.Set) != 0 || len(p.Relate) != 0 {
				t.Errorf("proposal = %+v, want an empty one for TKT-1", p)
			}
		})
	}
}

// TestLLMAction_CandidateReadBudget pins that one run reads the store the
// same number of times whatever the number of candidate rows
// (.claude/rules/collection-reads.md).
func TestLLMAction_CandidateReadBudget(t *testing.T) {
	reads := func(t *testing.T, n int) int {
		t.Helper()
		ents := []*entity.Entity{
			{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "Login broken", "priority": "low"}},
			{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "Blocked one", "priority": "low"}},
		}
		for i := range n {
			ents = append(ents, &entity.Entity{ID: fmt.Sprintf("CON-%d", i+1), Type: "concept",
				Properties: map[string]any{"title": fmt.Sprintf("Concept %d", i+1), "area": "auth", "note": "n"}})
		}
		var counting *storetest.Counting
		a := buildLLMApp(t, llmFixture{
			meta: llmGraphMeta(t), seed: ents,
			edges: []entity.RelationKey{
				{From: "TKT-1", Type: "blocks", To: "TKT-2"},
				{From: "TKT-1", Type: "affects", To: "CON-1"},
			},
			read: []string{"ticket", "concept", "component"},
			action: llmAction(
				[]dataentryconfig.LLMRelatedInput{{Relation: "blocks"}},
				[]string{"priority"},
				[]dataentryconfig.LLMRelateOutput{{
					Relation: "affects", Candidates: "entity.area == 'auth'", Fields: []string{"area", "note"},
				}},
			),
			reply: `{"set":{"priority":null},"relate":{"affects":[]}}`,
			wrap: func(s store.Store) store.Store {
				counting = storetest.NewCounting(s)
				return counting
			},
		})
		counting.Reset()
		code, _, body := a.run(t, "alice")
		if code != http.StatusOK {
			t.Fatalf("status %d: %s", code, body)
		}
		if got := len(a.chat.doc(t).Candidates["affects"].Entities); got != n {
			t.Fatalf("%d candidates sent, want %d", got, n)
		}
		return counting.Reads()
	}
	r10, r50 := reads(t, 10), reads(t, 50)
	if r10 == 0 {
		t.Fatal("no store reads counted: the run does not read through the counting store")
	}
	t.Logf("store reads per run: %d at 10, %d at 50", r10, r50)
	if r10 != r50 {
		t.Errorf("store reads grow with candidates: %d at 10, %d at 50", r10, r50)
	}
}

func TestLLMAction_ReplyInvalid(t *testing.T) {
	tests := []struct {
		name   string
		reply  string
		finish string
	}{
		{name: "cut off by length", reply: `{"set":{"priority":"high"},"relate":{"affects":[]}}`, finish: "length"},
		{name: "unknown candidate id", reply: `{"set":{"priority":null},"relate":{"affects":["CON-999"]}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := newLLMApp(t, tc.reply)
			a.chat.finish = tc.finish
			code, _, body := a.run(t, "alice")
			if code != http.StatusBadGateway || !strings.Contains(body, "ai_reply_invalid") {
				t.Fatalf("got %d %s, want 502 ai_reply_invalid", code, body)
			}
			a.unchanged(t)
		})
	}
}
