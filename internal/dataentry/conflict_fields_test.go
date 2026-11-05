package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestConflictFieldWrite(t *testing.T) {
	tests := []struct {
		name                   string
		ours, theirs, resolved map[string]any
		wantSet                map[string]any
		wantUnset              []string
	}{
		{
			name: "agreed property is no write", ours: map[string]any{"a": 1}, theirs: map[string]any{"a": 1},
			resolved: map[string]any{"a": 1}, wantSet: map[string]any{},
		},
		{
			name: "disagreed property is written with the resolved value", ours: map[string]any{"a": 1},
			theirs: map[string]any{"a": 2}, resolved: map[string]any{"a": 1}, wantSet: map[string]any{"a": 1},
		},
		{
			name: "property on one side only is written", ours: map[string]any{}, theirs: map[string]any{"a": 2},
			resolved: map[string]any{"a": 2}, wantSet: map[string]any{"a": 2},
		},
		{
			name: "property the resolution drops is unset", ours: map[string]any{"a": 1}, theirs: map[string]any{},
			resolved: map[string]any{}, wantSet: map[string]any{}, wantUnset: []string{"a"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			set, unset := conflictFieldWrite(tc.ours, tc.theirs, tc.resolved)
			if fmt.Sprint(set) != fmt.Sprint(tc.wantSet) || fmt.Sprint(unset) != fmt.Sprint(tc.wantUnset) {
				t.Errorf("got set %v unset %v, want set %v unset %v", set, unset, tc.wantSet, tc.wantUnset)
			}
		})
	}
}

// conflictedTicketPoints is a ticket conflict on title whose sides agree on
// points, a value out of the declared range 1 to 5.
const conflictedTicketPoints = `<<<<<<< HEAD
---
id: TKT-001
type: ticket
title: Ours
points: 9137
---
=======
---
id: TKT-001
type: ticket
title: Theirs
points: 9137
---
>>>>>>> incoming
Body text.
`

// TestV1ConflictResolve_FieldWriteGate pins finding 1 of BUG-197XPAU: a
// resolution writes every property its sides disagree on, so a field the
// caller may not write is refused like the same field in a PATCH, whichever
// side the caller picks. A field the sides agree on is no write.
func TestV1ConflictResolve_FieldWriteGate(t *testing.T) {
	tests := []struct {
		name     string
		verdicts FieldVerdicts
		choice   string
		wantCode int
	}{
		{"read-only field, theirs", FieldVerdicts{Writable: map[string]bool{"title": false}}, "theirs", http.StatusForbidden},
		{"read-only field, ours", FieldVerdicts{Writable: map[string]bool{"title": false}}, "ours", http.StatusForbidden},
		{"hidden field", FieldVerdicts{Visible: map[string]bool{"title": false}}, "theirs", http.StatusForbidden},
		{"read-only field both sides agree on", FieldVerdicts{Writable: map[string]bool{"status": false}}, "theirs", http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app, root := newConflictTestApp(t)
			setTestResolver(t, app, fakeResolver{fv: tc.verdicts})
			sink := audit.NewMemory()
			app.auditSink = sink
			file := strings.Replace(conflictedTicket, "title: Theirs\n", "title: Theirs\nstatus: open\n", 1)
			file = strings.Replace(file, "title: Ours\n", "title: Ours\nstatus: open\n", 1)
			path := writeProjectFile(t, root, "entities/ticket/TKT-001.md", file)

			rec := postConflictResolve(app,
				`{"path":"entities/ticket/TKT-001.md","property_choices":{"title":"`+tc.choice+`"}}`)

			if rec.Code != tc.wantCode {
				t.Fatalf("resolve = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body)
			}
			if tc.wantCode == http.StatusOK {
				return
			}
			if got, _ := os.ReadFile(path); string(got) != file {
				t.Error("refused resolve rewrote the file")
			}
			var denied bool
			for _, r := range sink.Records() {
				denied = denied || (r.Op == audit.OpDeniedWrite && r.Subject != nil && r.Subject.ID == "TKT-001")
				if r.Op == audit.OpUpdateEntity {
					t.Errorf("refused resolve audited a write: %+v", r)
				}
			}
			if !denied {
				t.Errorf("no denied-write audit row: %+v", sink.Records())
			}
		})
	}
}

