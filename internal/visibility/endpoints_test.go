package visibility_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestEndpointsReadable pins the 8.2 split: the head of an edge is entity
// level, the tail of a content-scoped edge is the face it attaches to.
func TestEndpointsReadable(t *testing.T) {
	onlyPublished := map[string]visibility.FaceSet{"policy": visibility.SomeFaces(facePublished)}
	for _, tc := range []struct {
		name string
		gate resolverGate
		rel  *entity.Relation
		want bool
	}{
		{name: "faceless ends", rel: &entity.Relation{From: "TKT-1", To: "FEAT-1"}, want: true},
		{name: "faced head is entity level", rel: &entity.Relation{From: "TKT-1", To: "POL-1"}, want: true},
		{name: "faced head with one readable face", gate: resolverGate{faces: onlyPublished},
			rel: &entity.Relation{From: "TKT-1", To: "POL-1"}, want: true},
		{name: "identity tail on a faced entity", rel: &entity.Relation{From: "POL-1", To: "TKT-1"}, want: true},
		{name: "content tail on a readable face",
			rel: &entity.Relation{From: "POL-1", FromFace: faceDraft, To: "TKT-1"}, want: true},
		{name: "content tail on a hidden face", gate: resolverGate{faces: onlyPublished},
			rel: &entity.Relation{From: "POL-1", FromFace: faceDraft, To: "TKT-1"}},
		{name: "content tail on a face that is not stored",
			rel: &entity.Relation{From: "POL-1", FromFace: "archived", To: "TKT-1"}},
		{name: "row-denied head", gate: resolverGate{deny: map[string]bool{"FEAT-1": true}},
			rel: &entity.Relation{From: "TKT-1", To: "FEAT-1"}},
		{name: "row-denied tail", gate: resolverGate{deny: map[string]bool{"TKT-1": true}},
			rel: &entity.Relation{From: "TKT-1", To: "FEAT-1"}},
		{name: "missing head", rel: &entity.Relation{From: "TKT-1", To: "NOPE-1"}},
		{name: "face gate error hides", gate: resolverGate{faceErr: errors.New("down")},
			rel: &entity.Relation{From: "TKT-1", To: "FEAT-1"}},
		{name: "nil relation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := mustResolver(t, tc.gate, visibility.NopRedactor{}, resolverStore(t))
			rels := []*entity.Relation{tc.rel}
			got := r.EndpointsReadable(context.Background(), rels)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("EndpointsReadable = %v, want [%v]", got, tc.want)
			}
			// The strict form agrees wherever the gate does not fail.
			if tc.gate.faceErr != nil {
				return
			}
			strict, err := r.EndpointsReadableErr(context.Background(), rels)
			if err != nil || len(strict) != 1 || strict[0] != tc.want {
				t.Fatalf("EndpointsReadableErr = %v, %v, want [%v]", strict, err, tc.want)
			}
		})
	}
}

// TestEndpointsReadableErr_ReturnsFaults: a failed header read or a gate
// error is returned, never folded into "unreadable".
func TestEndpointsReadableErr_ReturnsFaults(t *testing.T) {
	rels := []*entity.Relation{{From: "TKT-1", To: "FEAT-1"}}
	for _, tc := range []struct {
		name string
		r    *visibility.Resolver
	}{
		{"header read", mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{},
			failingLoader{err: errors.New("disk")})},
		{"gate", mustResolver(t, resolverGate{faceErr: errors.New("down")}, visibility.NopRedactor{},
			resolverStore(t))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.r.EndpointsReadableErr(context.Background(), rels)
			if err == nil || got != nil {
				t.Fatalf("EndpointsReadableErr = %v, %v, want nil and an error", got, err)
			}
		})
	}
	st := resolverStore(t)
	r := mustResolver(t, resolverGate{}, visibility.NopRedactor{}, st)
	if got, err := r.EndpointsReadableErr(context.Background(), nil); err != nil || len(got) != 0 {
		t.Fatalf("empty input = %v, %v", got, err)
	}
	if st.Reads() != 0 {
		t.Errorf("empty input reads = %s, want none", st)
	}
}

// TestEndpointsReadable_EmptyInput costs no store read.
func TestEndpointsReadable_EmptyInput(t *testing.T) {
	st := resolverStore(t)
	r := mustResolver(t, resolverGate{}, visibility.NopRedactor{}, st)
	if got := r.EndpointsReadable(context.Background(), nil); len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
	if st.Reads() != 0 {
		t.Errorf("reads = %s, want none", st)
	}
}

