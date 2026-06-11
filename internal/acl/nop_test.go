package acl_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// AC1.2: NopACL allows every write regardless of request shape.
func TestNopACL_AllowsAllWrites(t *testing.T) {
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
		{"zero request", acl.WriteRequest{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := acl.NopACL{}.AuthorizeWrite(context.Background(), tt.req)
			if !d.Allow {
				t.Errorf("Allow = false, want true (NopACL must never deny). Decision = %+v", d)
			}
		})
	}
}
