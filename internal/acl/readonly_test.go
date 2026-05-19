package acl_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// AC1.3: ReadOnlyACL denies every write with the documented Decision shape.
func TestReadOnlyACL_DeniesAllWrites(t *testing.T) {
	tests := []struct {
		name string
		req  acl.WriteRequest
	}{
		{"create entity", acl.WriteRequest{Op: acl.OpCreate, EntityType: "ticket"}},
		{"update entity", acl.WriteRequest{Op: acl.OpUpdate, EntityType: "concept"}},
		{"delete entity", acl.WriteRequest{Op: acl.OpDelete, EntityType: "person"}},
		{"rename entity", acl.WriteRequest{Op: acl.OpRename, EntityType: "feature"}},
		{"create relation", acl.WriteRequest{Op: acl.OpCreate, EntityType: "ticket", RelationType: "affects"}},
		{"update relation", acl.WriteRequest{Op: acl.OpUpdate, RelationType: "depends-on"}},
		{"delete relation", acl.WriteRequest{Op: acl.OpDelete, RelationType: "requires"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := acl.ReadOnlyACL{}.AuthorizeWrite(context.Background(), tt.req)
			if d.Allow {
				t.Errorf("Allow = true, want false (ReadOnlyACL must always deny)")
			}
			if d.RuleKind != "read-only" {
				t.Errorf("RuleKind = %q, want %q", d.RuleKind, "read-only")
			}
			if d.RuleID != "read-only-acl" {
				t.Errorf("RuleID = %q, want %q", d.RuleID, "read-only-acl")
			}
			if d.Reason == "" {
				t.Error("Reason is empty; want operator-facing explanation")
			}
		})
	}
}
