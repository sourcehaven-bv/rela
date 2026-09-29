package lua

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestWriteTargetReadable pins the write gate for faced entities: a bare id
// is an entity-level name and passes when some face is readable, while a
// named face must itself exist.
func TestWriteTargetReadable(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft"},
		{ID: "TKT-1", Type: "ticket"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.Ref(), err)
		}
	}
	// The first reader answers through Family; the raw store has no Family
	// and takes the list fallback. Both must agree.
	readers := map[string]EntityReader{
		"family":        visibility.Unrestricted(st),
		"list fallback": rawAddressReader{st},
	}
	for name, rd := range readers {
		for _, tc := range []struct {
			addr string
			want bool
		}{
			{"POL-1", true},
			{"POL-1@draft", true},
			{"POL-1@published", false},
			{"TKT-1", true},
			{"NOPE-1", false},
			{"POL-1@", false},
			{"", false},
		} {
			if got := writeTargetReadable(ctx, rd, tc.addr); got != tc.want {
				t.Errorf("%s: writeTargetReadable(%q) = %v, want %v", name, tc.addr, got, tc.want)
			}
		}
	}
}

// rawAddressReader is an ungated EntityReader without Family: it reads an
// address with store.GetEntityAt.
type rawAddressReader struct{ store.Store }

func (r rawAddressReader) GetEntity(ctx context.Context, addr string) (*entity.Entity, error) {
	return store.GetEntityAt(ctx, r.Store, addr)
}
