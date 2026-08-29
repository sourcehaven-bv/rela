package dataentry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// These tests pin the three non-view DisplayTitle surfaces of BUG-R9EHKV
// (mentions, settings relation-target picker, analyze — analyze lives in
// acl_analyze_test.go). Each fetched a target entity raw from the store and
// titled it via DisplayTitle against raw properties, leaking a hidden display
// value (and, for the ungated ones, unreadable entities entirely). The fix
// routes each through the read gate + redactor (visibility.Reader).

// TestACLMentions_RedactsHiddenPrimaryTitle: a mention (code-span entity id in
// markdown) to a readable entity whose display property is hidden must render
// the id, not the hidden value.
func TestACLMentions_RedactsHiddenPrimaryTitle(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"title": false},
	}}
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "SECRET-MENTION"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	got := collectMentions(gateCtxFor(aliceCtx(), t, d), app.store, app.viewReader,
		app.State().Meta, "see `TKT-001`")
	if m, ok := got["TKT-001"]; ok && strings.Contains(m.Title, "SECRET-MENTION") {
		t.Errorf("LEAK: hidden display value in mention title: %+v", m)
	}
}

// TestACLMentions_DropsUnreadable: a mention to an entity the principal may not
// read must not resolve at all (no id, no title).
func TestACLMentions_DropsUnreadable(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "unreadable feature"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	got := collectMentions(gateCtxFor(aliceCtx(), t, d), app.store, app.viewReader,
		app.State().Meta, "see `FEAT-001`")
	if _, ok := got["FEAT-001"]; ok {
		t.Errorf("LEAK: unreadable entity resolved as a mention: %+v", got)
	}
}

// TestACLSettings_RelationTargetsRedactHiddenTitle: the settings relation-target
// picker lists candidate targets via an ungated store read. A hidden display
// property must not leak, and an unreadable target must not appear.
func TestACLSettings_RelationTargetsRedactHiddenTitle(t *testing.T) {
	app := newTestAppV1(t)
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"title": false},
	}}
	// implements targets features; feature's display property is its title.
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "SECRET-TARGET"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket", "feature"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	req := httptest.NewRequest(http.MethodGet, "/api/settings", http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.appearance.handleAPIGetSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("settings: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); strings.Contains(body, "SECRET-TARGET") {
		t.Errorf("LEAK: hidden target title in settings relation picker: %s", body)
	}
}

// TestACLSettings_RelationTargetsDropUnreadable: a target of a type the
// principal cannot read must not appear in the picker at all.
func TestACLSettings_RelationTargetsDropUnreadable(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "UNREADABLE-TARGET"}})

	// viewer reads tickets but NOT features → feature targets are unreadable.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	req := httptest.NewRequest(http.MethodGet, "/api/settings", http.NoBody)
	req = req.WithContext(gateCtxFor(aliceCtx(), t, d))
	rec := httptest.NewRecorder()
	app.appearance.handleAPIGetSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("settings: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); strings.Contains(body, "UNREADABLE-TARGET") || strings.Contains(body, "FEAT-001") {
		t.Errorf("LEAK: unreadable target in settings relation picker: %s", body)
	}
}
