package lua_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// historyStore is a memstore with a canned version history per face, keyed
// by the entity's state ref (`ID` or `ID@face`).
type historyStore struct {
	store.Store
	versions map[string][]store.VersionSnapshot
}

func (h historyStore) ListVersions(_ context.Context, ref entity.Ref) ([]store.VersionMeta, error) {
	var out []store.VersionMeta
	for _, s := range h.versions[ref.String()] {
		out = append(out, s.VersionMeta)
	}
	return out, nil
}

func (h historyStore) GetVersion(_ context.Context, ref entity.Ref, n int) (*store.VersionSnapshot, error) {
	vs := h.versions[ref.String()]
	if n < 1 || n > len(vs) {
		return nil, store.ErrNotFound
	}
	return &vs[n-1], nil
}

func snapshot(n int, typ string, props map[string]any) store.VersionSnapshot {
	return store.VersionSnapshot{
		VersionMeta: store.VersionMeta{
			Version: n, Op: store.VersionOpUpdate, Type: typ,
			ContentHash: "hash" + strconv.Itoa(n), PrincipalUser: "bob",
			CreatedAt: time.Date(2026, 1, n, 0, 0, 0, 0, time.UTC),
		},
		Content:    "body " + strconv.Itoa(n),
		Properties: props,
	}
}

func otherFace(s store.VersionSnapshot) store.VersionSnapshot {
	s.Face = "draft"
	return s
}

func purged(s store.VersionSnapshot) store.VersionSnapshot {
	s.Op = store.VersionOpPurge
	return s
}

// newHistoryWorld is the ACL fixture over a store with history: P-1 has two
// versions, the hidden SEC-1 one, and TKT-1's lineage holds a version of
// another type, one of another face and a purge row.
func newHistoryWorld(t *testing.T) lua.WriteDeps {
	t.Helper()
	st := historyStore{Store: memstore.New(), versions: map[string][]store.VersionSnapshot{
		"P-1": {
			snapshot(1, "person", map[string]any{"name": "Ann", "salary": "90000"}),
			snapshot(2, "person", map[string]any{"name": "Ann B", "salary": "100000"}),
		},
		"SEC-1": {snapshot(1, "secret", map[string]any{"title": "Classified"})},
		"TKT-1": {
			snapshot(1, "ticket", map[string]any{"title": "Visible"}),
			snapshot(2, "secret", map[string]any{"title": "Other lineage"}),
			otherFace(snapshot(3, "ticket", map[string]any{"title": "Other face"})),
			purged(snapshot(4, "ticket", nil)),
		},
	}}
	_, deps := newACLWorldOn(t, st)
	return deps
}

// runAsAliceErr runs src as alice and returns the script error.
func runAsAliceErr(deps lua.WriteDeps, src string) error {
	var out strings.Builder
	ctx := principal.With(context.Background(), principal.Principal{
		User: "alice", Tool: principal.ToolScheduler,
	})
	rt := lua.NewWriter(deps, &out, lua.WithContext(ctx), lua.WithPrincipal(principal.From(ctx)))
	defer rt.Close()
	return rt.RunString(src)
}

func TestHistory_Timeline(t *testing.T) {
	deps := newHistoryWorld(t)
	out := runAsAlice(t, deps, `
local h = rela.history("P-1")
rela.output("n=" .. #h)
local v = h[2]
rela.output(v.version .. " " .. v.op .. " " .. v.type .. " " .. v.user .. " " .. v.created_at)
rela.output("hash=" .. tostring(v.content_hash))
rela.output("face=[" .. v.face .. "]")
`)
	for _, want := range []string{"n=2", "2 update person bob 2026-01-02T00:00:00Z", "face=[]",
		// The hash covers the unredacted snapshot, so it must not reach a
		// script that cannot see every field.
		"hash=nil",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}
}

func TestHistory_GetVersionRedacts(t *testing.T) {
	deps := newHistoryWorld(t)
	out := runAsAlice(t, deps, `
local e = rela.get_version("P-1", 1)
rela.output("v=" .. e.version .. " id=" .. e.id .. " type=" .. e.type)
rela.output("name=" .. tostring(e.properties.name))
rela.output("salary=" .. tostring(e.properties.salary))
rela.output("content=" .. e.content)
rela.output("mod=" .. e.mod_time)
`)
	for _, want := range []string{
		"v=1 id=P-1 type=person", "name=Ann", "salary=nil", "content=body 1", "mod=2026-01-01T00:00:00Z",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}
}