// TestPolicyReader_FilterRelationsBudget pins RR-S4S8ZG for FilterRelations:
// one header read for every endpoint of every relation, no body, the same at
// 10 and 50 relations.
func TestPolicyReader_FilterRelationsBudget(t *testing.T) {
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			ctx := context.Background()
			base := memstore.New()
			var rels []*entity.Relation
			for i := range n {
				pol := fmt.Sprintf("POL-%03d", i)
				tkt := fmt.Sprintf("TKT-%03d", i)
				for _, e := range []*entity.Entity{
					{ID: pol, Type: "policy", Face: faceDraft},
					{ID: pol, Type: "policy", Face: facePublished},
					{ID: tkt, Type: "ticket"},
				} {
					if err := base.CreateEntity(ctx, e); err != nil {
						t.Fatal(err)
					}
				}
				rels = append(rels,
					&entity.Relation{From: pol, FromFace: faceDraft, Type: "cites", To: tkt},
					&entity.Relation{From: tkt, Type: "governed-by", To: pol})
			}
			st := storetest.NewCounting(base)
			gate := resolverGate{faces: map[string]visibility.FaceSet{"policy": visibility.SomeFaces(facePublished)}}
			pr, err := visibility.NewPolicyReader(gate, visibility.NopRedactor{}, st)
			if err != nil {
				t.Fatal(err)
			}
			got := pr.FilterRelations(ctx, rels)
			if len(got) != n || slices.ContainsFunc(got, func(r *entity.Relation) bool { return r.Type == "cites" }) {
				t.Fatalf("kept %d relations, want the %d identity edges only", len(got), n)
			}
			if calls := st.Calls(); st.Reads() != 1 || calls["ListEntityHeaders"] != 1 {
				t.Errorf("reads = %s, want exactly one ListEntityHeaders", st)
			}
		})
	}
}

// TestScriptReader_Resolves pins that the script read surface is the
// resolver's: explicit faces read, a bare faced id misses in the default
// world (ruling 7.1) and resolves in a configured one, and an address the
// grammar refuses misses (9.3).
func TestScriptReader_Resolves(t *testing.T) {
	st := resolverStore(t)
	pr, err := visibility.NewPolicyReader(resolverGate{}, visibility.NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := visibility.NewScriptReader(pr, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	unr := visibility.Unrestricted(st)
	pub := visibility.WorldOf(publishedWorld())
	type getter func(context.Context, string) (*entity.Entity, error)
	readers := map[string]struct {
		get, world getter
	}{
		"script":       {sr.GetAddress, sr.WithWorld(pub).GetAddress},
		"unrestricted": {unr.GetAddress, unr.WithWorld(pub).GetAddress},
	}
	ctx := context.Background()
	for name, rd := range readers {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name, addr string
				get        getter
				want       entity.Face
				miss       bool
			}{
				{name: "explicit face", addr: "POL-1@draft", get: rd.get, want: faceDraft},
				{name: "faceless bare id", addr: "TKT-1", get: rd.get},
				{name: "faced bare id in the default world", addr: "POL-1", get: rd.get, miss: true},
				{name: "faced bare id in a world", addr: "POL-1", get: rd.world, want: facePublished},
				{name: "grammar-refused address", addr: "POL-1@", get: rd.get, miss: true},
				{name: "empty address", addr: "", get: rd.get, miss: true},
				{name: "missing", addr: "NOPE-1", get: rd.get, miss: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					e, err := tc.get(ctx, tc.addr)
					if tc.miss {
						if !errors.Is(err, store.ErrNotFound) {
							t.Fatalf("got (%v, %v), want ErrNotFound", e, err)
						}
						return
					}
					if err != nil || e == nil || e.Face != tc.want {
						t.Fatalf("got (%v, %v), want face %q", e, err, tc.want)
					}
				})
			}
		})
	}
}

// TestScriptReader_Family reads headers only and answers entity-level
// existence for a bare id; an address with a face is not an entity id.
func TestScriptReader_Family(t *testing.T) {
	st := resolverStore(t)
	gate := resolverGate{faces: map[string]visibility.FaceSet{"policy": visibility.SomeFaces(facePublished)}}
	pr, err := visibility.NewPolicyReader(gate, visibility.NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := visibility.NewScriptReader(pr, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	type familyFn func(context.Context, string) (visibility.Family, bool, error)
	for name, family := range map[string]familyFn{
		"script":       sr.Family,
		"unrestricted": visibility.Unrestricted(st).Family,
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			fam, ok, err := family(ctx, "POL-1")
			if err != nil || !ok || fam.Type != "policy" {
				t.Fatalf("Family(POL-1) = (%+v, %v, %v)", fam, ok, err)
			}
			for _, id := range []string{"POL-1@draft", "NOPE-1", "", "bad id"} {
				if _, ok, err := family(ctx, id); ok || err != nil {
					t.Errorf("Family(%q) = (%v, %v), want a miss", id, ok, err)
				}
			}
		})
	}
	st.Reset()
	if _, _, err := sr.Family(context.Background(), "POL-1"); err != nil {
		t.Fatal(err)
	}
	if calls := st.Calls(); calls["ListEntities"]+calls["GetEntity"] != 0 {
		t.Errorf("Family loaded a body: %s", st)
	}
}

