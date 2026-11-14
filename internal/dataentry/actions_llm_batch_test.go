package dataentry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/actionproposal"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Batch runs of LLM actions (TKT-I2SRNWR): one model call for several
// entities, each gated as the detail action is.

func batchTriageAction(max int) dataentryconfig.Action {
	return dataentryconfig.Action{
		Label:       "Triage",
		Permission:  "ticket.triage",
		AvailableOn: &dataentryconfig.ActionScope{EntityTypes: []string{"ticket"}},
		When:        "entity.priority == 'low'",
		LLM: &dataentryconfig.LLMAction{
			Prompt: "Triage the tickets.",
			Inputs: dataentryconfig.LLMInputs{
				Properties: []string{"title", "secret"},
				Content:    &dataentryconfig.LLMContentInput{},
			},
			Outputs: dataentryconfig.LLMOutputs{Set: []string{"priority"}},
			Batch:   &dataentryconfig.LLMBatch{Max: max},
		},
	}
}

func batchTickets(n int) []*entity.Entity {
	out := make([]*entity.Entity, 0, n)
	for i := range n {
		out = append(out, &entity.Entity{
			ID: fmt.Sprintf("TKT-%d", i+1), Type: "ticket", Content: fmt.Sprintf("Body %d", i+1),
			Properties: map[string]any{"title": fmt.Sprintf("Ticket %d", i+1), "priority": "low", "secret": "S3CRET"},
		})
	}
	return out
}

func newBatchApp(t *testing.T, reply string) llmApp {
	t.Helper()
	seed := batchTickets(2)
	seed = append(seed, &entity.Entity{ID: "TKT-3", Type: "ticket",
		Properties: map[string]any{"title": "Already high", "priority": "high"}},
		&entity.Entity{ID: "CON-1", Type: "concept", Properties: map[string]any{"title": "Sessions"}})
	// The edge is what llmApp.unchanged expects to find.
	return buildLLMApp(t, llmFixture{
		meta: llmMeta(t), seed: seed, read: []string{"ticket", "concept"},
		edges:  []entity.RelationKey{{From: "TKT-1", Type: "affects", To: "CON-1"}},
		action: batchTriageAction(5), reply: reply,
	})
}

func (a llmApp) runBatch(t *testing.T, user, body string) (int, *actionproposal.Batch, string) {
	t.Helper()
	rec := postAction(principalCtx(user), t, a.app, a.decl, "triage", body)
	var resp v1.ActionResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v: %s", err, rec.Body)
		}
	}
	return rec.Code, resp.Batch, rec.Body.String()
}

func (f *fakeChat) userDoc(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.got) != 1 {
		t.Fatalf("provider called %d times, want 1", len(f.got))
	}
	return f.got[0].Messages[1].Content
}

func TestLLMBatch_ProposesPerRowWithoutWriting(t *testing.T) {
	a := newBatchApp(t, `{"records":{"TKT-1":{"set":{"priority":"high"}},"TKT-2":{"set":{"priority":"urgent"}}}}`)
	// TKT-3 fails when; TKT-404 does not exist; TKT-1 twice runs once.
	code, b, body := a.runBatch(t, "alice", `{"entity_ids":["TKT-1","TKT-2","TKT-3","TKT-404","TKT-1"]}`)
	if code != http.StatusOK || b == nil {
		t.Fatalf("status %d: %s", code, body)
	}
	if len(b.Proposals) != 1 || b.Proposals[0].Entity != "TKT-1" || b.Proposals[0].Title != "Ticket 1" {
		t.Fatalf("proposals = %+v", b.Proposals)
	}
	if p := b.Proposals[0]; len(p.Set) != 1 || string(p.Set[0].Proposed) != `"high"` {
		t.Errorf("TKT-1 set = %+v", p.Set)
	}
	if len(b.Failed) != 1 || b.Failed[0].Entity != "TKT-2" || !strings.Contains(b.Failed[0].Reason, "urgent") {
		t.Errorf("failed = %+v", b.Failed)
	}
	if b.Skipped != 2 {
		t.Errorf("skipped = %d, want 2", b.Skipped)
	}
	doc := a.chat.userDoc(t)
	if strings.Contains(doc, "TKT-3") || strings.Contains(doc, "TKT-404") {
		t.Errorf("a gated-out row reached the model:\n%s", doc)
	}
	if strings.Count(doc, `"id":"TKT-1"`) != 1 {
		t.Errorf("TKT-1 not sent exactly once:\n%s", doc)
	}
	a.unchanged(t)
}

func TestLLMBatch_HiddenFieldsAreNotSent(t *testing.T) {
	a := newBatchApp(t, `{"records":{"TKT-1":{"set":{"priority":null}}}}`)
	setTestResolver(t, a.app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"secret": false}}})
	code, _, body := a.runBatch(t, "alice", `{"entity_ids":["TKT-1"]}`)
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	if sent := a.chat.sent(t); strings.Contains(sent, "S3CRET") {
		t.Errorf("hidden field sent to the model:\n%s", sent)
	}
}