// TestHistory_MissesAnswerNil covers every case that must look like a
// missing entity: a hidden one, an absent one, an absent version, a snapshot
// of another type or face than the live row, and a purge row. A version
// number past the int32 range raises.
func TestHistory_MissesAnswerNil(t *testing.T) {
	deps := newHistoryWorld(t)
	out := runAsAlice(t, deps, `
rela.output("hidden_history=" .. tostring(rela.history("SEC-1")))
rela.output("hidden_version=" .. tostring(rela.get_version("SEC-1", 1)))
rela.output("absent_history=" .. tostring(rela.history("NOPE-1")))
rela.output("absent_version=" .. tostring(rela.get_version("NOPE-1", 1)))
rela.output("no_such_version=" .. tostring(rela.get_version("P-1", 99)))
rela.output("other_type=" .. tostring(rela.get_version("TKT-1", 2)))
rela.output("other_face=" .. tostring(rela.get_version("TKT-1", 3)))
rela.output("purged=" .. tostring(rela.get_version("TKT-1", 4)))
rela.output("too_large=" .. tostring(pcall(rela.get_version, "TKT-1", 2^40)))
rela.output("own_type=" .. rela.get_version("TKT-1", 1).properties.title)
`)
	for _, want := range []string{
		"hidden_history=nil", "hidden_version=nil", "absent_history=nil", "absent_version=nil",
		"no_such_version=nil", "other_type=nil", "other_face=nil", "purged=nil", "too_large=false",
		"own_type=Visible",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}
}

func TestHistory_BadVersionRaises(t *testing.T) {
	deps := newHistoryWorld(t)
	for _, n := range []string{"0", "-1", "1.5"} {
		err := runAsAliceErr(deps, `rela.get_version("P-1", `+n+`)`)
		if err == nil || !strings.Contains(err.Error(), "version must be a positive integer") {
			t.Errorf("version %s: err = %v", n, err)
		}
	}
}

// TestHistory_UnsupportedRaises: a store without history raises for an
// existing and a missing id alike, never answering an empty timeline.
func TestHistory_UnsupportedRaises(t *testing.T) {
	_, deps := newACLWorld(t)
	for _, src := range []string{
		`rela.history("TKT-1")`, `rela.history("NOPE-1")`,
		`rela.get_version("TKT-1", 1)`, `rela.get_version("NOPE-1", 1)`,
	} {
		err := runAsAliceErr(deps, src)
		if err == nil || !strings.Contains(err.Error(), store.ErrHistoryUnsupported.Error()) {
			t.Errorf("%s: err = %v", src, err)
		}
	}
}

// TestHistory_ReaderWithoutHistoryRaises: a VisibleReader that does not
// serve history raises like a backend without it.
func TestHistory_ReaderWithoutHistoryRaises(t *testing.T) {
	_, deps := newACLWorld(t)
	deps.VisibleReader = readerOnly{deps.VisibleReader}
	err := runAsAliceErr(deps, `rela.history("TKT-1")`)
	if err == nil || !strings.Contains(err.Error(), store.ErrHistoryUnsupported.Error()) {
		t.Errorf("err = %v", err)
	}
}

// readerOnly hides every method but the EntityReader ones.
type readerOnly struct{ lua.EntityReader }

// TestHistory_UnrestrictedReaderDoesNotRedact: the ungated reader the CLI
// uses serves the whole snapshot.
func TestHistory_UnrestrictedReaderDoesNotRedact(t *testing.T) {
	deps := newHistoryWorld(t)
	st := historyStore{Store: memstore.New(), versions: map[string][]store.VersionSnapshot{
		"P-1": {snapshot(1, "person", map[string]any{"name": "Ann", "salary": "90000"})},
	}}
	if err := st.CreateEntity(context.Background(), &entity.Entity{ID: "P-1", Type: "person"}); err != nil {
		t.Fatal(err)
	}
	deps.VisibleReader = visibility.Unrestricted(st).WithWorld(visibility.WorldOf(store.TrivialScope())).WithHistory(st)
	out := runAsAlice(t, deps, `rela.output("salary=" .. tostring(rela.get_version("P-1", 1).properties.salary))`)
	if !strings.Contains(out, "salary=90000") {
		t.Errorf("output %q", out)
	}
}