// TestScriptReader_GateErrorIsAMiss pins that an untyped read answers a gate
// failure like a miss. The gate only runs for an id the header read found,
// so a returned error would tell an existing id from a missing one.
func TestScriptReader_GateErrorIsAMiss(t *testing.T) {
	buf := captureWarn(t)
	st := resolverStore(t)
	pr, err := visibility.NewPolicyReader(resolverGate{err: errors.New("down")}, visibility.NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := visibility.NewScriptReader(pr, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, id := range []string{"TKT-1", "NOPE-1"} {
		if _, err := sr.GetAddress(ctx, id); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("GetAddress(%s) err = %v, want ErrNotFound", id, err)
		}
		if _, ok, err := sr.Family(ctx, id); ok || err != nil {
			t.Errorf("Family(%s) = (%v, %v), want a silent miss", id, ok, err)
		}
	}
	if buf.Len() == 0 {
		t.Error("the gate failure was not logged")
	}
}

// TestScriptReader_FamilyReadsHeadersOnce pins that the untyped Family reads
// the stored type and the faces in the same header read.
func TestScriptReader_FamilyReadsHeadersOnce(t *testing.T) {
	st := resolverStore(t)
	sr, err := visibility.NewScriptReader(mustPolicyReader(t, st), st, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := sr.Family(context.Background(), "POL-1"); !ok || err != nil {
		t.Fatalf("Family = (%v, %v)", ok, err)
	}
	if st.Reads() != 1 {
		t.Errorf("reads = %s, want one header read", st)
	}
}

func mustPolicyReader(t *testing.T, st *storetest.Counting) *visibility.PolicyReader {
	t.Helper()
	pr, err := visibility.NewPolicyReader(resolverGate{}, visibility.NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	return pr
}

// resolverlessReader is a Reader whose Resolver is nil.
type resolverlessReader struct{}

func (resolverlessReader) Resolver() *visibility.Resolver { return nil }

func (resolverlessReader) Filter(context.Context, []*entity.Entity) []*entity.Entity { return nil }
func (resolverlessReader) FilterRelations(context.Context, []*entity.Relation) []*entity.Relation {
	return nil
}

func (resolverlessReader) FilterRelationsStrict(context.Context, []*entity.Relation) ([]*entity.Relation, error) {
	return nil, nil
}

func TestNewScriptReader_RequiresAResolver(t *testing.T) {
	if _, err := visibility.NewScriptReader(resolverlessReader{}, memstore.New(), nil); err == nil {
		t.Fatal("a reader without a Resolver was accepted")
	}
}

// TestScriptReader_ListRelationsStrictReturnsGateFaults pins TKT-5LW875: the
// tolerant read hides the edges a faulted gate cannot judge, and the strict
// read returns the fault. An aggregate folds from the strict read, so a
// fault fails the count instead of reading as a missing relation.
func TestScriptReader_ListRelationsStrictReturnsGateFaults(t *testing.T) {
	ctx := context.Background()
	st := resolverStore(t)
	if _, err := st.CreateRelation(ctx, "TKT-1", "implements", "FEAT-1", nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		gate    visibility.RowGate
		wantErr bool
	}{
		{name: "readable", gate: resolverGate{}},
		{name: "gate fault", gate: resolverGate{faceErr: errors.New("down")}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pr, err := visibility.NewPolicyReader(tc.gate, visibility.NopRedactor{}, st)
			if err != nil {
				t.Fatal(err)
			}
			sr, err := visibility.NewScriptReader(pr, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			tolerant, strict := 0, 0
			var strictErr error
			for _, err := range sr.ListRelations(ctx, store.RelationQuery{}) {
				if err != nil {
					t.Fatalf("tolerant read: %v", err)
				}
				tolerant++
			}
			for _, err := range sr.ListRelationsStrict(ctx, store.RelationQuery{}) {
				if err != nil {
					strictErr = err
					break
				}
				strict++
			}
			if tc.wantErr {
				if tolerant != 0 || strictErr == nil {
					t.Fatalf("tolerant=%d strictErr=%v, want the tolerant read to hide and the strict read to fail",
						tolerant, strictErr)
				}
				return
			}
			if strictErr != nil || tolerant != 1 || strict != 1 {
				t.Fatalf("tolerant=%d strict=%d err=%v, want 1, 1, nil", tolerant, strict, strictErr)
			}
		})
	}
}
