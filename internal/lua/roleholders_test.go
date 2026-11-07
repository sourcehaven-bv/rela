package lua

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

type fakeRoleHolders struct {
	ids []string
	err error
}

func (f fakeRoleHolders) PrincipalsWithRole(_ context.Context, role string) (string, []entity.Ref, error) {
	if f.err != nil {
		return "", nil, f.err
	}
	if role != "cpo" {
		return "", nil, errors.New(`"` + role + `": role is not declared in acl.yaml`)
	}
	refs := make([]entity.Ref, 0, len(f.ids))
	for _, id := range f.ids {
		refs = append(refs, entity.Ref{ID: id, Face: entity.ImplicitFace})
	}
	return "person", refs, nil
}

// gateDropReader withholds one id from batch header reads, standing in for
// the row gate.
type gateDropReader struct {
	EntityReader
	hidden string
}

func (h gateDropReader) ResolveHeadersErr(
	ctx context.Context, refs []entity.Ref,
) (map[entity.Ref]visibility.ResolvedHeader, error) {
	got, err := h.EntityReader.(headerResolver).ResolveHeadersErr(ctx, refs)
	for ref := range got {
		if ref.ID == h.hidden {
			delete(got, ref)
		}
	}
	return got, err
}

func TestPrincipalsWithRole(t *testing.T) {
	st := memstore.New()
	for _, id := range []string{"PER-1", "PER-2", "PER-3"} {
		if err := st.CreateEntity(t.Context(), &entity.Entity{ID: id, Type: "person"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreateEntity(t.Context(), &entity.Entity{ID: "TKT-1", Type: "ticket"}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		holders RoleHolders
		hidden  string
		src     string
		wantErr string
	}{
		{
			name:    "sorted holders",
			holders: fakeRoleHolders{ids: []string{"PER-3", "PER-1"}},
			src:     `local ids = rela.principals_with_role("cpo"); assert(#ids == 2 and ids[1] == "PER-1" and ids[2] == "PER-3")`,
		},
		{
			name:    "holders the script cannot read are dropped",
			holders: fakeRoleHolders{ids: []string{"PER-1", "PER-2"}},
			hidden:  "PER-2",
			src:     `local ids = rela.principals_with_role("cpo"); assert(#ids == 1 and ids[1] == "PER-1")`,
		},
		{
			name:    "no holders",
			holders: fakeRoleHolders{},
			src:     `assert(#rela.principals_with_role("cpo") == 0)`,
		},
		{
			name:    "an id of another type is dropped",
			holders: fakeRoleHolders{ids: []string{"TKT-1", "PER-2"}},
			src:     `local ids = rela.principals_with_role("cpo"); assert(#ids == 1 and ids[1] == "PER-2")`,
		},
		{
			name:    "a stored id with no entity is dropped",
			holders: fakeRoleHolders{ids: []string{"PER-9"}},
			src:     `assert(#rela.principals_with_role("cpo") == 0)`,
		},
		{
			name:    "unknown role raises",
			holders: fakeRoleHolders{},
			src:     `rela.principals_with_role("admin")`,
			wantErr: "not declared",
		},
		{
			name:    "store failure raises",
			holders: fakeRoleHolders{err: errors.New("idproles: read failed")},
			src:     `rela.principals_with_role("cpo")`,
			wantErr: "read failed",
		},
		{
			name:    "unavailable without the capability",
			src:     `rela.principals_with_role("cpo")`,
			wantErr: "not available in this context",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rd := unrestricted(st)
			if tc.hidden != "" {
				rd = gateDropReader{EntityReader: unrestricted(st), hidden: tc.hidden}
			}
			var buf bytes.Buffer
			r := NewReader(ReadDeps{VisibleReader: rd, World: store.TrivialScope(), RoleHolders: tc.holders}, &buf)
			defer r.Close()
			err := r.RunString(tc.src)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("RunString: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}
