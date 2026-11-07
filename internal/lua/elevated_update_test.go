package lua

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// recordingUpdater is an ElevatedUpdater that records each patch and returns
// the stored entity, or err when set.
type recordingUpdater struct {
	ws      *mockWorkspace
	ids     []string
	patches []entity.Patch
	err     error
}

func (u *recordingUpdater) PatchEntity(ctx context.Context, id string, p entity.Patch) (*entity.UpdateResult, error) {
	if u.err != nil {
		return nil, u.err
	}
	u.ids = append(u.ids, id)
	u.patches = append(u.patches, p)
	e, _ := u.ws.store.GetEntity(ctx, entity.Ref{ID: id})
	return &entity.UpdateResult{Entity: e}, nil
}

// notFoundError is the structural not-found error the manager returns.
type notFoundError struct{}

func (notFoundError) Error() string        { return "not found" }
func (notFoundError) EntityNotFound() bool { return true }

func writerWithUpdater(t *testing.T, u *recordingUpdater) *Runtime {
	t.Helper()
	deps := u.ws.services("/tmp")
	deps.ElevatedUpdater = u
	var buf bytes.Buffer
	return NewWriter(deps, &buf)
}

// TestBypassACL_ElevatedUpdater pins the admin handle of a data-entry action
// that declares elevated_updates (TKT-JA5SABG): update_entity is the only
// method, it passes a property-only patch to the updater, and it raises on
// misuse.
func TestBypassACL_ElevatedUpdater(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		script  string
		err     error
		wantErr string
		wantIDs []string
		check   func(t *testing.T, p entity.Patch)
	}{
		{
			name: "updates through the updater",
			script: `
				local e, warnings = rela.bypass_acl(function(admin)
					return admin.update_entity("TKT-001", {status = "done", title = rela.sync.EMPTY})
				end)
				assert(e.id == "TKT-001", "entity returned")
				assert(warnings == nil, "no warnings")`,
			wantIDs: []string{"TKT-001"},
			check: func(t *testing.T, p entity.Patch) {
				t.Helper()
				if p.Properties["status"] != "done" || len(p.Properties) != 1 {
					t.Errorf("Properties = %v, want only status=done", p.Properties)
				}
				if len(p.MetaUnset) != 1 || p.MetaUnset[0] != "title" {
					t.Errorf("MetaUnset = %v, want [title]", p.MetaUnset)
				}
				if p.Content != nil || p.WriteExternalRefs {
					t.Errorf("patch carries content or external refs: %+v", p)
				}
			},
		},
		{
			name: "no other admin methods",
			script: `rela.bypass_acl(function(admin)
				for _, m in ipairs({"get_entity", "list_entities", "get_relations",
					"create_relation", "delete_relation", "delete_entity"}) do
					assert(admin[m] == nil, m .. " present")
				end
			end)`,
		},
		{
			name:    "content argument refused",
			script:  `rela.bypass_acl(function(admin) admin.update_entity("TKT-001", {status = "done"}, "body") end)`,
			wantErr: "content and options are not supported",
		},
		{
			name:    "empty id refused",
			script:  `rela.bypass_acl(function(admin) admin.update_entity("", {status = "done"}) end)`,
			wantErr: "entity ID cannot be empty",
		},
		{
			name:    "unknown target",
			script:  `rela.bypass_acl(function(admin) admin.update_entity("TKT-999", {status = "done"}) end)`,
			wantErr: "entity not found: TKT-999",
		},
		{
			name:    "manager not-found maps to entity not found",
			script:  `rela.bypass_acl(function(admin) admin.update_entity("TKT-001", {status = "done"}) end)`,
			err:     notFoundError{},
			wantErr: "entity not found: TKT-001",
		},
		{
			name:    "manager error surfaces",
			script:  `rela.bypass_acl(function(admin) admin.update_entity("TKT-001", {status = "done"}) end)`,
			err:     errors.New(`property "secret" of "ticket" is outside the scope`),
			wantErr: "outside the scope",
		},
		{
			name: "invalidated after the closure",
			script: `local stashed
				rela.bypass_acl(function(admin) stashed = admin end)
				stashed.update_entity("TKT-001", {status = "done"})`,
			wantErr: "invalidated",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := &recordingUpdater{ws: newMockWorkspace(t), err: tc.err}
			r := writerWithUpdater(t, u)
			defer r.Close()
			err := r.RunString(tc.script)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				if tc.err == nil && len(u.ids) != 0 {
					t.Errorf("updater called %v on a refused call", u.ids)
				}
				return
			}
			if err != nil {
				t.Fatalf("script: %v", err)
			}
			if strings.Join(u.ids, ",") != strings.Join(tc.wantIDs, ",") {
				t.Errorf("updated ids = %v, want %v", u.ids, tc.wantIDs)
			}
			if tc.check != nil {
				tc.check(t, u.patches[0])
			}
		})
	}
}

// TestBypassACL_NoUpdateEntityWithoutUpdater pins that the automation-action
// handle (elevated manager and reader) has no update_entity.
func TestBypassACL_NoUpdateEntityWithoutUpdater(t *testing.T) {
	t.Parallel()
	r := writerWithElevated(t, &recordingMutator{})
	defer r.Close()
	if err := r.RunString(`rela.bypass_acl(function(admin)
		assert(admin.update_entity == nil, "update_entity present")
	end)`); err != nil {
		t.Fatal(err)
	}
}
