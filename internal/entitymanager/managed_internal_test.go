package entitymanager

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

func TestManagedWriteDenial(t *testing.T) {
	t.Parallel()
	alice := principal.Principal{User: "alice", Tool: principal.ToolCLI}
	engine := principal.Principal{User: principal.UserFlow, Tool: principal.ToolFlow}
	run := acl.NewFacelessEntitySubject(entity.FlowRunType, "RUN-1")
	carries := acl.RelationSubject{Type: entity.CarriesRelation, FromType: entity.FlowRunType, FromID: "RUN-1"}
	tests := []struct {
		name   string
		who    principal.Principal
		req    acl.WriteRequest
		denied bool
	}{
		{"operator type", alice, acl.WriteRequest{Op: acl.OpCreate, Subject: acl.NewFacelessEntitySubject("task", "T-1")}, false},
		{"alice creates a run", alice, acl.WriteRequest{Op: acl.OpCreate, Subject: run}, true},
		{"engine creates a run", engine, acl.WriteRequest{Op: acl.OpCreate, Subject: run}, false},
		{"engine updates a run", engine, acl.WriteRequest{Op: acl.OpUpdate, Subject: run}, false},
		{"engine renames a run", engine, acl.WriteRequest{Op: acl.OpRename, Subject: acl.NewFamilySubject(entity.FlowRunType, "RUN-1")}, true},
		{"engine converts a run", engine, acl.WriteRequest{Op: acl.OpConvert, Subject: acl.NewConversionSubject(entity.FlowRunType, "RUN-1", "task")}, true},
		{"engine converts into a run", engine, acl.WriteRequest{Op: acl.OpConvert, Subject: acl.NewConversionSubject("task", "T-1", entity.FlowRunType)}, true},
		{"alice links a run", alice, acl.WriteRequest{Op: acl.OpCreate, Subject: carries}, true},
		{"engine links a run", engine, acl.WriteRequest{Op: acl.OpCreate, Subject: carries}, false},
		{"alice writes a managed relation type", alice, acl.WriteRequest{Op: acl.OpDelete, Subject: acl.RelationSubject{Type: entity.ProducedRelation, FromType: "task"}}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := managedWriteDenial(principal.With(context.Background(), tc.who), tc.req) != ""; got != tc.denied {
				t.Errorf("denied = %t, want %t", got, tc.denied)
			}
		})
	}
}
