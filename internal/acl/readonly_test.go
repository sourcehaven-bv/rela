package acl_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// AC1.3: ReadOnlyACL denies every write with the documented Decision shape.
func TestReadOnlyACL_DeniesAllWrites(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  acl.WriteRequest
	}{
		{"create entity", acl.WriteRequest{Op: acl.OpCreate, Subject: acl.EntitySubject{Type: "ticket"}}},
		{"update entity", acl.WriteRequest{Op: acl.OpUpdate, Subject: acl.EntitySubject{Type: "concept", ID: "C-1"}}},
		{"delete entity", acl.WriteRequest{Op: acl.OpDelete, Subject: acl.EntitySubject{Type: "person", ID: "P-1"}}},
		{"rename entity", acl.WriteRequest{Op: acl.OpRename, Subject: acl.EntitySubject{Type: "feature", ID: "FEAT-1"}}},
		{"create relation", acl.WriteRequest{Op: acl.OpCreate, Subject: acl.RelationSubject{Type: "affects", FromType: "ticket"}}},
		{"update relation", acl.WriteRequest{Op: acl.OpUpdate, Subject: acl.RelationSubject{Type: "depends-on", FromType: "ticket"}}},
		{"delete relation", acl.WriteRequest{Op: acl.OpDelete, Subject: acl.RelationSubject{Type: "requires", FromType: "feature"}}},
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
