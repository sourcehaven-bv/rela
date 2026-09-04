package pgstore

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// worldSQL is pure string generation, so it is testable without a
// database — which matters because the conformance suite that exercises
// the resulting SQL is DB-gated and SKIPS when RELA_TEST_DATABASE_URL is
// unset. A skip and a pass look identical in an exit code, so the
// properties that must never regress are pinned here too.

// scope builds a one-type world. The type name is fixed: these tests
// are about the generated SQL's SHAPE, and varying the type would only
// change which string lands in the bind args.
func scope(t *testing.T, fb store.Fallback, chain ...string) store.WorldScope {
	t.Helper()
	const typ = "page"
	coords := make([]entity.Face, 0, len(chain))
	for _, c := range chain {
		p, err := entity.ParseFace(c)
		require.NoError(t, err)
		coords = append(coords, p)
	}
	return store.NewWorldScope(map[string]store.TypeResolution{
		typ: {Chain: coords, Fallback: fb},
	})
}

func TestWorldSQL_BindsCoordinatesAsParameters(t *testing.T) {
	// A coordinate must never be interpolated into the SQL text: the
	// compiled scope is trusted, but "trusted input is interpolated
	// here" is exactly the habit that later meets untrusted input.
	var args []any
	rank, candidate := worldSQL(scope(t, store.FallbackExclude, "published"), "", &args)

	assert.NotContains(t, rank, "published", "coordinate must be a bind parameter, not SQL text")
	assert.NotContains(t, candidate, "published", "coordinate must be a bind parameter, not SQL text")
	assert.Contains(t, args, "published")
	assert.Contains(t, args, "page")
}

func TestWorldSQL_DefaultRowRanksBelowEveryCoordinate(t *testing.T) {
	// Under `otherwise: default` the default row is the LAST resort, so
	// it must rank len(chain) — ranking it 0 would make it beat every
	// real coordinate and the world would serve drafts.
	var args []any
	rank, _ := worldSQL(scope(t, store.FallbackDefaultState, "published", "review"), "", &args)

	assert.Contains(t, rank, "THEN 2",
		"the default row must rank len(chain)=2, below both coordinates")
}

func TestWorldSQL_ExcludeOmitsTheDefaultRowFromCandidates(t *testing.T) {
	// Under `otherwise: exclude` the default row is not a candidate at
	// all: absence IS the publication bit. If it leaked into the
	// candidate set it would win by default for any entity lacking a
	// published state — the exact leak the fallback exists to prevent.
	var args []any
	_, exclude := worldSQL(scope(t, store.FallbackExclude, "published"), "", &args)

	var args2 []any
	_, dflt := worldSQL(scope(t, store.FallbackDefaultState, "published"), "", &args2)

	// Compare the SCOPED-type arm only. Both predicates end with a
	// rule-1 trailing arm (`NOT (type = $1) AND face = ''`) admitting
	// the default row of types the world does not scope, so asserting on
	// the whole string would test the wrong clause.
	scopedArm := func(pred string) string {
		i := strings.Index(pred, " OR (NOT (")
		require.GreaterOrEqual(t, i, 0, "predicate must carry a rule-1 trailing arm")
		return pred[:i]
	}
	assert.NotContains(t, scopedArm(exclude), "face = ''",
		"exclude must not admit the scoped type's default row as a candidate")
	assert.Contains(t, scopedArm(dflt), "face = ''",
		"otherwise: default must admit the default row as a last resort")
}

func TestWorldSQL_UnscopedTypesKeepTheirDefaultState(t *testing.T) {
	// Rule 1: a type the world says nothing about contributes its
	// default state. The trailing arm must therefore admit rows whose
	// type is NOT one of the scoped ones — a candidate predicate that
	// only listed scoped types would silently drop every other type
	// from a mixed-type query.
	var args []any
	_, candidate := worldSQL(scope(t, store.FallbackExclude, "published"), "", &args)

	assert.Contains(t, candidate, "NOT (",
		"rule 1 needs a trailing arm for types the world does not scope")
	assert.Contains(t, candidate, "face = ''",
		"unscoped types contribute their default state")
}

