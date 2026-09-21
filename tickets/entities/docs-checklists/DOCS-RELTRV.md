---
id: DOCS-RELTRV
type: docs-checklist
title: 'Docs: relation traversal in conditions'
status: done
---

## Code Documentation

- [x] Exported symbols have godoc (`related()` form, `TraversalSpec`,
      `GateTraversal`, `EndpointPredicate`, `ResolveTraversalTarget`)
- [x] Non-obvious decisions explain WHY, not what
- [x] Security invariants documented at the seam that enforces them

## Project Documentation

- [x] ~~User-facing docs updated~~ (N/A: the feature has no user-facing
      surface yet. Nothing calls `GateTraversal` or `ValidateTraversals`, so
      `related(...)` cannot be written in any config an operator edits —
      documenting a syntax nobody can use would be documenting a promise.
      The docs belong with the wiring ticket.)
- [x] ~~`docs/metamodel.md` condition syntax section~~ (N/A: same reason —
      added when the form becomes reachable.)
- [x] Design rationale captured in RES-RELTRV (problem, options, rejected
      alternatives, measurements) and TKT-RELTRV.

## External Documentation

- [x] ~~CHANGELOG entry~~ (N/A: no user-visible behaviour change.)
- [x] ~~Migration notes~~ (N/A: additive; `EndpointMatch` nil means the
      pre-existing behaviour, pinned by a conformance case.)
