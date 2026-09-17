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

- Empirical probe against `store.ResolveWorldPrimes` confirming the exemption is
vacuous for a faced type: faced-only entity under `otherwise: default`, chain
`[published]` → `map[]` (excluded); the same world with a bare row present →
`{Face:"", Via:2}` (`ResolutionFallbackDefault`). This is what justifies
collapsing the key.
- Mutation check: reverting `primacyKey` to the three-field form while keeping
the new tests fails `TestFacePrimacy_SameHeadDifferentOtherwiseIsATie`, and
passes with the change. The test has teeth.
- All three in-tree prototypes still validate (`rela validate` on
`prototypes/worlds/project`, `prototypes/perf/project`,
`prototypes/data-entry`).
- The corrected `docs/metamodel.md` example was validated as a real schema in
both directions: it loads with `primary_for: nl`, and fails the load with the
claim removed. The doc's rule is executable, not just asserted.

## Quality

- [x] Code follows project patterns — the rule keeps its load-error discipline
and its existing message.
- [x] Checked for DRY opportunities: none warranted; the change removes a
field rather than adding logic.
- [x] No security issues introduced — narrowing only, no runtime resolution or
ACL behaviour changes.
- [x] No silent failures — an ambiguous schema fails the load rather than
resolving by map order.
- [x] No debug code left behind (the empirical probe was run from a temp file
and removed; it is recorded above rather than committed).
