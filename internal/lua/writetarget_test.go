package lua

import (
	"context"
	"strings"
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
	// A reader without Family is refused outright: every gated reader
	// provides it, so its absence is a wiring bug, not a reason to read more.
	readers := map[string]EntityReader{
		"family":    visibility.Unrestricted(st),
		"no family": rawAddressReader{st},
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
			want := tc.want
			if name == "no family" && !strings.Contains(tc.addr, "@") {
				want = false
			}
			if got := writeTargetReadable(ctx, rd, tc.addr); got != want {
				t.Errorf("%s: writeTargetReadable(%q) = %v, want %v", name, tc.addr, got, want)
			}
		}
	}
}

// rawAddressReader is an ungated EntityReader without Family: it reads an
// address with readAddress.
type rawAddressReader struct{ store.Store }

func (r rawAddressReader) GetAddress(ctx context.Context, addr string) (*entity.Entity, error) {
	return readAddress(ctx, r.Store, addr)
}

// readAddress loads the row an address (`ID` or `ID@face`) names. An address
// that does not parse is ErrNotFound, as it is for the store.
func readAddress(ctx context.Context, s store.Store, addr string) (*entity.Entity, error) {
	ref, err := entity.ParseRef(addr)
	if err != nil {
		return nil, store.ErrNotFound
	}
	return s.GetEntity(ctx, ref)
}
