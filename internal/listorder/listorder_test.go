package listorder

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func newService(t *testing.T) (*Service, state.KV) {
	t.Helper()
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/root")
	if err != nil {
		t.Fatal(err)
	}
	kv := state.NewFSKV(rfs)
	s, err := New(kv)
	if err != nil {
		t.Fatal(err)
	}
	return s, kv
}

func as(user string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolCLI})
}

func TestNew_RejectsNilKV(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("want error")
	}
}

func TestPlaced(t *testing.T) {
	full := []string{"a", "b", "c", "d"}
	tests := []struct {
		name          string
		old           []string
		id, bef, aftr string
		want          []string
		err           error
	}{
		{"to the top", nil, "c", "a", "", []string{"c"}, nil},
		{"after a", nil, "d", "", "a", []string{"a", "d"}, nil},
		{"to the bottom", nil, "a", "", "d", []string{"b", "c", "d", "a"}, nil},
		{"before itself", nil, "a", "a", "", nil, ErrBadMove},
		{"both neighbors", nil, "a", "b", "c", nil, ErrBadMove},
		{"no neighbor", nil, "a", "", "", nil, ErrBadMove},
		{"keeps placements below", []string{"a", "b"}, "d", "a", "", []string{"d", "a", "b"}, nil},
		{"keeps a placement below", []string{"a", "e"}, "d", "a", "", []string{"d", "a", "e"}, nil},
		{"keeps a filtered id", []string{"a", "x", "b"}, "c", "a", "", []string{"c", "a", "x", "b"}, nil},
		{"filtered id first", []string{"x", "a"}, "b", "", "a", []string{"x", "a", "b"}, nil},
		{"row not in list", nil, "x", "a", "", nil, ErrNotFound},
		{"neighbor not in list", nil, "a", "x", "", nil, ErrNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Placed(full, tc.old, tc.id, tc.bef, tc.aftr)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPlaced_PastTheCap(t *testing.T) {
	full := make([]string, MaxIDs+10)
	for i := range full {
		full[i] = fmt.Sprintf("r%d", i)
	}
	last := full[len(full)-1]
	got, err := Placed(full, nil, last, "", full[MaxIDs+5])
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MaxIDs || got[MaxIDs-1] != last {
		t.Fatalf("len %d, last %q", len(got), got[len(got)-1])
	}
}

func TestApply(t *testing.T) {
	rows := []string{"a", "b", "c", "d", "e"}
	got := Apply(rows, []string{"d", "gone", "b"})
	ids := make([]string, len(got))
	for i, j := range got {
		ids[i] = rows[j]
	}
	if want := []string{"d", "b", "a", "c", "e"}; !slices.Equal(ids, want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
}

func TestService_RoundTripPerOwner(t *testing.T) {
	s, _ := newService(t)
	full := []string{"a", "b", "c"}
	if err := s.Move(as("alice"), "todo", full, "c", "a", ""); err != nil {
		t.Fatal(err)
	}
	got, err := s.Order(as("alice"), "todo")
	if err != nil || !slices.Equal(got, []string{"c"}) {
		t.Fatalf("alice: %v %v", got, err)
	}
	for _, ctx := range []context.Context{as("bob"), as("alice")} {
		other, err := s.Order(ctx, "other")
		if err != nil || len(other) != 0 {
			t.Fatalf("other list: %v %v", other, err)
		}
	}
	if bob, _ := s.Order(as("bob"), "todo"); len(bob) != 0 {
		t.Fatalf("bob sees %v", bob)
	}
}

func TestService_NoOwner(t *testing.T) {
	s, _ := newService(t)
	if _, err := s.Order(context.Background(), "todo"); !errors.Is(err, ErrNoOwner) {
		t.Fatalf("got %v", err)
	}
	if err := s.Move(as(principal.Unknown), "todo", []string{"a", "b"}, "a", "", "b"); !errors.Is(err, ErrNoOwner) {
		t.Fatalf("got %v", err)
	}
}

func TestService_KeyHidesNames(t *testing.T) {
	k := keyFor("../../alice", "list/../x")
	if err := state.ValidateKey(k); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(k, "alice") || strings.Contains(k, "..") {
		t.Fatalf("key leaks names: %s", k)
	}
}

func TestService_CorruptDocumentIsEmpty(t *testing.T) {
	s, kv := newService(t)
	if err := kv.Put(context.Background(), keyFor("alice", "todo"), []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Order(as("alice"), "todo")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestClean(t *testing.T) {
	got := clean([]string{"a", "", "a", "b\x00", strings.Repeat("x", MaxIDBytes+1), "c"})
	if !slices.Equal(got, []string{"a", "c"}) {
		t.Fatalf("got %v", got)
	}
}

func TestService_ConcurrentMoves(t *testing.T) {
	s, _ := newService(t)
	full := []string{"a", "b", "c", "d"}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			_ = s.Move(as("alice"), "todo", full, full[i%4], "", full[(i+1)%4])
		})
	}
	wg.Wait()
	if _, err := s.Order(as("alice"), "todo"); err != nil {
		t.Fatal(err)
	}
}
