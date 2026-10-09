package affordances

import (
	"context"
	"testing"
)

func TestCheckFieldWrite(t *testing.T) {
	declared := map[string]bool{"title": true, "status": true, "tags": true, "salary": true}
	v := FieldVerdicts{
		Visible:     map[string]bool{"salary": false},
		Writable:    map[string]bool{"status": false, "extra": true},
		Options:     map[string]map[string]bool{"tags": {"secret": false, "ok": true}},
		Attribution: map[string]string{"status": "role:viewer", "tags=secret": "role:tagger"},
	}
	for _, tc := range []struct {
		name   string
		set    map[string]any
		unset  []string
		wantID string
		attrib string
	}{
		{name: "allowed", set: map[string]any{"title": "x", "tags": "ok"}},
		{name: "undeclared is hidden", set: map[string]any{"nope": 1}, wantID: "field-affordance:hidden:nope"},
		{name: "known to resolver passes", set: map[string]any{"extra": 1}},
		{name: "hidden", set: map[string]any{"salary": "1"}, wantID: "field-affordance:hidden:salary"},
		{name: "read-only", set: map[string]any{"status": "x"}, wantID: "field-affordance:read-only:status",
			attrib: "role:viewer"},
		{name: "unset read-only", unset: []string{"status"}, wantID: "field-affordance:read-only:status"},
		{name: "unset hidden", unset: []string{"salary"}, wantID: "field-affordance:hidden:salary"},
		{name: "enum scalar", set: map[string]any{"tags": "secret"}, wantID: "field-affordance:enum-filtered:tags=secret",
			attrib: "role:tagger"},
		{name: "enum list", set: map[string]any{"tags": []any{"ok", "secret"}},
			wantID: "field-affordance:enum-filtered:tags=secret"},
		{name: "enum string list", set: map[string]any{"tags": []string{"secret"}},
			wantID: "field-affordance:enum-filtered:tags=secret"},
		{name: "nil value skips enum", set: map[string]any{"tags": nil}},
		{name: "first in name order", set: map[string]any{"status": "x", "salary": "1"},
			wantID: "field-affordance:hidden:salary"},
		{name: "set before unset", set: map[string]any{"status": "x"}, unset: []string{"salary"},
			wantID: "field-affordance:read-only:status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := CheckFieldWrite(v, declared, tc.set, tc.unset)
			if tc.wantID == "" {
				if d != nil {
					t.Fatalf("denied %s, want allowed", d.RuleID())
				}
				return
			}
			if d == nil {
				t.Fatalf("allowed, want %s", tc.wantID)
			}
			if d.RuleID() != tc.wantID {
				t.Errorf("rule = %s, want %s", d.RuleID(), tc.wantID)
			}
			if tc.attrib != "" && d.Attribution != tc.attrib {
				t.Errorf("attribution = %q, want %q", d.Attribution, tc.attrib)
			}
		})
	}
}

func TestNewWriteGate_RejectsNil(t *testing.T) {
	if _, err := NewWriteGate(nil); err == nil {
		t.Fatal("NewWriteGate(nil) succeeded")
	}
}

// TestWriteGate_NilEntityRefuses: a gate unsure what it checks refuses.
func TestWriteGate_NilEntityRefuses(t *testing.T) {
	g := &WriteGate{resolver: &PolicyResolver{}}
	if err := g.CheckFieldWrite(context.Background(), nil, map[string]any{"x": 1}, nil); err == nil {
		t.Fatal("nil entity allowed")
	}
}

func TestFieldWriteError_AuditSummary(t *testing.T) {
	d := &FieldWriteError{Rule: RuleFieldReadOnly, Path: "status", Reason: "r", Attribution: "role:x"}
	want := "denied: r (rule_kind=affordance rule_id=field-affordance:read-only:status) attribution=role:x"
	if got := d.AuditSummary(); got != want {
		t.Errorf("AuditSummary = %q, want %q", got, want)
	}
}
