package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/actionproposal"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// An `llm:` action with `apply: auto` (TKT-WQ925LL): the route queues it,
// and the background runner proposes and applies as the queued user.

type fakeLLMQueue struct {
	mu   sync.Mutex
	jobs [][3]string
}

func (q *fakeLLMQueue) EnqueueLLMAction(_ context.Context, action, address, _ string, who principal.Principal) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, [3]string{action, address, who.User})
	return nil
}

func autoTriage() dataentryconfig.Action {
	a := triageAction()
	a.LLM.Apply = dataentryconfig.LLMApplyAuto
	return a
}

func newAutoLLMApp(t *testing.T, reply string, create []string, action dataentryconfig.Action) llmApp {
	t.Helper()
	return newAutoLLMAppWith(t, reply, create, action, nil)
}

// newAutoLLMAppWith is newAutoLLMApp with policy editing the ACL policy.
func newAutoLLMAppWith(
	t *testing.T, reply string, create []string, action dataentryconfig.Action, policy func(*acl.Policy),
) llmApp {
	t.Helper()
	// A relation create needs the create grant on its source type.
	create = append([]string{"ticket"}, create...)
	return buildLLMApp(t, llmFixture{
		meta: llmMeta(t),
		seed: []*entity.Entity{
			{ID: "TKT-1", Type: "ticket", Content: "Users cannot log in.",
				Properties: map[string]any{"title": "Login broken", "priority": "low", "secret": "S3CRET"}},
			{ID: "CON-1", Type: "concept", Properties: map[string]any{"title": "Sessions", "area": "auth"}},
			{ID: "CON-3", Type: "concept", Properties: map[string]any{"title": "Billing", "area": "billing"}},
		},
		edges: []entity.RelationKey{{From: "TKT-1", Type: "affects", To: "CON-3"}},
		read:  []string{"ticket", "concept"}, action: action, reply: reply, create: create, policy: policy,
	})
}

