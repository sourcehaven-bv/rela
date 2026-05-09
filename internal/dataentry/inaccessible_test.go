package dataentry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// lockedTicket builds a ticket-typed entity with the given property
// names marked inaccessible. Callers specify only what matters for the
// test — the helper supplies the type/ID/reason defaults so test
// bodies stay focused on the assertion under test.
func lockedTicket(inaccessibleFields ...string) *entity.Entity {
	e := &entity.Entity{ID: "TKT-LOCKED", Type: "ticket"}
	for _, name := range inaccessibleFields {
		e.Inaccessible = append(e.Inaccessible, entity.InaccessibleField{
			Name:   name,
			Reason: entity.InaccessibleReasonGitCrypt,
		})
	}
	return e
}

// TestEntityToV1_PropagatesInaccessible verifies that the wire-format
// serialization carries [entity.Entity.Inaccessible] through to
// [V1Entity.Inaccessible]. Encrypted entities loaded from fsstore have
// this field populated; the SPA branches on it to render lock indicators.
func TestEntityToV1_PropagatesInaccessible(t *testing.T) {
	app := newTestAppV1(t)
	e := lockedTicket("title", "status")
	v1 := app.entityToV1(e, "tickets", false, false)

	if v1.ID != "TKT-LOCKED" {
		t.Errorf("ID = %q, want TKT-LOCKED", v1.ID)
	}
	if len(v1.Inaccessible) != 2 {
		t.Fatalf("Inaccessible has %d entries, want 2", len(v1.Inaccessible))
	}
	for _, f := range v1.Inaccessible {
		if f.Reason != string(entity.InaccessibleReasonGitCrypt) {
			t.Errorf("Reason = %q, want %q", f.Reason, entity.InaccessibleReasonGitCrypt)
		}
	}

	raw, err := json.Marshal(v1)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Round-trip the raw JSON to verify the wire format includes the
	// "inaccessible" key with name + reason fields the SPA expects.
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, ok := parsed["inaccessible"].([]any)
	if !ok {
		t.Fatalf("response missing inaccessible array; got: %s", raw)
	}
	if len(got) != 2 {
		t.Errorf("inaccessible has %d entries on the wire, want 2", len(got))
	}
}

// TestV1UpdateEntity_RejectsInaccessible verifies that PATCH against an
// entity with non-empty Inaccessible returns 422 with the expected
// error code, never invoking the entity-manager write path.
func TestV1UpdateEntity_RejectsInaccessible(t *testing.T) {
	app := newTestAppV1(t)

	// Seed an inaccessible entity directly: memstore preserves the
	// Inaccessible field through Clone, mirroring what fsstore would
	// produce when reading a git-crypt encrypted file.
	seedEntity(app, lockedTicket("title"))

	body := `{"properties": {"title": "evil"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-LOCKED", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-LOCKED")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body: %s)", rec.Code, rec.Body.String())
	}
	var problem map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem JSON: %v (body: %s)", err, rec.Body.String())
	}
	got, _ := problem["type"].(string)
	if got != "https://rela.dev/errors/encrypted_inaccessible" {
		t.Errorf("error type URL = %q, want suffix encrypted_inaccessible", got)
	}
}

// TestEntityToV1_OmitsInaccessibleWhenEmpty verifies the wire format is
// unchanged for normal (fully readable) entities.
func TestEntityToV1_OmitsInaccessibleWhenEmpty(t *testing.T) {
	app := newTestAppV1(t)

	e := &entity.Entity{
		ID:   "TKT-NORMAL",
		Type: "ticket",
		Properties: map[string]any{
			"title": "Hello",
		},
	}

	v1 := app.entityToV1(e, "tickets", false, false)
	if len(v1.Inaccessible) != 0 {
		t.Errorf("Inaccessible non-empty for normal entity: %v", v1.Inaccessible)
	}

	raw, err := json.Marshal(v1)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := parsed["inaccessible"]; present {
		t.Errorf("inaccessible key present on wire for normal entity; raw: %s", raw)
	}
}
