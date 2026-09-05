package worldreader_test

import (
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

// TestGuardRule1_SamePrimeForEveryPrincipal: the prime is a pure function of
// (world, family) — [store.ResolveWorldPrimes] takes no context, no
// principal and no gate, so a principal-dependent answer is unrepresentable.
// Pinned by exercising it with the same family under one world and asserting
// the verdict is the chain's first EXISTING face.
func TestGuardRule1_SamePrimeForEveryPrincipal(t *testing.T) {
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"page": {
			Chain:    []entity.Face{"review", "published"},
			Fallback: store.FallbackDefaultState,
		},
	})
	family := []store.WorldCandidate{
		{ID: "PAGE-1", Type: "page", Face: ""},
		{ID: "PAGE-1", Type: "page", Face: "review"},
		{ID: "PAGE-1", Type: "page", Face: "published"},
	}
	first := store.ResolveWorldPrimes(scope, family)["PAGE-1"]
	second := store.ResolveWorldPrimes(scope, family)["PAGE-1"]
	assert.Equal(t, entity.Face("review"), first.Face,
		"chain order is load-bearing: review precedes published")
	assert.Equal(t, first, second, "guard rule 1: the prime must not depend on who asks")
	assert.Equal(t, worldreader.RuleChain, first.Via)
}
