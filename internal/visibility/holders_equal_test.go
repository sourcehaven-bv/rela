package visibility

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// hideHostOf is a FieldRedactor that hides "host" on the listed ids.
type hideHostOf map[string]bool

func (h hideHostOf) HiddenProperties(_ context.Context, e *entity.Entity) map[string]struct{} {
	if e != nil && h[e.ID] {
		return map[string]struct{}{"host": {}}
	}
	return nil
}

// seedServers seeds the rows of the FirstEqual tests and a policy where erin
// reads every server and fred reads none. The ids do not collide with
// [seedHolders], so both can share one store.
func seedServers(t *testing.T, st store.Store) holderFixture {
	t.Helper()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		// A list holding the value holds no value: it never matches.
		{ID: "SRV-1", Type: "server", Properties: map[string]any{"host": []any{"web1"}}},
		// The lowest match, but its host is hidden from gated callers.
		{ID: "SRV-2", Type: "server", Properties: map[string]any{"host": "web1"}},
		{ID: "SRV-3", Type: "server", Properties: map[string]any{"host": "web1", "env": "prod"}},
		{ID: "SRV-5", Type: "server", Properties: map[string]any{"host": "web1", "env": "prod"}},
		{ID: "SRV-6", Type: "server", Properties: map[string]any{"port": 8080}},
		{ID: "SRV-7", Type: "server", Properties: map[string]any{"up": true}},
		// Another type never matches, however low its id.
		{ID: "R-1", Type: "router", Properties: map[string]any{"host": "web1"}},
		{ID: "erin", Type: "user"},
		{ID: "fred", Type: "user"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles:       map[string]acl.RoleDef{"serverreaders": {Read: []string{"server", "router"}}},
		Assignments: map[string]string{"erin": "serverreaders"},
	}, aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	return holderFixture{st: st, d: d}
}

func bindAs(t *testing.T, user string, d *acl.Declarative) context.Context {
	t.Helper()
	ctx := principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolDataEntry})
	return acltest.Bind(ctx, t, d)
}

// TestHolderLookup_FirstEqual pins the ingest find (TKT-GIAGLQ3) on
// memstore; the sqlite and postgres twins run under their tags.
func TestHolderLookup_FirstEqual(t *testing.T) {
	assertFirstEqual(t, memstore.New())
}

// assertFirstEqual seeds st with [seedServers] and checks every mode of
// [HolderLookup.FirstEqual] against it.
func assertFirstEqual(t *testing.T, st store.Store) {
	t.Helper()
	f := seedServers(t, st)
	redact := hideHostOf{"SRV-2": true}
	pushed, err := NewHolderLookup(f.scriptReaderFor(t, redact, true))
	if err != nil {
		t.Fatal(err)
	}
	fallback, err := NewHolderLookup(f.scriptReaderFor(t, redact, false))
	if err != nil {
		t.Fatal(err)
	}
	open, err := NewUnrestrictedHolderLookup(st)
	if err != nil {
		t.Fatal(err)
	}
	faceless := store.InWorld(store.TrivialScope())

	for _, tc := range []struct {
		name   string
		lookup *HolderLookup
		user   string
		want   map[string]string
		found  string // "" for no match
	}{
		{"gated skips a hidden property", pushed, "erin", map[string]string{"host": "web1"}, "SRV-3"},
		{"fallback skips a hidden property", fallback, "erin", map[string]string{"host": "web1"}, "SRV-3"},
		{"unrestricted takes the lowest id", open, "erin", map[string]string{"host": "web1"}, "SRV-2"},
		{"every property", pushed, "erin", map[string]string{"host": "web1", "env": "prod"}, "SRV-3"},
		{"a number by its text form", pushed, "erin", map[string]string{"port": "8080"}, "SRV-6"},
		{"a bool by its text form", pushed, "erin", map[string]string{"up": "true"}, "SRV-7"},
		{"no row holds it", pushed, "erin", map[string]string{"host": "web9"}, ""},
		{"an empty value matches nothing", open, "erin", map[string]string{"env": "", "host": "web1"}, ""},
		{"no read grant", pushed, "fred", map[string]string{"host": "web1"}, ""},
		{"no read grant, fallback", fallback, "fred", map[string]string{"host": "web1"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref, found, err := tc.lookup.FirstEqual(bindAs(t, tc.user, f.d), "server", faceless, tc.want)
			if err != nil {
				t.Fatal(err)
			}
			got := ""
			if found {
				got = ref.String()
			}
			if got != tc.found {
				t.Errorf("FirstEqual = %q, want %q", got, tc.found)
			}
		})
	}
}