func TestLLMBatch_EveryRowSkipped(t *testing.T) {
	a := newBatchApp(t, "")
	code, b, body := a.runBatch(t, "alice", `{"entity_ids":["TKT-3","nope"]}`)
	if code != http.StatusOK || b == nil {
		t.Fatalf("status %d: %s", code, body)
	}
	if len(b.Proposals) != 0 || b.Skipped != 2 {
		t.Errorf("batch = %+v", b)
	}
	if n := a.chat.calls(); n != 0 {
		t.Errorf("provider called %d times with nothing to send", n)
	}
	if !strings.Contains(body, `"proposals":[]`) {
		t.Errorf("empty proposals not encoded as a list: %s", body)
	}
}

func TestLLMBatch_Refusals(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		body     string
		reply    string
		single   bool
		want     int
		wantCode string
	}{
		{name: "permission missing", user: "bob", body: `{"entity_ids":["TKT-1"]}`,
			want: http.StatusForbidden, wantCode: "permission_required"},
		{name: "not a batch action", user: "alice", single: true, body: `{"entity_ids":["TKT-1"]}`,
			want: http.StatusBadRequest, wantCode: "batch_not_supported"},
		{name: "both id forms", user: "alice", body: `{"entity_id":"TKT-1","entity_ids":["TKT-2"]}`,
			want: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "empty list", user: "alice", body: `{"entity_ids":[]}`,
			want: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "not strings", user: "alice", body: `{"entity_ids":[1,2]}`,
			want: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "over max", user: "alice", body: `{"entity_ids":["a","b","c","d","e","f"]}`,
			want: http.StatusBadRequest, wantCode: "batch_too_large"},
		{name: "unknown record in reply", user: "alice", body: `{"entity_ids":["TKT-1"]}`,
			reply: `{"records":{"TKT-1":{"set":{"priority":null}},"TKT-9":{"set":{"priority":null}}}}`,
			want:  http.StatusBadGateway, wantCode: "ai_reply_invalid"},
		{name: "reply not json", user: "alice", body: `{"entity_ids":["TKT-1"]}`, reply: "sure",
			want: http.StatusBadGateway, wantCode: "ai_reply_invalid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := newBatchApp(t, tc.reply)
			if tc.single {
				action := batchTriageAction(5)
				action.LLM.Batch = nil
				a.app.schema.Publish(&Schema{
					Cfg:  &Config{Actions: map[string]dataentryconfig.Action{"triage": action}},
					Meta: a.app.Meta(),
				})
			}
			code, _, body := a.runBatch(t, tc.user, tc.body)
			if code != tc.want || !strings.Contains(body, tc.wantCode) {
				t.Fatalf("got %d %s, want %d %s", code, body, tc.want, tc.wantCode)
			}
			a.unchanged(t)
		})
	}
}

// A batch run reads its rows in batches: the store reads do not grow with
// the number of rows.
func TestLLMBatch_ReadBudget(t *testing.T) {
	reads := func(t *testing.T, n int) int {
		t.Helper()
		var counting *storetest.Counting
		a := buildLLMApp(t, llmFixture{
			meta: llmMeta(t), seed: batchTickets(n), read: []string{"ticket", "concept"},
			action: batchTriageAction(50), reply: `{"records":{}}`,
			wrap: func(s store.Store) store.Store {
				counting = storetest.NewCounting(s)
				return counting
			},
		})
		ids := make([]string, n)
		for i := range n {
			ids[i] = fmt.Sprintf("%q", fmt.Sprintf("TKT-%d", i+1))
		}
		counting.Reset()
		code, b, body := a.runBatch(t, "alice", `{"entity_ids":[`+strings.Join(ids, ",")+`]}`)
		if code != http.StatusOK || b == nil {
			t.Fatalf("status %d: %s", code, body)
		}
		if len(b.Failed) != n {
			t.Fatalf("%d rows sent, want %d", len(b.Failed), n)
		}
		return counting.Reads()
	}
	r10, r50 := reads(t, 10), reads(t, 50)
	if r10 == 0 {
		t.Fatal("no store reads counted: the run does not read through the counting store")
	}
	t.Logf("store reads per run: %d at 10, %d at 50", r10, r50)
	if r10 != r50 {
		t.Errorf("store reads grow with rows: %d at 10, %d at 50", r10, r50)
	}
}

