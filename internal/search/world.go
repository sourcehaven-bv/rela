package search

import (
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Candidate is [store.WorldCandidate]: one indexed face offered to
// [ResolvePrimes]. Backends pass their index metadata rather than a loaded
// entity, so resolution costs no I/O.
type Candidate = store.WorldCandidate

// Resolved is [store.WorldResolved]: which face the world serves, and the
// provenance a caller must be able to label it with.
type Resolved = store.WorldResolved

// ResolvePrimes selects, per bare id, the single face world w resolves to,
// with its provenance — [store.ResolveWorldPrimes], the ONE implementation of
// the three rules. Search resolves rather than filters because a world IS
// the search scope (TKT-9KZGJO): a predicate over faces would return two
// rows for an entity holding both, breaking the one-hit-per-entity invariant
// that makes `limit` count entities. Candidates must be WHOLE families.
//
// Exported because the bleve backend lives in its own package and must
// resolve with exactly these rules.
func ResolvePrimes(w store.WorldScope, candidates []Candidate) map[string]Resolved {
	return store.ResolveWorldPrimes(w, candidates)
}
