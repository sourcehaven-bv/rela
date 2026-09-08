---
id: IMPL-WAVXD1
type: implementation-checklist
title: 'Implementation: Worlds: metamodel declaration, resolver, pushdown, selection (Step 2)'
status: done
---

<!-- @managed: claude-workflow v1 -->

Delivered as a four-PR stack: PR-A #1393 (metamodel declaration + compiler),
PR-B #1399 (store contract + fs/mem resolution), PR-C #1402 (pgstore SQL
pushdown), PR-D (runtime resolver + wiring).

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

The store contract is verified by ONE shared conformance suite
(`storetest.RunWorldTests`, 13 cases) running unconditionally against
fs, mem AND pgstore — the transitional `Capabilities.Worlds` flag was
deleted in PR-C rather than set, because world resolution is part of the
store contract, not an optional capability.

Error-surfacing was an explicit design decision in two places: the
resolver returns a store fault rather than collapsing it into a clean
miss (that is the visibility Reader's job, and doing it in the resolver
would make a backend outage read as a world exclusion), and
`getState` returns an explicit `found bool` rather than `(nil, nil)` —
absence vs fault is the distinction the chain walk turns on.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Two tests deliberately assert on the CONSTRUCTED QUERY rather than on
results, because no assertion on results can discriminate the failure:
the Q4 fallback trap (nil vs face-to-zero `FromFace` are
indistinguishable when a fixture holds only default-tail edges) and the
decorator/pushdown parity test (a wrongly-defaulted world still returns
plausible rows — which is exactly why the original leak was invisible).

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- **Every fix in this arc was verified NON-VACUOUS** by reverting it and
  confirming the guarding test fails. Applied to: the `CompareStateKeys`
  ordering comparator, the fs/mem index construction comparator, the
  `GraphQuery.World` propagation, the pgstore `worldSQL` pushdown (all 11
  world cases fail when neutered), the Q4 identity/content tail split, the
  guard-rule-1 package scan and arch-lint entry, the searcher
  constructibility refusal, and the header-path world coverage (neutered
  against memstore's NATIVE implementation, since fsstore's fallback
  cannot fail that way).
- **pgstore verified against a REAL PostgreSQL**, not a skip:
  `RELA_TEST_DATABASE_REQUIRED=1` was set deliberately because the
  justfile warns a skip and a pass are indistinguishable in the exit code.
- **End-to-end wiring** exercised by `TestWorldSurface_ResolvesThroughTheWiredStack`:
  worlds declared in schema.yaml → compiled at boot → looked up by name →
  resolved against a real store, covering all three resolution rules,
  exclusion, the default world, and the alias trap.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — `storeutil.MatchEntityQuery` folds
      the duplicated fs/mem matcher (finding F3); `storeutil.WorldPrimes`
      is the single resolution contract that pgstore's SQL rank mirrors so
      the backends cannot drift. `keepPrimes`/`worldKeep` were deliberately
      LEFT duplicated (nine lines, two element types) — a generic there
      adds more indirection than it saves.
- [x] No security issues introduced — the arc CLOSED two fail-opens
      (RR-IDXSRT, RR-GQWRLD) rather than introducing any; see the review
      checklist.
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