func TestHolderLookup_FirstEqualRefuses(t *testing.T) {
	f := seedServers(t, memstore.New())
	ctx := bindAs(t, "erin", f.d)
	faceless := store.InWorld(store.TrivialScope())
	open, err := NewUnrestrictedHolderLookup(f.st)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		lookup *HolderLookup
		typ    string
		faces  store.FaceSelection
		want   map[string]string
		err    error
	}{
		{"no type", open, "", faceless, map[string]string{"host": "web1"}, store.ErrInvalidQuery},
		{"no property", open, "server", faceless, nil, store.ErrInvalidQuery},
		{"no face selection", open, "server", store.FaceSelection{}, map[string]string{"host": "web1"},
			store.ErrInvalidQuery},
		{"refusing", NewRefusingHolderLookup(), "server", faceless, map[string]string{"host": "web1"},
			ErrReaderUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := tc.lookup.FirstEqual(ctx, tc.typ, tc.faces, tc.want); !errors.Is(err, tc.err) {
				t.Errorf("err = %v, want %v", err, tc.err)
			}
		})
	}
}

// TestHolderLookupFor pins the tier each reader maps to: a reader the
// function does not know refuses rather than reading ungated.
func TestHolderLookupFor(t *testing.T) {
	f := seedServers(t, memstore.New())
	ctx := bindAs(t, "erin", f.d)
	want := map[string]string{"host": "web1"}
	faceless := store.InWorld(store.TrivialScope())
	var nilScript *ScriptReader
	for _, tc := range []struct {
		name   string
		reader store.EntityLister
		found  string
		err    error
	}{
		{"script reader is gated", f.scriptReaderFor(t, hideHostOf{"SRV-2": true}, true), "SRV-3", nil},
		{"unrestricted reader is ungated", Unrestricted(f.st), "SRV-2", nil},
		{"deny reader refuses", DenyReader{}, "", ErrReaderUnavailable},
		{"nil script reader refuses", nilScript, "", ErrReaderUnavailable},
		{"nil refuses", nil, "", ErrReaderUnavailable},
		{"unknown reader refuses", f.st, "", ErrReaderUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref, _, err := HolderLookupFor(tc.reader).FirstEqual(ctx, "server", faceless, want)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if tc.err == nil && ref.String() != tc.found {
				t.Errorf("FirstEqual = %q, want %q", ref.String(), tc.found)
			}
		})
	}
}

// TestHolderLookup_FirstEqualCostIsScaleFree pins TKT-GIAGLQ3: the ingest
// find and its read of the chosen row cost the same over a store of 100 and
// of 1,000 rows of the find type. Before it, the find listed every row of
// the type per record.
func TestHolderLookup_FirstEqualCostIsScaleFree(t *testing.T) {
	want := map[string]string{"host": "web1"}
	faceless := store.InWorld(store.TrivialScope())
	storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
		mem := memstore.New()
		for i := range n {
			e := &entity.Entity{
				ID: fmt.Sprintf("PAD-%04d", i), Type: "server", Properties: map[string]any{"host": fmt.Sprintf("pad%d", i)},
			}
			if err := mem.CreateEntity(context.Background(), e); err != nil {
				t.Fatal(err)
			}
		}
		counting := storetest.NewCounting(mem)
		f := seedServers(t, counting)
		sr := f.scriptReaderFor(t, hideHostOf{"SRV-2": true}, true).WithWorld(WorldOf(store.TrivialScope()))
		ctx := bindAs(t, "erin", f.d)
		counting.Reset()
		ref, found, err := HolderLookupFor(sr).FirstEqual(ctx, "server", faceless, want)
		if err != nil || !found || ref.ID != "SRV-3" {
			t.Fatalf("FirstEqual = %v, %v, %v; want SRV-3", ref, found, err)
		}
		if _, err := sr.GetAddress(ctx, ref.String()); err != nil {
			t.Fatal(err)
		}
		return counting.Cost(), counting.String()
	})
}
