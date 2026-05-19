package acl_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// AC1.2: NopACL allows every write regardless of request shape.
func TestNopACL_AllowsAllWrites(t *testing.T) {
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
		{"zero request", acl.WriteRequest{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := acl.NopACL{}.AuthorizeWrite(context.Background(), tt.req)
			if !d.Allow {
				t.Errorf("Allow = false, want true (NopACL must never deny). Decision = %+v", d)
			}
		})
	}
}
