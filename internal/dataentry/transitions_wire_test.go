package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
)

// fakeTransitionResolver is a FieldVerdictResolver that ALSO implements
// TransitionResolver, so tests can drive the `_transitions` wire path and the
// create-lock without compiling a real state machine. The base field/relation
// verdicts are permissive (empty).
type fakeTransitionResolver struct {
	verdicts map[string][]statemachine.TransitionVerdict
	entries  map[string]string
	hidden   map[string]bool // field name → hidden (Visible=false)
}

func (f fakeTransitionResolver) FieldVerdicts(context.Context, *entity.Entity) FieldVerdicts {
	if len(f.hidden) == 0 {
		return FieldVerdicts{}
	}
	vis := map[string]bool{}
	for name := range f.hidden {
		vis[name] = false
	}
	return FieldVerdicts{Visible: vis}
}

func (f fakeTransitionResolver) RelationVerdicts(context.Context, *entity.Entity) RelationVerdicts {
	return RelationVerdicts{}
}

func (f fakeTransitionResolver) TransitionVerdicts(
	_ context.Context, _ *entity.Entity,
) map[string][]statemachine.TransitionVerdict {
	return f.verdicts
}

func (f fakeTransitionResolver) EntryValues(string) map[string]string {
	return f.entries
}

// Compile-time assertion that the fake satisfies both seams.
var (
	_ FieldVerdictResolver = fakeTransitionResolver{}
	_ TransitionResolver   = fakeTransitionResolver{}
)

func getEntityV1(t *testing.T, app *App, typeName, plural, id string) v1.Entity {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/"+plural+"/"+id, http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, typeName, plural, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: got %d, want 200; body=%s", id, rec.Code, rec.Body.String())
	}
	var e v1.Entity
	if err := json.NewDecoder(rec.Body).Decode(&e); err != nil {
		t.Fatalf("decode GET: %v", err)
	}
	return e
}

// AC1: a per-entity GET carries `_transitions` for a machine-typed field,
// mapping every field of the TransitionVerdict onto the wire shape (to, label,
// guard, allowed, reason).
func TestTransitionsWire_GETCarriesTransitions(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = fakeTransitionResolver{
		verdicts: map[string][]statemachine.TransitionVerdict{
			"status": {
				{To: "doing", Label: "Start progress", Allowed: true, Reason: statemachine.VerdictAllowed},
				{To: "done", Label: "Complete", Guard: "close", Allowed: false, Reason: statemachine.VerdictGuard},
			},
		},
	}
	seedEntity(app, &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "x", "status": "todo"},
	})

	e := getEntityV1(t, app, "ticket", "tickets", "TKT-001")
	if e.Transitions == nil {
		t.Fatalf("expected _transitions present, got nil")
	}
	got := (*e.Transitions)["status"]
	if len(got) != 2 {
		t.Fatalf("expected 2 transitions, got %d: %+v", len(got), got)
	}

	start := got[0]
	if start.To != "doing" || start.Label != "Start progress" || !start.Allowed || start.Reason != "" {
		t.Errorf("start move wire mismatch: %+v", start)
	}
	complete := got[1]
	badComplete := complete.To != "done" || complete.Label != "Complete" ||
		complete.Guard != "close" || complete.Allowed || complete.Reason != "guard"
	if badComplete {
		t.Errorf("complete move wire mismatch: %+v", complete)
	}
}

// AC1 (absence): a resolver that does NOT implement TransitionResolver (the
// default Nop, or the plain fakeResolver) leaves `_transitions` off the wire —
// the SPA then falls back to the ordinary enum control.
func TestTransitionsWire_AbsentWithoutTransitionResolver(t *testing.T) {
	app := newTestAppV1(t)
	// newTestAppV1 wires NopFieldVerdictResolver, which does not implement
	// TransitionResolver.
	seedEntity(app, &entity.Entity{
		ID:         "TKT-002",
		Type:       "ticket",
		Properties: map[string]any{"title": "x", "status": "todo"},
	})

	e := getEntityV1(t, app, "ticket", "tickets", "TKT-002")
	if e.Transitions != nil {
		t.Fatalf("expected _transitions absent under non-TransitionResolver, got %+v", *e.Transitions)
	}
}

