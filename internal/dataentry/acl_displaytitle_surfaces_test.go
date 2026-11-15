package dataentry

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// These tests pin the non-view DisplayTitle surfaces of BUG-R9EHKV
// (mentions here, analyze in acl_analyze_test.go). The third, the settings
// relation-target picker, no longer reads entities (TKT-WJP6PBM); see
// settings_budget_test.go. Each fetched a target entity raw from the store and
// titled it via DisplayTitle against raw properties, leaking a hidden display
// value (and, for the ungated ones, unreadable entities entirely). The fix
// routes each through the read gate + redactor (visibility.Reader).

// TestACLMentions_RedactsHiddenPrimaryTitle: a mention (code-span entity id in
// markdown) to a readable entity whose display property is hidden must render
// the id, not the hidden value.
func TestACLMentions_RedactsHiddenPrimaryTitle(t *testing.T) {
	app := newTestAppV1(t)
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"title": false},
	}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "SECRET-MENTION"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	got := collectMentions(gateCtxFor(aliceCtx(), t, d), app.viewReader,
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
	setTestACL(t, app, d)

	got := collectMentions(gateCtxFor(aliceCtx(), t, d), app.viewReader,
		app.State().Meta, "see `FEAT-001`")
	if _, ok := got["FEAT-001"]; ok {
		t.Errorf("LEAK: unreadable entity resolved as a mention: %+v", got)
	}
}
