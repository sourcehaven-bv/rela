package visibility

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func headerRefs(rows []store.EntityHeader) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = entity.FormatStateRef(r.ID, r.Face)
	}
	return out
}

// TestHolderLookup_Headers pins the gated header query: the caller's
// predicates narrow the rows the read gate admits, with and without
// pushdown, and a value redaction hides never selects a row.
func TestHolderLookup_Headers(t *testing.T) {
	f := seedHolders(t, memstore.New())
	redact := hideOwnerOf{"POL-4": true}
	byOwner := HeaderQuery{EntityType: "policy", Equal: map[string]string{"owner": holderID}, Faces: store.AllFaces(),
		Limit: 10}
	anyOwner := HeaderQuery{EntityType: "policy", Faces: store.AllFaces(), Limit: 10,
		AnyOf: []map[string]string{{"owner": holderID}, {"owner": "USR-70"}}}
	for _, tc := range []struct {
		name string
		user string
		q    HeaderQuery
		want []string
	}{
		{"bob equal", "bob", byOwner, []string{"POL-1", "POL-2@draft"}},
		{"carol equal", "carol", byOwner, []string{}},
		{"dave equal", "dave", byOwner, []string{}},
		// POL-4 matches in the store, but its owner is redacted for carol.
		{"carol any of", "carol", anyOwner, []string{"POL-6@published"}},
		{"bob any of", "bob", anyOwner, []string{"POL-1", "POL-2@draft", "POL-6@published"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := acltest.Bind(principal.With(context.Background(),
				principal.Principal{User: tc.user, Tool: principal.ToolDataEntry}), t, f.d)
			for _, mode := range []bool{true, false} {
				h, err := NewHolderLookup(f.scriptReaderFor(t, redact, mode))
				if err != nil {
					t.Fatal(err)
				}
				rows, more, err := h.Headers(ctx, tc.q)
				if err != nil {
					t.Fatalf("pushdown %v: %v", mode, err)
				}
				if got := headerRefs(rows); !slices.Equal(got, tc.want) || more {
					t.Errorf("pushdown %v: got %v (more %v), want %v", mode, got, more, tc.want)
				}
			}
		})
	}

	t.Run("limit reports more", func(t *testing.T) {
		ctx := acltest.Bind(principal.With(context.Background(),
			principal.Principal{User: "bob", Tool: principal.ToolDataEntry}), t, f.d)
		h, _ := NewHolderLookup(f.scriptReaderFor(t, redact, true))
		q := byOwner
		q.Limit = 1
		rows, more, err := h.Headers(ctx, q)
		if err != nil || len(rows) != 1 || !more {
			t.Fatalf("got %v more %v err %v", headerRefs(rows), more, err)
		}
	})

	// The order picks which rows the limit keeps, so a cut keeps the top of
	// the order rather than the lowest ids.
	t.Run("order picks the rows the limit keeps", func(t *testing.T) {
		ctx := acltest.Bind(principal.With(context.Background(),
			principal.Principal{User: "bob", Tool: principal.ToolDataEntry}), t, f.d)
		for _, tc := range []struct {
			desc bool
			want []string
		}{
			{true, []string{"POL-6@published", "POL-1"}},
			{false, []string{"POL-1", "POL-2@draft"}},
		} {
			for _, mode := range []bool{true, false} {
				h, _ := NewHolderLookup(f.scriptReaderFor(t, redact, mode))
				q := anyOwner
				q.Limit = 2
				q.Order = []store.OrderSpec{{Property: "owner", Descending: tc.desc}}
				rows, more, err := h.Headers(ctx, q)
				if err != nil {
					t.Fatal(err)
				}
				if got := headerRefs(rows); !slices.Equal(got, tc.want) || !more {
					t.Errorf("desc %v pushdown %v: got %v (more %v), want %v", tc.desc, mode, got, more, tc.want)
				}
			}
		}
	})

	t.Run("unrestricted and refusing", func(t *testing.T) {
		h, _ := NewUnrestrictedHolderLookup(f.st)
		rows, _, err := h.Headers(context.Background(), byOwner)
		if err != nil {
			t.Fatal(err)
		}
		if got := headerRefs(rows); !slices.Equal(got, []string{"POL-1", "POL-2@draft", "POL-4@published"}) {
			t.Errorf("unrestricted = %v", got)
		}
		_, _, err = NewRefusingHolderLookup().Headers(context.Background(), byOwner)
		if !errors.Is(err, ErrReaderUnavailable) {
			t.Errorf("refusing = %v", err)
		}
		if _, _, err := h.Headers(context.Background(), HeaderQuery{EntityType: "policy"}); err == nil {
			t.Error("a zero limit passed")
		}
	})
}