func TestWorldSQL_IsDeterministic(t *testing.T) {
	// Map iteration order must not leak into the SQL text: an unstable
	// string defeats PostgreSQL's plan cache and makes failures
	// irreproducible.
	w := store.NewWorldScope(map[string]store.TypeResolution{
		"page":  {Chain: []entity.Face{"published"}, Fallback: store.FallbackExclude},
		"note":  {Chain: []entity.Face{"live"}, Fallback: store.FallbackDefaultState},
		"draft": {Chain: []entity.Face{"wip"}, Fallback: store.FallbackExclude},
	})
	var firstRank, firstCand string
	var firstArgs []any
	for i := range 20 {
		var args []any
		rank, cand := worldSQL(w, "", &args)
		if i == 0 {
			firstRank, firstCand, firstArgs = rank, cand, args
			continue
		}
		require.Equal(t, firstRank, rank, "rank SQL must not vary between builds")
		require.Equal(t, firstCand, cand, "candidate SQL must not vary between builds")
		require.Equal(t, firstArgs, args, "bind order must not vary between builds")
	}
}

func TestWorldSQL_EmptyChainWithExcludeContributesNothing(t *testing.T) {
	// A scoped type with an empty chain and `exclude` has no candidate
	// at all. The explicit false arm keeps the trailing rule-1 arm
	// meaning ONLY "unscoped type" rather than silently absorbing this
	// type and serving its default state.
	var args []any
	_, candidate := worldSQL(scope(t, store.FallbackExclude), "", &args)

	assert.Contains(t, candidate, "AND false",
		"an empty chain under exclude must contribute no rows")
}

func TestWorldSQL_RankAndCandidateShareBindParameters(t *testing.T) {
	// Both expressions go into ONE statement, so they must be built in
	// a single call against the same arg slice. Building them
	// separately would bind every coordinate twice and, worse, make the
	// rank's placeholders refer to the wrong positions.
	var args []any
	rank, candidate := worldSQL(scope(t, store.FallbackDefaultState, "published"), "", &args)

	for _, ph := range []string{"$1", "$2"} {
		assert.True(t, strings.Contains(rank, ph) || strings.Contains(candidate, ph),
			"placeholder %s must be used by one of the two expressions", ph)
	}
	assert.Len(t, args, 2, "one type + one coordinate = exactly two binds")
}

func TestWorldSQL_AliasQualifiesEveryColumnReference(t *testing.T) {
	// The graph queries alias the entities table as "e", so every column
	// reference must be qualified. This is a PARAMETER rather than a
	// post-hoc ReplaceAll over the generated SQL: rewriting the string
	// would corrupt any column whose name merely CONTAINS "face"
	// (from_face is one), and would silently miss a column added
	// later.
	w := scope(t, store.FallbackDefaultState, "published")

	var bare []any
	bareRank, bareCand := worldSQL(w, "", &bare)
	var aliased []any
	aliasRank, aliasCand := worldSQL(w, "e", &aliased)

	assert.NotContains(t, bareRank, "e.", "no alias means no qualification")
	assert.NotContains(t, bareCand, "e.", "no alias means no qualification")

	// Every column mention must be qualified in the aliased form: no
	// bare "face"/"type" may survive, or PostgreSQL resolves it
	// against whatever else is in scope.
	for _, frag := range []string{" face", "(face", " type ", "(type "} {
		assert.NotContains(t, aliasRank, frag, "unqualified column in aliased rank: %q", frag)
		assert.NotContains(t, aliasCand, frag, "unqualified column in aliased candidate: %q", frag)
	}
	assert.Contains(t, aliasCand, "e.face")
	assert.Contains(t, aliasCand, "e.type")
	assert.Equal(t, bare, aliased, "alias must not change the bind parameters")
}

func TestWorldSQL_ZeroInChainRanksAboveTheFallback(t *testing.T) {
	// A chain contains the ZERO coordinate whenever a world names the type's
	// DEFAULT face — internal/worlds stores metamodel.StoredFace, which maps
	// a `bare_face` name to "". Under `otherwise: default` that emits TWO
	// `face = ''` arms: the chain entry at rank i, and the fallback at rank
	// len(chain). CASE takes the first match, so the arms must be emitted in
	// that order or a selected default face would be labeled a fallback and,
	// worse, out-ranked by nothing while ranking below its own chain position.
	var args []any
	rank, _ := worldSQL(store.NewWorldScope(map[string]store.TypeResolution{
		"page": {
			Chain:    []entity.Face{"published", ""},
			Fallback: store.FallbackDefaultState,
		},
	}), "", &args)

	chainArm := strings.Index(rank, "THEN 1")
	fallbackArm := strings.Index(rank, "THEN 2")
	assert.GreaterOrEqual(t, chainArm, 0, "expected the zero coordinate's chain arm at rank 1")
	assert.GreaterOrEqual(t, fallbackArm, 0, "expected the fallback arm at rank len(chain)=2")
	assert.Less(t, chainArm, fallbackArm,
		"the chain arm must precede the fallback arm; CASE takes the first match")
}
