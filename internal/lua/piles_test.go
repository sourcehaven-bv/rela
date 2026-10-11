package lua

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// fakePiles is an in-memory piles capability. It records pushes and removes
// so a test asserts what reached the boundary.
type fakePiles struct {
	piles   map[string]PileSummary
	pushes  []PilePush
	removed map[string][]entity.Ref
	pushErr error
}

func newFakePiles(t *testing.T, ps ...PileSummary) (*fakePiles, *PileFuncs) {
	t.Helper()
	f := &fakePiles{piles: map[string]PileSummary{}, removed: map[string][]entity.Ref{}}
	for _, p := range ps {
		f.piles[p.Name] = p
	}
	funcs, err := NewPileFuncs(PileFuncs{
		List: func(context.Context) ([]PileSummary, error) {
			out := make([]PileSummary, 0, len(f.piles))
			for _, p := range ps {
				out = append(out, f.piles[p.Name])
			}
			return out, nil
		},
		Find: func(_ context.Context, name string) (PileSummary, error) {
			p, ok := f.piles[name]
			if !ok {
				return PileSummary{}, errors.New("piles: pile not found")
			}
			return p, nil
		},
		Push: func(_ context.Context, p PilePush) (int, error) {
			if f.pushErr != nil {
				return 0, f.pushErr
			}
			f.pushes = append(f.pushes, p)
			return len(p.Refs), nil
		},
		Remove: func(_ context.Context, id string, refs []entity.Ref) error {
			f.removed[id] = append(f.removed[id], refs...)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewPileFuncs: %v", err)
	}
	return f, funcs
}

// pileStore seeds a faceless ticket, a faced policy with one face and a
// faced policy with two faces.
func pileStore(t *testing.T) store.Store {
	t.Helper()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "One"}},
		{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "Two"}},
		{ID: "POL-1", Type: "policy", Face: "draft"},
		{ID: "POL-2", Type: "policy", Face: "draft"},
		{ID: "POL-2", Type: "policy", Face: "published"},
	} {
		if err := st.CreateEntity(context.Background(), e); err != nil {
			t.Fatalf("seed %s: %v", e.Ref(), err)
		}
	}
	return st
}

func runPileScript(t *testing.T, rd EntityReader, pf *PileFuncs, writes bool, src string) error {
	t.Helper()
	var buf bytes.Buffer
	var r *Runtime
	read := ReadDeps{VisibleReader: rd, World: store.TrivialScope()}
	if pf != nil {
		read.Piles = pf
	}
	if writes {
		d := WriteDeps{ReadDeps: read, EntityManager: &mockManager{}}
		if pf != nil {
			d.PileWriter = pf
		}
		r = NewWriter(d, &buf)
	} else {
		r = NewReader(read, &buf)
	}
	defer r.Close()
	return r.RunString(src)
}

func unrestricted(st store.Store) EntityReader {
	return visibility.Unrestricted(st).WithWorld(visibility.WorldOf(store.TrivialScope()))
}

func TestPiles_AddResolvesAddresses(t *testing.T) {
	f, pf := newFakePiles(t)
	err := runPileScript(t, unrestricted(pileStore(t)), pf, true, `
local e = rela.get_entity("TKT-2")
local n = rela.piles.add{pile = "Inbox", entities = {"TKT-1", "POL-1@draft", e}, owner = "PER-1"}
assert(n == 3, "added " .. tostring(n))
`)
	if err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if len(f.pushes) != 1 {
		t.Fatalf("pushes = %d, want 1", len(f.pushes))
	}
	got := f.pushes[0]
	if got.Owner != "PER-1" || got.Pile != "Inbox" || !got.Create {
		t.Errorf("push = %+v, want owner PER-1, pile Inbox, create true", got)
	}
	want := []string{"TKT-1", "POL-1@draft", "TKT-2"}
	if len(got.Refs) != len(want) {
		t.Fatalf("refs = %v, want %v", got.Refs, want)
	}
	for i, ref := range got.Refs {
		if ref.String() != want[i] {
			t.Errorf("refs[%d] = %s, want %s", i, ref, want[i])
		}
	}
}