// The list and kanban `_actions` key of a batch action and the gate agree on
// the permission.
func TestLLMBatch_AffordanceAndGateAgree(t *testing.T) {
	for _, tc := range []struct {
		user string
		want bool
	}{{"alice", true}, {"bob", false}} {
		t.Run(tc.user, func(t *testing.T) {
			a := newBatchApp(t, `{"records":{"TKT-1":{"set":{"priority":null}}}}`)
			a.app.schema.Publish(&Schema{
				Cfg: &Config{
					Actions: map[string]dataentryconfig.Action{"triage": batchTriageAction(5)},
					Kanbans: map[string]dataentryconfig.Kanban{"board": {EntityType: "ticket", Actions: []string{"triage"}}},
				},
				Meta: a.app.Meta(),
			})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody).
				WithContext(gateCtxFor(principalCtx(tc.user), t, a.decl))
			rec := httptest.NewRecorder()
			a.app.handleV1ListEntities(rec, req, "ticket", "tickets")
			var got struct {
				Actions map[string]bool `json:"_actions"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v: %s", err, rec.Body)
			}
			if got.Actions["action:triage"] != tc.want {
				t.Errorf("affordance = %v, want %v", got.Actions["action:triage"], tc.want)
			}
			code, _, body := a.runBatch(t, tc.user, `{"entity_ids":["TKT-1"]}`)
			if ran := code == http.StatusOK; ran != tc.want {
				t.Errorf("run status %d: %s", code, body)
			}
		})
	}
}

// Rows the gate leaves out are skipped and never reach the model: a row of a
// type the caller cannot read, an address naming a face the row does not
// have, and a row whose when reads a property hidden from the caller.
func TestLLMBatch_GatedRowsAreNotSent(t *testing.T) {
	tests := []struct {
		name   string
		read   []string
		hidden bool
		when   string
		body   string
	}{
		{name: "unreadable type", read: []string{"concept"},
			body: `{"entity_ids":["TKT-1","TKT-2"]}`},
		{name: "face not served", read: []string{"ticket"},
			body: `{"entity_ids":["TKT-1@draft","TKT-2@draft"]}`},
		{name: "when reads a hidden property", read: []string{"ticket"}, hidden: true,
			when: "entity.secret == 'S3CRET'", body: `{"entity_ids":["TKT-1","TKT-2"]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			action := batchTriageAction(5)
			if tc.when != "" {
				action.When = tc.when
			}
			a := buildLLMApp(t, llmFixture{
				meta: llmMeta(t), seed: batchTickets(2), read: tc.read,
				action: action, reply: `{"records":{}}`,
			})
			if tc.hidden {
				setTestResolver(t, a.app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"secret": false}}})
			}
			code, b, body := a.runBatch(t, "alice", tc.body)
			if code != http.StatusOK || b == nil {
				t.Fatalf("status %d: %s", code, body)
			}
			if len(b.Proposals) != 0 || len(b.Failed) != 0 || b.Skipped != 2 {
				t.Errorf("batch = %+v, want 2 skipped", b)
			}
			if n := a.chat.calls(); n != 0 {
				t.Errorf("provider called %d times for gated-out rows", n)
			}
		})
	}
}

// newPolicyBatchApp is newBatchApp with field verdicts taken from the ACL
// policy itself and extra roles on top of the defaults.
func newPolicyBatchApp(t *testing.T, reply string, roles map[string]acl.RoleDef, assign map[string]string) llmApp {
	t.Helper()
	return buildLLMApp(t, llmFixture{
		meta: llmMeta(t), seed: batchTickets(2), read: []string{"ticket", "concept"},
		action: batchTriageAction(5), reply: reply,
		roles: roles, assign: assign, policyFields: true,
	})
}

// TestLLMBatch_PolicyHiddenFieldIsNotSent runs the redaction through a real
// `visible:` grant rather than a stubbed verdict.
func TestLLMBatch_PolicyHiddenFieldIsNotSent(t *testing.T) {
	a := newPolicyBatchApp(t, `{"records":{"TKT-1":{"set":{"priority":null}},"TKT-2":{"set":{"priority":null}}}}`,
		map[string]acl.RoleDef{"masked": {
			Read: []string{"ticket", "concept"}, Update: []string{"ticket"}, Permissions: []string{"ticket.triage"},
			Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "title"}, {Field: "priority"}}},
		}},
		map[string]string{"carol": "masked"})
	code, b, body := a.runBatch(t, "carol", `{"entity_ids":["TKT-1","TKT-2"]}`)
	if code != http.StatusOK || b == nil {
		t.Fatalf("status %d: %s", code, body)
	}
	sent := a.chat.sent(t)
	if strings.Contains(sent, "S3CRET") {
		t.Errorf("hidden field sent to the model:\n%s", sent)
	}
	if !strings.Contains(sent, "Ticket 1") {
		t.Errorf("visible field missing from the prompt:\n%s", sent)
	}
}

// TestLLMBatch_ReadableButNotUpdatableIsSkipped pins that a row the caller
// may read but not update is not sent: a proposal for it could not be applied.
func TestLLMBatch_ReadableButNotUpdatableIsSkipped(t *testing.T) {
	a := newPolicyBatchApp(t, "",
		map[string]acl.RoleDef{"viewer": {Read: []string{"ticket", "concept"}, Permissions: []string{"ticket.triage"}}},
		map[string]string{"dave": "viewer"})
	code, b, body := a.runBatch(t, "dave", `{"entity_ids":["TKT-1","TKT-2"]}`)
	if code != http.StatusOK || b == nil {
		t.Fatalf("status %d: %s", code, body)
	}
	if len(b.Proposals) != 0 || b.Skipped != 2 {
		t.Errorf("batch = %+v, want every row skipped", b)
	}
	if n := a.chat.calls(); n != 0 {
		t.Errorf("provider called %d times for rows the caller cannot update", n)
	}
}