// TestV1ConflictResolve_RelationMetaWriteGate is the relation half of
// TestV1ConflictResolve_FieldWriteGate: a meta field the source's verdicts
// make read-only is refused when the sides disagree on it.
func TestV1ConflictResolve_RelationMetaWriteGate(t *testing.T) {
	app, root := newConflictTestApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
	setTestResolver(t, app, fakeResolver{rv: RelationVerdicts{Types: map[string]RelationVerdict{
		"blocks": {Creatable: true, Removable: true, Fields: map[string]bool{"note": false}},
	}}})
	path := writeProjectFile(t, root, "relations/TKT-001--blocks--TKT-002.md", conflictedRelation)

	rec := postConflictResolve(app,
		`{"path":"relations/TKT-001--blocks--TKT-002.md","property_choices":{"note":"theirs"}}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("resolve = %d, want 403: %s", rec.Code, rec.Body)
	}
	if got, _ := os.ReadFile(path); string(got) != conflictedRelation {
		t.Error("refused resolve rewrote the file")
	}
}

// TestV1ConflictResolve_ValidationErrorHidesValue pins finding 2 of
// BUG-197XPAU: the 422 for an invalid value in a hidden field names the
// rule, not the value.
func TestV1ConflictResolve_ValidationErrorHidesValue(t *testing.T) {
	app, root := newConflictTestApp(t)
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"points": false}}})
	writeProjectFile(t, root, "entities/ticket/TKT-001.md", conflictedTicketPoints)

	rec := postConflictResolve(app, `{"path":"entities/ticket/TKT-001.md","property_choices":{"title":"theirs"}}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("resolve = %d, want 422: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "9137") {
		t.Errorf("422 carries the hidden value: %s", rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "points: Out of range") {
		t.Errorf("422 does not name the property and the rule: %s", rec.Body)
	}
}

// TestV1UpdateEntity_WarningHidesHiddenValue pins that the normal update
// path has the same guarantee: a hidden field's invalid value is validated
// on every write of its entity, and the warning on the 200 names the rule,
// not the value.
func TestV1UpdateEntity_WarningHidesHiddenValue(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "t", "points": 9137}})
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"points": false}}})

	rec := patchJSON(app, "TKT-001", `{"properties":{"title":"u"}}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "/properties/points") {
		t.Fatalf("no warning for the invalid hidden field: %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "9137") {
		t.Errorf("response carries the hidden value: %s", rec.Body)
	}
}

// lockedResolver makes title read-only on a row whose status is "locked",
// so a test can tell which row the field check judged.
type lockedResolver struct{ fakeResolver }

func (lockedResolver) FieldVerdicts(_ context.Context, e *entity.Entity) FieldVerdicts {
	if e != nil && e.Properties["status"] == "locked" {
		return FieldVerdicts{Writable: map[string]bool{"title": false}}
	}
	return FieldVerdicts{}
}

// ticketConflict is a TKT-001 conflict whose sides carry the given extra
// frontmatter lines after the title.
func ticketConflict(ours, theirs string) string {
	return "<<<<<<< HEAD\n---\nid: TKT-001\ntype: ticket\ntitle: Ours\n" + ours +
		"---\n=======\n---\nid: TKT-001\ntype: ticket\ntitle: Theirs\n" + theirs +
		"---\n>>>>>>> incoming\nBody text.\n"
}

// TestV1ConflictResolve_FieldWriteCases covers what the fixed-verdict cases
// above cannot: the check judges each side, not only the resolved row; a
// property the resolution drops is an unset; a filtered enum option is
// refused; and a property no schema or rule names does not block the
// resolve, because the caller has been shown it and no rule governs it.
func TestV1ConflictResolve_FieldWriteCases(t *testing.T) {
	tests := []struct {
		name     string
		resolver FieldVerdictResolver
		file     string
		choices  string
		wantCode int
	}{
		{
			name:     "a side the rule locks refuses, though the resolved row is unlocked",
			resolver: lockedResolver{},
			file:     ticketConflict("status: locked\n", "status: open\n"),
			choices:  `{"title":"theirs","status":"theirs"}`, wantCode: http.StatusForbidden,
		},
		{
			name:     "no side locked",
			resolver: lockedResolver{},
			file:     ticketConflict("status: open\n", "status: review\n"),
			choices:  `{"title":"theirs","status":"theirs"}`, wantCode: http.StatusOK,
		},
		{
			name:     "dropping a read-only property is refused",
			resolver: fakeResolver{fv: FieldVerdicts{Writable: map[string]bool{"status": false}}},
			file:     ticketConflict("status: open\n", ""),
			choices:  `{"title":"theirs","status":"theirs"}`, wantCode: http.StatusForbidden,
		},
		{
			name: "a filtered enum option is refused",
			resolver: fakeResolver{fv: FieldVerdicts{Options: map[string]map[string]bool{
				"status": {"closed": false},
			}}},
			file:    ticketConflict("status: open\n", "status: closed\n"),
			choices: `{"title":"theirs","status":"theirs"}`, wantCode: http.StatusForbidden,
		},
		{
			name:     "an undeclared property does not block the resolve",
			resolver: fakeResolver{},
			file:     ticketConflict("legacy: a\n", "legacy: b\n"),
			choices:  `{"title":"theirs","legacy":"theirs"}`, wantCode: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app, root := newConflictTestApp(t)
			setTestResolver(t, app, tc.resolver)
			path := writeProjectFile(t, root, "entities/ticket/TKT-001.md", tc.file)

			rec := postConflictResolve(app,
				`{"path":"entities/ticket/TKT-001.md","property_choices":`+tc.choices+`}`)

			if rec.Code != tc.wantCode {
				t.Fatalf("resolve = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body)
			}
			if got, _ := os.ReadFile(path); tc.wantCode != http.StatusOK && string(got) != tc.file {
				t.Error("refused resolve rewrote the file")
			}
		})
	}
}

// TestV1ConflictResolve_RelationMetaAllowed is the positive relation case: a
// read-only meta field the sides agree on is no write, so a resolve that
// changes only other meta passes.
func TestV1ConflictResolve_RelationMetaAllowed(t *testing.T) {
	app, root := newConflictTestApp(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
	setTestResolver(t, app, fakeResolver{rv: RelationVerdicts{Types: map[string]RelationVerdict{
		"blocks": {Creatable: true, Removable: true, Fields: map[string]bool{"weight": false}},
	}}})
	writeProjectFile(t, root, "relations/TKT-001--blocks--TKT-002.md", conflictedRelation)

	rec := postConflictResolve(app,
		`{"path":"relations/TKT-001--blocks--TKT-002.md","property_choices":{"note":"theirs"}}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("resolve = %d, want 200: %s", rec.Code, rec.Body)
	}
}