func TestPiles_AddCreateFalse(t *testing.T) {
	f, pf := newFakePiles(t)
	if err := runPileScript(t, unrestricted(pileStore(t)), pf, true,
		`rela.piles.add{pile = "Inbox", entities = {"TKT-1"}, create = false}`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if f.pushes[0].Create {
		t.Error("create = false did not reach the push")
	}
}

func TestPiles_AddErrors(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{
		{"unknown id", `rela.piles.add{pile = "P", entities = {"NOPE-1"}}`, "entity not found: NOPE-1"},
		{"missing pile", `rela.piles.add{entities = {"TKT-1"}}`, "pile (a pile name) is required"},
		{"missing entities", `rela.piles.add{pile = "P"}`, "entities (an array of ids or entities) is required"},
		{"bad element", `rela.piles.add{pile = "P", entities = {42}}`, "expected an id or an entity table"},
		{"bad owner", `rela.piles.add{pile = "P", entities = {"TKT-1"}, owner = 1}`, "owner must be a string"},
		{"bad create", `rela.piles.add{pile = "P", entities = {"TKT-1"}, create = "yes"}`, "create must be a boolean"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, pf := newFakePiles(t)
			err := runPileScript(t, unrestricted(pileStore(t)), pf, true, tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
			if len(f.pushes) != 0 {
				t.Errorf("a failed add pushed %v", f.pushes)
			}
		})
	}
}

func TestPiles_AddAmbiguityNamesFaces(t *testing.T) {
	_, pf := newFakePiles(t)
	err := runPileScript(t, unrestricted(pileStore(t)), pf, true,
		`rela.piles.add{pile = "P", entities = {"POL-2"}}`)
	if err == nil || !strings.Contains(err.Error(), "draft") || !strings.Contains(err.Error(), "published") {
		t.Fatalf("err = %v, want the ambiguity error naming both faces", err)
	}
}

func TestPiles_PushErrorRaises(t *testing.T) {
	f, pf := newFakePiles(t)
	f.pushErr = errors.New("piles: unknown owner: \"X\"")
	err := runPileScript(t, unrestricted(pileStore(t)), pf, true,
		`rela.piles.add{pile = "P", entities = {"TKT-1"}, owner = "X"}`)
	if err == nil || !strings.Contains(err.Error(), "unknown owner") {
		t.Fatalf("err = %v, want the push error", err)
	}
}

func TestPiles_Remove(t *testing.T) {
	pol2Draft, _ := entity.ParseRef("POL-2@draft")
	pol2Pub, _ := entity.ParseRef("POL-2@published")
	tkt1, _ := entity.ParseRef("TKT-1")
	f, pf := newFakePiles(t, PileSummary{ID: "PIL-AAAA", Name: "Mine", Items: []entity.Ref{tkt1, pol2Draft, pol2Pub}})
	if err := runPileScript(t, unrestricted(pileStore(t)), pf, true,
		`assert(rela.piles.remove{pile = "Mine", entities = {"POL-2", "GONE-1", {id = "TKT-1", face = ""}}})`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	got := f.removed["PIL-AAAA"]
	want := []entity.Ref{pol2Draft, pol2Pub, tkt1}
	if len(got) != len(want) {
		t.Fatalf("removed = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("removed[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestPiles_RemoveNamedFaceOnly(t *testing.T) {
	pol2Draft, _ := entity.ParseRef("POL-2@draft")
	pol2Pub, _ := entity.ParseRef("POL-2@published")
	f, pf := newFakePiles(t, PileSummary{ID: "PIL-AAAA", Name: "Mine", Items: []entity.Ref{pol2Draft, pol2Pub}})
	if err := runPileScript(t, unrestricted(pileStore(t)), pf, true,
		`rela.piles.remove{pile = "Mine", entities = {"POL-2@published"}}`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if got := f.removed["PIL-AAAA"]; len(got) != 1 || got[0] != pol2Pub {
		t.Fatalf("removed = %v, want only POL-2@published", got)
	}
}

func TestPiles_ListAndItemsAreReadableOnly(t *testing.T) {
	tkt1, _ := entity.ParseRef("TKT-1")
	tkt2, _ := entity.ParseRef("TKT-2")
	pol1, _ := entity.ParseRef("POL-1@draft")
	gone, _ := entity.ParseRef("GONE-1")
	_, pf := newFakePiles(t,
		PileSummary{ID: "PIL-AAAA", Name: "Mine", Icon: "star", Items: []entity.Ref{tkt2, gone, pol1, tkt1}},
		PileSummary{ID: "PIL-BBBB", Name: "Empty", Icon: "layers"},
	)
	// The reader hides TKT-2: it is on the pile but must not be listed or
	// counted.
	rd := hidingReader{EntityReader: unrestricted(pileStore(t)), hidden: "TKT-2"}
	err := runPileScript(t, rd, pf, false, `
local ps = rela.piles.list()
assert(#ps == 2, "piles " .. #ps)
assert(ps[1].id == "PIL-AAAA" and ps[1].name == "Mine" and ps[1].icon == "star", "pile 1")
assert(ps[1].count == 2, "count " .. ps[1].count)
assert(ps[2].count == 0, "empty count")
local items = rela.piles.items("Mine")
local got = {}
for _, it in ipairs(items) do got[#got + 1] = it.address .. "|" .. it.type .. "|" .. it.face .. "|" .. it.title end
local s = table.concat(got, ",")
assert(s == "POL-1@draft|policy|draft|POL-1,TKT-1|ticket||TKT-1", s)
`)
	if err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestPiles_ReaderRuntimeHasNoWrites(t *testing.T) {
	_, pf := newFakePiles(t)
	err := runPileScript(t, unrestricted(pileStore(t)), pf, false,
		`rela.piles.add{pile = "P", entities = {"TKT-1"}}`)
	if err == nil || !strings.Contains(err.Error(), "non-function") {
		t.Fatalf("err = %v, want add to be absent on a reader runtime", err)
	}
}

func TestPiles_UnavailableWithoutCapability(t *testing.T) {
	for _, src := range []string{
		`rela.piles.list()`,
		`rela.piles.items("P")`,
		`rela.piles.add{pile = "P", entities = {"TKT-1"}}`,
		`rela.piles.remove{pile = "P", entities = {"TKT-1"}}`,
	} {
		err := runPileScript(t, unrestricted(pileStore(t)), nil, true, src)
		if err == nil || !strings.Contains(err.Error(), "piles are not available in this context") {
			t.Errorf("%s: err = %v, want the unavailable error", src, err)
		}
	}
}

func TestNewPileFuncs_RejectsNil(t *testing.T) {
	if _, err := NewPileFuncs(PileFuncs{}); err == nil {
		t.Fatal("NewPileFuncs accepted missing functions")
	}
}

// hidingReader hides one id from the batched header read, as a gated reader
// would.
type hidingReader struct {
	EntityReader
	hidden string
}

func (h hidingReader) ResolveHeadersErr(
	ctx context.Context, refs []entity.Ref,
) (map[entity.Ref]visibility.ResolvedHeader, error) {
	out, err := h.EntityReader.(headerResolver).ResolveHeadersErr(ctx, refs)
	for ref := range out {
		if ref.ID == h.hidden {
			delete(out, ref)
		}
	}
	return out, err
}

// failingHeaders is a reader whose batched header read fails, as a store
// fault would.
type failingHeaders struct{ EntityReader }

var errHeaderRead = errors.New("header read failed")

func (failingHeaders) ResolveHeadersErr(
	context.Context, []entity.Ref,
) (map[entity.Ref]visibility.ResolvedHeader, error) {
	return nil, errHeaderRead
}

// A failed header read raises: it is never answered as an empty pile.
func TestPiles_HeaderReadFailureRaises(t *testing.T) {
	tkt1, _ := entity.ParseRef("TKT-1")
	for _, tc := range []struct{ name, src string }{
		{"list", `rela.piles.list()`},
		{"items", `rela.piles.items("Mine")`},
		{"add", `rela.piles.add{pile = "Mine", entities = {"TKT-1"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, pf := newFakePiles(t, PileSummary{ID: "PIL-AAAA", Name: "Mine", Items: []entity.Ref{tkt1}})
			rd := failingHeaders{EntityReader: unrestricted(pileStore(t))}
			err := runPileScript(t, rd, pf, true, tc.src)
			if err == nil || !strings.Contains(err.Error(), errHeaderRead.Error()) {
				t.Fatalf("err = %v, want the header read failure", err)
			}
			if len(f.pushes) != 0 {
				t.Errorf("a failed read pushed %v", f.pushes)
			}
		})
	}
}

// rela.piles.add resolves its entities with the same number of store reads
// whatever their number: one batch, never a read per entity.
func TestPiles_AddReadBudget(t *testing.T) {
	reads := make([]int, 0, 2)
	for _, n := range []int{10, 50} {
		st := memstore.New()
		ids := make([]string, n)
		for i := range ids {
			ids[i] = fmt.Sprintf("TKT-%04d", i+1)
			if err := st.CreateEntity(context.Background(), &entity.Entity{ID: ids[i], Type: "ticket"}); err != nil {
				t.Fatal(err)
			}
		}
		counting := storetest.NewCounting(st)
		f, pf := newFakePiles(t)
		src := `rela.piles.add{pile = "P", entities = {"` + strings.Join(ids, `", "`) + `"}}`
		counting.Reset()
		if err := runPileScript(t, unrestricted(counting), pf, true, src); err != nil {
			t.Fatalf("RunString: %v", err)
		}
		if len(f.pushes) != 1 || len(f.pushes[0].Refs) != n {
			t.Fatalf("pushes = %v, want one push of %d refs", f.pushes, n)
		}
		reads = append(reads, counting.Reads())
	}
	if reads[0] != reads[1] {
		t.Errorf("reads grew with the entity count: %d at 10, %d at 50", reads[0], reads[1])
	}
}