// bound is ctx for user with its ACL request bound, as the job handler
// binds it.
func (a llmApp) bound(t *testing.T, user string) context.Context {
	t.Helper()
	ctx, err := acl.BindRequest(principal.With(context.Background(),
		principal.Principal{User: user, Tool: principal.ToolLLMAction}), a.decl)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func TestLLMAction_AutoQueues(t *testing.T) {
	a := newAutoLLMApp(t, `{}`, nil, autoTriage())

	code, _, body := a.run(t, "alice")
	if code != http.StatusServiceUnavailable || !strings.Contains(body, "background_unavailable") {
		t.Fatalf("without a queue: status %d: %s", code, body)
	}

	q := &fakeLLMQueue{}
	if err := SetLLMActionQueue(a.app, q); err != nil {
		t.Fatal(err)
	}
	code, resp, body := a.run(t, "alice")
	if code != http.StatusOK || resp.Proposal != nil || resp.Message == "" {
		t.Fatalf("status %d: %s", code, body)
	}
	if len(q.jobs) != 1 || q.jobs[0] != [3]string{"triage", "TKT-1", "alice"} {
		t.Errorf("jobs = %v", q.jobs)
	}
	if n := a.chat.calls(); n != 0 {
		t.Errorf("the route asked the model %d times", n)
	}

	// The gates still run before anything is queued.
	code, _, _ = a.run(t, "bob")
	if code != http.StatusForbidden || len(q.jobs) != 1 {
		t.Errorf("bob: status %d, jobs %v", code, q.jobs)
	}
	a.unchanged(t)
}

func TestLLMActionRunner_Applies(t *testing.T) {
	a := newAutoLLMApp(t, `{"set":{"priority":"high"},"relate":{"affects":["CON-1"]}}`, nil, autoTriage())
	r, err := NewLLMActionRunner(a.app)
	if err != nil {
		t.Fatal(err)
	}
	if retry, runErr := r.RunLLMAction(a.bound(t, "alice"), "triage", "TKT-1", ""); runErr != nil {
		t.Fatalf("run: retry=%v: %v", retry, runErr)
	}
	ctx := context.Background()
	e, err := a.store.GetEntity(ctx, entity.Ref{ID: "TKT-1"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Properties["priority"] != "high" {
		t.Errorf("priority = %v", e.Properties["priority"])
	}
	if _, err := a.store.GetRelation(ctx, entity.RelationKey{From: "TKT-1", Type: "affects", To: "CON-1"}); err != nil {
		t.Errorf("relation not created: %v", err)
	}
}

func TestLLMActionRunner_Refusals(t *testing.T) {
	a := newAutoLLMApp(t, `{"set":{"priority":"high"},"relate":{"affects":[]}}`, nil, autoTriage())
	r, err := NewLLMActionRunner(a.app)
	if err != nil {
		t.Fatal(err)
	}
	// bob lacks the permission: final, and nothing is asked or written.
	if retry, err := r.RunLLMAction(a.bound(t, "bob"), "triage", "TKT-1", ""); err == nil || retry {
		t.Errorf("bob: retry=%v err=%v", retry, err)
	}
	if retry, err := r.RunLLMAction(a.bound(t, "alice"), "nope", "TKT-1", ""); err == nil || retry {
		t.Errorf("unknown action: retry=%v err=%v", retry, err)
	}
	// Nothing bound: refused before any read.
	if _, err := r.RunLLMAction(context.Background(), "triage", "TKT-1", ""); err == nil {
		t.Error("unbound run succeeded")
	}
	if retry, err := r.RunLLMAction(a.bound(t, "alice"), "triage", "TKT-1", "nowhere"); err == nil || retry {
		t.Errorf("unknown world: retry %v err %v, want a refusal that is not retried", retry, err)
	}
	if n := a.chat.calls(); n != 0 {
		t.Errorf("the model was asked %d times", n)
	}
	a.unchanged(t)
}

func TestLLMActionRunner_ProposingActionRefused(t *testing.T) {
	a := newAutoLLMApp(t, `{}`, nil, triageAction())
	r, err := NewLLMActionRunner(a.app)
	if err != nil {
		t.Fatal(err)
	}
	if retry, err := r.RunLLMAction(a.bound(t, "alice"), "triage", "TKT-1", ""); err == nil || retry {
		t.Errorf("retry=%v err=%v", retry, err)
	}
}

func TestLLMActionRunner_Creates(t *testing.T) {
	action := autoTriage()
	action.LLM.Outputs = dataentryconfig.LLMOutputs{Create: []dataentryconfig.LLMCreateOutput{{
		Type: "concept", Relation: "affects", Properties: []string{"title", "area"}, Max: 2,
	}}}
	a := newAutoLLMApp(t, `{"set":{},"relate":{},"create":{"affects":[{"title":"Tokens","area":null}]}}`,
		[]string{"concept"}, action)
	r, err := NewLLMActionRunner(a.app)
	if err != nil {
		t.Fatal(err)
	}
	if retry, runErr := r.RunLLMAction(a.bound(t, "alice"), "triage", "TKT-1", ""); runErr != nil {
		t.Fatalf("run: retry=%v: %v", retry, runErr)
	}
	ctx := context.Background()
	var created []string
	for rel, err := range a.store.ListRelations(ctx, store.RelationQuery{EntityID: "TKT-1", Direction: store.DirectionOutgoing}) {
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, rel.To)
	}
	var fresh []string
	for _, id := range created {
		if id != "CON-3" {
			fresh = append(fresh, id)
		}
	}
	if len(fresh) != 1 || fresh[0] == "CON-1" {
		t.Fatalf("outgoing = %v, want CON-3 and one new concept", created)
	}
	e, err := a.store.GetEntity(ctx, entity.Ref{ID: fresh[0]})
	if err != nil {
		t.Fatal(err)
	}
	if e.Properties["title"] != "Tokens" {
		t.Errorf("new concept = %+v", e.Properties)
	}
}

func TestApplyProposal_SkipsStale(t *testing.T) {
	a := newAutoLLMApp(t, `{}`, nil, autoTriage())
	ctx, err := bindBackgroundRequest(a.bound(t, "alice"), a.app, "")
	if err != nil {
		t.Fatal(err)
	}
	p := &actionproposal.Proposal{Entity: "TKT-1", Set: []actionproposal.Value{
		{Property: "priority", Current: json.RawMessage(`"high"`), Proposed: json.RawMessage(`"low"`)},
		{Property: "title", Current: json.RawMessage(`"Login broken"`), Proposed: json.RawMessage(`"Login fails"`)},
	}}
	got, err := actionOps{h: a.app.write}.applyProposal(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.stale) != 1 || got.stale[0] != "priority" || len(got.set) != 1 {
		t.Errorf("applied = %+v", got)
	}
	if msg := got.message(); !strings.Contains(msg, "Skipped priority") {
		t.Errorf("message = %q", msg)
	}
	e, err := a.store.GetEntity(context.Background(), entity.Ref{ID: "TKT-1"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Properties["title"] != "Login fails" || e.Properties["priority"] != "low" {
		t.Errorf("properties = %+v", e.Properties)
	}
}

func TestRetryableStatus(t *testing.T) {
	for status, want := range map[int]bool{
		http.StatusConflict: true, http.StatusBadGateway: true,
		http.StatusServiceUnavailable: true, http.StatusGatewayTimeout: true,
		http.StatusUnprocessableEntity: false, http.StatusForbidden: false,
		http.StatusNotFound: false, http.StatusBadRequest: false,
	} {
		if got := retryableStatus(status); got != want {
			t.Errorf("retryableStatus(%d) = %v, want %v", status, got, want)
		}
	}
}

// A relation made while the model was answering is skipped, not a failure.
func TestApplyProposal_ExistingRelationSkipped(t *testing.T) {
	a := newAutoLLMApp(t, `{}`, nil, autoTriage())
	ctx, err := bindBackgroundRequest(a.bound(t, "alice"), a.app, "")
	if err != nil {
		t.Fatal(err)
	}
	p := &actionproposal.Proposal{Entity: "TKT-1", Relate: []actionproposal.Relation{
		{Relation: "affects", To: "CON-3"}, {Relation: "affects", To: "CON-1"},
	}}
	got, err := actionOps{h: a.app.write}.applyProposal(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if got.related != 1 {
		t.Errorf("related = %d, want 1 (CON-3 already there)", got.related)
	}
}

// boundAs is ctx for p with its ACL request bound, as the job handler binds
// it.
func (a llmApp) boundAs(t *testing.T, p principal.Principal) context.Context {
	t.Helper()
	ctx, err := acl.BindRequest(principal.With(context.Background(), p), a.decl)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

// A background run keeps the queuing client's ceiling: a token without the
// write scope cannot apply, through the real ACL, and one with it can.
func TestLLMActionRunner_ClientCeiling(t *testing.T) {
	ceiling := func(p *acl.Policy) {
		p.ClientBaselines = map[string]acl.ClientBaseline{"apps": {
			AppliesTo: []string{"app"},
			Restriction: acl.Restriction{
				Read: []string{"ticket", "concept"}, Update: []string{}, Permissions: []string{"ticket.triage"},
			},
		}}
		p.ScopeGrants = map[string]acl.ScopeGrant{"write": {Restriction: acl.Restriction{Update: []string{"ticket"}}}}
	}
	client := func(scopes ...string) principal.Principal {
		return principal.VerifiedFrom("alice", principal.ToolLLMAction,
			principal.Claims{PrincipalType: "app", Scopes: scopes})
	}
	reply := `{"set":{"priority":"high"},"relate":{"affects":[]}}`

	a := newAutoLLMAppWith(t, reply, nil, autoTriage(), ceiling)
	r, err := NewLLMActionRunner(a.app)
	if err != nil {
		t.Fatal(err)
	}
	// priority is not writable under the token, so it is not offered to
	// the model, and a reply that proposes it anyway is refused.
	if _, runErr := r.RunLLMAction(a.boundAs(t, client("read")), "triage", "TKT-1", ""); runErr == nil {
		t.Error("read-only token: applied")
	}
	a.unchanged(t)

	a = newAutoLLMAppWith(t, reply, nil, autoTriage(), ceiling)
	if r, err = NewLLMActionRunner(a.app); err != nil {
		t.Fatal(err)
	}
	if _, runErr := r.RunLLMAction(a.boundAs(t, client("read", "write")), "triage", "TKT-1", ""); runErr != nil {
		t.Fatalf("token with the write scope: %v", runErr)
	}
	e, err := a.store.GetEntity(context.Background(), entity.Ref{ID: "TKT-1"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Properties["priority"] != "high" {
		t.Errorf("priority = %v", e.Properties["priority"])
	}
}

// A user who holds the action but may not write the proposed field writes
// nothing.
func TestLLMActionRunner_FieldDenied(t *testing.T) {
	titleOnly := func(p *acl.Policy) {
		role := p.Roles["triager"]
		role.Fields = map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}}
		p.Roles["triager"] = role
	}
	a := newAutoLLMAppWith(t, `{"set":{"priority":"high"},"relate":{"affects":[]}}`, nil, autoTriage(), titleOnly)
	resolver, err := affordances.New(llmMeta(t), storeRelationLookup{st: a.store}, a.decl)
	if err != nil {
		t.Fatal(err)
	}
	setTestResolver(t, a.app, &policyResolver{inner: resolver})
	r, err := NewLLMActionRunner(a.app)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.RunLLMAction(a.bound(t, "alice"), "triage", "TKT-1", ""); err == nil {
		t.Error("applied a field the user may not write")
	}
	a.unchanged(t)
}

// A declared world the user holds no grant on is refused, not read in the
// default world instead.
func TestLLMActionRunner_WorldWithoutGrant(t *testing.T) {
	a := newAutoLLMApp(t, `{"set":{"priority":"high"},"relate":{"affects":[]}}`, nil, autoTriage())
	a.app.worlds = stubWorlds{names: map[string]bool{"review": true}, resolveDefault: true}
	r, err := NewLLMActionRunner(a.app)
	if err != nil {
		t.Fatal(err)
	}
	if retry, err := r.RunLLMAction(a.bound(t, "alice"), "triage", "TKT-1", "review"); err == nil || retry {
		t.Errorf("retry %v err %v, want a refusal that is not retried", retry, err)
	}
	if n := a.chat.calls(); n != 0 {
		t.Errorf("the model was asked %d times", n)
	}
	a.unchanged(t)
}
