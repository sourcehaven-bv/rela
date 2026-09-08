package perfseed_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/perfseed"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const testScale = 0.01

// digest hashes everything a generator emits, in order, so two runs can be
// compared byte for byte without holding both in memory.
func digest(t *testing.T, g *perfseed.Generator) (sum string, entities, relations int) {
	t.Helper()
	h := sha256.New()
	ents, rels := 0, 0
	for e := range g.Entities() {
		ents++
		fmt.Fprintf(h, "E|%s|%s|%s|%v|%s\n", e.ID, e.Face, e.Type, e.Properties, e.Content)
	}
	for r := range g.Relations() {
		rels++
		fmt.Fprintf(h, "R|%s|%s|%s|%s\n", r.From, r.FromFace, r.Type, r.To)
	}
	return hex.EncodeToString(h.Sum(nil)), ents, rels
}

func TestGenerator_Deterministic(t *testing.T) {
	a, ea, ra := digest(t, perfseed.New(perfseed.Perf(testScale), 7))
	b, eb, rb := digest(t, perfseed.New(perfseed.Perf(testScale), 7))
	require.Equal(t, a, b, "same profile and seed must produce identical streams")
	require.Equal(t, ea, eb)
	require.Equal(t, ra, rb)
	require.Positive(t, ra)

	c, _, _ := digest(t, perfseed.New(perfseed.Perf(testScale), 8))
	require.NotEqual(t, a, c, "a different seed must produce a different graph")
}

func TestGenerator_StreamsAreIndependent(t *testing.T) {
	// Consuming relations before entities (or only relations) must yield
	// the same relations: nothing is drawn from a shared stream.
	g := perfseed.New(perfseed.Perf(testScale), 3)
	var first []perfseed.Relation
	for r := range g.Relations() {
		first = append(first, r)
	}
	for e := range g.Entities() {
		_ = e
	}
	var second []perfseed.Relation
	for r := range g.Relations() {
		second = append(second, r)
	}
	require.Equal(t, first, second)
}

func TestGenerator_ShapeMatchesProfile(t *testing.T) {
	p := perfseed.Perf(testScale)
	g := perfseed.New(p, 1)
	byType := map[string]int{}
	faces := map[string]int{}
	ids := map[string]bool{}
	demo := map[string]bool{}
	for e := range g.Entities() {
		if e.Face.IsDefault() {
			byType[e.Type]++ // one entity per default row; faces are counted below
		} else {
			faces[e.Type+"@"+e.Face.String()]++
		}
		key := e.ID + "@" + e.Face.String()
		require.False(t, ids[key], "duplicate row %s", key)
		ids[key] = true
		require.NotEmpty(t, e.GetString("title"), "%s has no title", e.ID)
		require.NotEmpty(t, e.Content, "%s has no body", e.ID)
		if e.Type == "person" {
			demo[e.GetString("email")] = true
		}
	}
	require.Equal(t, p.Teams, byType["team"])
	require.Equal(t, p.People, byType["person"])
	require.Equal(t, p.Projects, byType["project"])
	require.Equal(t, p.Tasks, byType["task"])
	require.Equal(t, p.Controls, byType["control"])
	require.Equal(t, p.Risks, byType["risk"])
	require.Equal(t, p.Policies, byType["policy"])
	require.Equal(t, p.Documents, byType["document"])
	require.Positive(t, faces["policy@published"], "some policies must be published")
	require.Less(t, faces["policy@published"], p.Policies, "not every policy is published")
	require.Positive(t, faces["document@nl"])
	for _, email := range []string{"alice@perf.example", "bob@perf.example", "carol@perf.example"} {
		require.True(t, demo[email], "demo principal %s missing", email)
	}
}

func TestGenerator_RelationsReferenceEmittedRows(t *testing.T) {
	g := perfseed.New(perfseed.Perf(testScale), 5)
	rows := map[string]bool{}
	for e := range g.Entities() {
		rows[e.ID+"@"+e.Face.String()] = true
	}
	n := 0
	for r := range g.Relations() {
		n++
		require.True(t, rows[r.From+"@"+r.FromFace.String()], "edge from unknown row %s@%s", r.From, r.FromFace)
		require.True(t, rows[r.To+"@"], "edge to unknown entity %s", r.To)
		require.NotEqual(t, r.From, r.To, "self edge %s --%s--> %s", r.From, r.Type, r.To)
	}
	require.Positive(t, n)
}

func TestLoad_WritesEverythingIntoStore(t *testing.T) {
	g := perfseed.New(perfseed.Perf(testScale), 1)
	_, wantE, wantR := digest(t, g)

	st := memstore.New()
	var progress int
	sum, err := perfseed.Load(context.Background(), st, g, perfseed.LoadOptions{
		BatchSize: 50,
		Progress:  func(perfseed.Summary) { progress++ },
	})
	require.NoError(t, err)
	require.Equal(t, wantE, sum.Entities)
	require.Equal(t, wantR, sum.Relations)
	require.Positive(t, progress)

	ctx := context.Background()
	gotE, err := st.CountEntities(ctx, store.EntityQuery{AllStates: true})
	require.NoError(t, err)
	require.Equal(t, wantE, gotE, "every row, faces included, must be in the store")
	gotR, err := st.CountRelations(ctx, store.RelationQuery{})
	require.NoError(t, err)
	require.Equal(t, wantR, gotR)

	// A published face is addressable as a state, and its content-scoped
	// edges hang off that face, not the draft.
	pub := 0
	for e := range g.Entities() {
		if e.Type != "policy" || e.Face.IsDefault() {
			continue
		}
		got, err := st.GetEntityState(ctx, e.ID, e.Face)
		require.NoError(t, err)
		require.Equal(t, e.GetString("title"), got.GetString("title"))
		face := e.Face
		n, err := st.CountRelations(ctx, store.RelationQuery{From: e.ID, Type: "implements", FromFace: &face})
		require.NoError(t, err)
		require.Positive(t, n)
		pub++
		if pub == 3 {
			break
		}
	}
	require.Equal(t, 3, pub)
}

func TestLoad_StopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sum, err := perfseed.Load(ctx, memstore.New(), perfseed.New(perfseed.Perf(testScale), 1), perfseed.LoadOptions{})
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, sum.Entities)
}

func BenchmarkGenerate(b *testing.B) {
	g := perfseed.New(perfseed.Perf(0.1), 1)
	for b.Loop() {
		for e := range g.Entities() {
			_, _ = io.Discard.Write([]byte(e.Content))
		}
	}
}