// RR-DENG8U: a machine field hidden by field-visibility policy must NOT leak its
// current state via `_transitions` — the out-edge set is a function of the
// current value, so emitting it would defeat the hidden-field strip.
func TestTransitionsWire_HiddenMachineFieldOmitted(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = fakeTransitionResolver{
		verdicts: map[string][]statemachine.TransitionVerdict{
			"status": {{To: "doing", Label: "Start progress", Allowed: true}},
		},
		hidden: map[string]bool{"status": true},
	}
	seedEntity(app, &entity.Entity{
		ID:         "TKT-003",
		Type:       "ticket",
		Properties: map[string]any{"title": "x", "status": "todo"},
	})

	e := getEntityV1(t, app, "ticket", "tickets", "TKT-003")
	// The hidden value is stripped from properties...
	if _, present := e.Properties["status"]; present {
		t.Errorf("hidden status must be stripped from properties, got %v", e.Properties["status"])
	}
	// ...and must not reappear via _transitions.
	if e.Transitions != nil {
		if _, present := (*e.Transitions)["status"]; present {
			t.Errorf("hidden machine field must not carry _transitions (leak), got %+v", *e.Transitions)
		}
	}
}

// RR-C3OJ33: the create dry-run must not re-insert a hidden machine field's entry
// value into the response (which would re-leak it AND re-surface it in the SPA
// create form, whose visible-field set derives from the response property keys).
func TestTransitionsWire_CreateLockSkipsHiddenField(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = fakeTransitionResolver{
		entries: map[string]string{"status": "todo"},
		hidden:  map[string]bool{"status": true},
	}

	body := `{"properties":{"title":"new ticket"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets?dry_run=true", strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.handleV1DryRunCreate(rec, req, "ticket", "tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run create: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var e v1.Entity
	if err := json.NewDecoder(rec.Body).Decode(&e); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}
	// The hidden field's entry value must NOT be re-added to the wire.
	if _, present := e.Properties["status"]; present {
		t.Errorf("create-lock must not re-insert a hidden field, got %v", e.Properties["status"])
	}
}

// AC4: the create dry-run locks a machine field to its entry value — the field
// is marked read-only in `_fields`, its value is pinned to the entry, and it
// carries no `_transitions` (a create is an entry, not a move).
func TestTransitionsWire_CreateLocksMachineField(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = fakeTransitionResolver{
		// On a create candidate the resolver would still report transitions
		// from the seeded value; applyCreateLock must strip them for the
		// locked field.
		verdicts: map[string][]statemachine.TransitionVerdict{
			"status": {{To: "doing", Label: "Start progress", Allowed: true}},
		},
		entries: map[string]string{"status": "todo"},
	}

	body := `{"properties":{"title":"new ticket","status":"doing"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets?dry_run=true", strings.NewReader(body))
	rec := httptest.NewRecorder()
	app.handleV1DryRunCreate(rec, req, "ticket", "tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run create: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var e v1.Entity
	if err := json.NewDecoder(rec.Body).Decode(&e); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}

	// Value pinned to the entry, overriding the "doing" the client sent.
	if got := e.Properties["status"]; got != "todo" {
		t.Errorf("status must be pinned to entry %q, got %v", "todo", got)
	}
	// Field marked read-only so the SPA renders it locked.
	if e.FieldAffordances == nil {
		t.Fatalf("expected _fields present")
	}
	fa, ok := (*e.FieldAffordances)["status"]
	if !ok || fa.Writable == nil || *fa.Writable {
		t.Errorf("status must be writable=false on create, got %+v (present=%v)", fa, ok)
	}
	// No transitions for the locked field — create is an entry, not a move.
	if e.Transitions != nil {
		if _, present := (*e.Transitions)["status"]; present {
			t.Errorf("locked machine field must not carry _transitions on create, got %+v", *e.Transitions)
		}
	}
}
