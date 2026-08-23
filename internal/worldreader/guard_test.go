package worldreader_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
)

// GUARD RULE 1 — resolution is PRINCIPAL-INDEPENDENT.
//
// A world resolves the same prime for every reader. This file pins that
// twice, because the two failures look nothing alike:
//
//   - BEHAVIOURALLY: two principals resolve the same entity to the same
//     face, and a denied principal gets NOTHING rather than a different
//     face.
//   - STRUCTURALLY: the package cannot even express a principal-dependent
//     resolution, because it imports nothing that carries a principal.
//
// The behavioral test alone would not stop someone adding a gate and
// keeping the test green for the fixtures it happens to use; the
// structural test alone would not catch resolution reading a principal
// off ctx. Both are needed.
//
// The stake: if the gate ran BEFORE resolution, a prime the principal may
// not read would fall through to the chain's next candidate, and the face
// a reader receives would reveal what the ACL denied them — an existence
// oracle. Resolve-then-gate is the fixed order, and the only way to keep
// it is that the resolver cannot consult a gate at all.

// forbiddenImports are packages whose presence would mean resolution can
// see a principal or an ACL decision.
var forbiddenImports = map[string]string{
	"internal/acl":         "an ACL type in the resolver means resolution could depend on a decision",
	"internal/visibility":  "the gate must run AFTER resolution, never inside it",
	"internal/principal":   "a principal in scope means resolution could vary by reader",
	"internal/affordances": "affordance policy is principal-scoped",
	"internal/aclmap":      "ACL mapping is principal-scoped",
}

// exemptFiles are files NOT scanned. An EXEMPTION list, not an inclusion
// list, so a newly added file fails closed: it must be clean or be
// exempted here with a reason.
var exemptFiles = map[string]string{
	// This guard names the forbidden packages in order to forbid them.
	"guard_test.go": "the guard itself — names the packages it forbids",
}

// TestGuardRule1_PackageCannotSeeAPrincipal is the structural half: scan
// every non-exempt file for an import that would let resolution vary by
// reader.
func TestGuardRule1_PackageCannotSeeAPrincipal(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if reason, exempt := exemptFiles[name]; exempt {
			t.Logf("skipping %s: %s", name, reason)
			continue
		}
		src, readErr := os.ReadFile(filepath.Join(".", name))
		require.NoError(t, readErr)
		scanned++

		for pkg, why := range forbiddenImports {
			// Assert on a bool rather than via NotContains: the latter
			// dumps the whole file into the failure message, which buries
			// the one line that matters.
			if strings.Contains(string(src), "rela/"+pkg+"\"") {
				t.Errorf("%s imports %s — guard rule 1 forbids it: %s", name, pkg, why)
			}
		}
	}
	require.Positive(t, scanned, "the guard must actually scan files, or it proves nothing")
}

// TestGuardRule1_ConstructorTakesNoGate pins the constructor's shape by
// its signature rather than its body: NewResolver's collaborators are a
// state reader, a scope and a canonicalizer. Adding a gate parameter
// would have to change this call, which is the point.
func TestGuardRule1_ConstructorTakesNoGate(_ *testing.T) {
	// The signature is SPELLED OUT deliberately. `var x = NewResolver`
	// infers whatever the constructor's type currently is, so it pins
	// nothing and a new gate parameter would compile clean — the pin was a
	// no-op until TKT-DN37J2's design review caught it (RR-NFDPOA).
	//nolint:staticcheck // ST1023 wants the type inferred from the RHS —
	// which is exactly the bug this test had before RR-NFDPOA. Inferring
	// makes the declaration accept ANY signature, so a new gate parameter
	// would compile clean and the guard would pin nothing. The explicit
	// type IS the assertion.
	var newResolver func(
		worldreader.StateReader, store.WorldScope, worldreader.TypeCanonicalizer,
	) (*worldreader.Resolver, error) = worldreader.NewResolver
	_ = newResolver
}

// principalKey mimics a caller stashing an identity on ctx. The resolver
// must ignore it — it has no way to read it, which is the guarantee.
type principalKey struct{}

// TestGuardRule1_SamePrimeForEveryPrincipal is the behavioral half, in
// the shape the plan specifies: an entity with default, review and
// published states; a world selecting [review, published] with
// `otherwise: default`. Both principals must get 'review'.
func TestGuardRule1_SamePrimeForEveryPrincipal(t *testing.T) {
	states := map[entity.Pointer]string{
		"":          "the default face",
		"review":    "the review face",
		"published": "the published face",
	}
	reader := stateReaderFunc(func(_ context.Context, id string, p entity.Pointer) (*entity.Entity, error) {
		title, ok := states[p]
		if id != "PAGE-1" || !ok {
			return nil, store.ErrNotFound
		}
		e := entity.New(id, "page")
		e.Pointer = p
		e.SetString("title", title)
		return e, nil
	})

	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"page": {
			Chain:    []entity.Pointer{"review", "published"},
			Fallback: store.FallbackDefaultState,
		},
	})
	r, err := worldreader.NewResolver(reader, scope, identityCanon{})
	require.NoError(t, err)

	// Two "principals", distinguishable only by what the caller put on
	// ctx. The resolver has no way to read either.
	principalA := context.WithValue(context.Background(), principalKey{}, "may-read-everything")
	principalB := context.WithValue(context.Background(), principalKey{}, "denied-on-PAGE-1")

	resA, err := r.Resolve(principalA, "page", "PAGE-1")
	require.NoError(t, err)
	resB, err := r.Resolve(principalB, "page", "PAGE-1")
	require.NoError(t, err)

	assert.Equal(t, entity.Pointer("review"), resA.Pointer,
		"chain order is load-bearing: review precedes published")
	assert.Equal(t, resA.Pointer, resB.Pointer,
		"guard rule 1: the prime must not depend on the principal")
	assert.Equal(t, "the review face", resB.Entity.GetString("title"))

	// The gate's job is to withhold the resolved face ENTIRELY. It must
	// never hand principal B a different face — that is the oracle. The
	// resolver cannot express that, which is what this whole file pins:
	// B either gets 'review' (as here, before gating) or nothing at all
	// once the gate downstream denies it.
}
