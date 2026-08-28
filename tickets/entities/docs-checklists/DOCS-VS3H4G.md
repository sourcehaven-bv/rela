---
id: DOCS-VS3H4G
type: docs-checklist
title: 'Docs: store.GraphQuery property predicates and relation negation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on `store.PropPredicate`, `store.PropOp` (`PropEqual` /
`PropNotEqual`) and `RelationPredicate.Negate` — including *why* Negate is a
separate flag rather than an overload of a nil predicate (nil already means "do
not constrain this direction"), and why ordered comparison is absent (it needs
the metamodel's declared type to avoid comparing dates lexicographically)
- [x] Godoc on `RelationPredicate.Endpoints` documenting that an empty set means
"ANY endpoint" — explicitly flagged as a **widening**, with the warning that
callers deriving endpoints from a principal must guard against it, and that
`InheritThrough` is inert without endpoints
- [x] Package doc on the new `internal/propmatch` — why it exists (two layers
need one emptiness rule but sit on opposite sides of an arch-lint boundary), why
it must stay a pure stdlib leaf (it is imported by the store layer, so anything
added lands there), and what it deliberately does not cover (typed/ordered
comparison stays in `internal/filter`)
- [x] Godoc note that `internal/filter` intercepts list-valued properties before
delegating, so only its empty-list case reaches `propmatch.Decide` — added so a
reader does not assume full coverage from that caller
- [x] Comment in `internal/acl/readquery.go` stating the fail-closed invariant
locally rather than relying on `ForPrincipal`'s validation in another file
- [x] `.go-arch-lint.yml` carries an inline warning that `propmatch` is a pure
leaf on purpose and must not be given dependencies
- [x] ~~CLAUDE.md pattern update~~ (N/A: no new architectural pattern — this
extends an existing seam (`store.GraphQueryer`) and follows the established
conformance-suite discipline the root CLAUDE.md already mandates for store
implementations)

## Project Documentation

- [x] ~~`docs/` update~~ (N/A: `store.GraphQuery` is an internal Go API with no
user-facing surface. It is not reachable from `metamodel.yaml`,
`data-entry.yaml`, the CLI, the HTTP API, or Lua — nothing an operator or end
user can write today changes behaviour because of this ticket.)

The consumer that *will* expose these predicates to operators is the next-action
source config (TKT-CXD0A4 / FEAT-79DTF9), and documenting the `query:` /
`where:` surface belongs with that work, where the operator-facing syntax is
actually decided.

## External Documentation

- [x] ~~README / external docs~~ (N/A: internal API, no external surface)

**Docs verified:** godoc reviewed for the three changed types plus the new
package; the empty-Endpoints widening is documented at the point of use
(`RelationPredicate.Endpoints`) rather than only in a commit message, because
that is where a future caller composing a predicate from a lookup will read it.
