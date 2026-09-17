---
id: IMPL-HPRT4C
type: implementation-checklist
title: 'Implementation: Make face→world resolution total: a tie is the chain head alone'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests~~ (N/A: load-time validation; the unit tests parse a
whole schema through the loader, which is the full flow.)
- [x] Happy path implemented — `primacyKey` drops `otherwise`, so a shared
chain head is the tie.
- [x] Edge cases from planning handled — `primary_for:` resolves the new tie
class exactly as it does the old one; per-type headship via `overrides:` is
unchanged and still covered.
- [x] Error handling in place: the existing load error already names the type,
both worlds, the face, and the `primary_for:` remedy.

## Test Quality

- [x] Using fixture builders: tests reuse the package's `primacyPrefix` schema
fixture and `parseWorlds` helper.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test — each schema declares
only the two competing worlds.
- [x] ~~Interpolated values constructed from objects~~ (N/A: assertions check
that the error text names operator-facing identifiers, which are literals in the
schema under test by design.)
- [x] Property comparisons use the error's own text, not a golden string.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- Mutation check: reverting `primacyKey` to the three-field form while keeping
the new tests fails `TestFacePrimacy_SameHeadDifferentOtherwiseIsATie`, and
passes with the change. The test has teeth.
- All three in-tree prototypes still validate (`rela validate` on
`prototypes/worlds/project`, `prototypes/perf/project`,
`prototypes/data-entry`).
- The corrected `docs/metamodel.md` example was validated as a real schema in
both directions: it loads with `primary_for: nl`, and fails the load with the
claim removed. The doc's rule is executable, not just asserted.
- Go: `go test ./...` (bar `cmd/rela-desktop`, which needs a cgo toolchain this
machine lacks), `arch-lint`, `comment-lint`, `golangci-lint` clean. Frontend:
`test:run` on the schema store (47 pass), `typecheck`, `eslint`.

**Corrected evidence (RR-VACUOUS):** an earlier revision of this checklist
recorded a probe showing a faced-only entity resolving to `map[]` under
`otherwise: default`, and read it as proof the `FallbackDefaultState` arm is
unreachable for any faced type. That probe was run against a constructed case
with no bare row, and generalising from it was wrong. Against
`prototypes/perf/project`'s shape — `perfseed` writes every policy at the bare
coordinate, chain `[draft, published]` — the arm fires (`Via:2`) and the two
`otherwise:` values differ observably (`exclude` → `map[]`). The justification
has been rewritten to rest on the per-type/per-face argument, which needs no
claim about stored rows.

## Quality

- [x] Code follows project patterns — the rule keeps its load-error discipline
and its existing message.
- [x] Checked for DRY opportunities: none warranted; the change removes a
field rather than adding logic.
- [x] No security issues introduced — narrowing only, no runtime resolution or
ACL behaviour changes.
- [x] No silent failures — an ambiguous schema fails the load rather than
resolving by map order.
- [x] No debug code left behind (the probes were run from temp files and
removed; their results are recorded above rather than committed).
