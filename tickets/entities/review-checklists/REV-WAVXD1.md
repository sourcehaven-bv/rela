---
id: REV-WAVXD1
type: review-checklist
title: 'Review: Worlds: metamodel declaration, resolver, pushdown, selection (Step 2)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — green on every PR in the stack.
      pgstore additionally verified against a REAL PostgreSQL with
      `RELA_TEST_DATABASE_REQUIRED=1`, deliberately, because the justfile
      warns a skip and a pass are indistinguishable in the exit code.
- [x] Lint clean (`just lint`) — 0 issues. Lint caught three real defects
      in this arc's own code, not just style: a `gocritic` off-by-one
      (`strings.Index` can return -1) in a test helper, a `nilnil`
      returning `(nil, nil)` for "not found" where absence-vs-fault is the
      exact distinction the chain walk turns on, and `perfsprint`.
- [x] Coverage maintained (`just coverage-check`) — no floor lowered at
      any point in the arc.

Also run and green: `just arch-lint` (with a new `worldreader` entry
pinning it to `entity + store`), the god-object lint (`Services` held at
25 exported methods by converting `WorldSurface` to a package-level
function rather than raising the directive), `docs-check`, and
`go test -race` across the store packages.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) —
      run per-PR across the stack.
- [x] All critical review-responses addressed — 4 of 4.
- [x] All significant review-responses addressed — 12 of 12.
- [x] Self-reviewed the diff for unrelated changes — additionally, each
      PR's diff was swept against the reviewer-framing list (string-
      rewritten SQL, fragment-assembled predicates, aliasing struct
      copies, swallowed errors, zero-values that mean "unfiltered")
      BEFORE handing it to review. That sweep found three real defects in
      this arc's own code: the `ReplaceAll` alias corruption, the
      `DirectionBoth` zero-value narrowing, and the load-bearing
      `*rqr.Query` copy.

**Review Responses:** 22 linked. Critical: RR-7FOWDB, RR-CUUZ9Z,
RR-GQWRLD, RR-IDXSRT. Significant: RR-B8TCA1, RR-CGRV0X, RR-CZN30X,
RR-E1C216, RR-EHER1V, RR-FAMTYP, RR-HDRWLD, RR-LLLBQY, RR-MATCHF,
RR-MNOBJK, RR-QB632G, RR-S0XH7B. Minor: RR-CNTALL, RR-KNDLGR, RR-NJSCP5,
RR-PSVTR8, RR-ZA6008. Nit: RR-3JRSFV (deferred with reason).

**Two of the criticals were real fail-opens found DURING this arc, not
inherited:**

- **RR-IDXSRT** — the fs/mem index CONSTRUCTION sites still sorted with
  the old comparator while every mutation site used the new one, so after
  any process restart the slice was sorted one way and binary-searched
  another. Symptom: a PANIC on the ordinary default-world `DeleteEntity`,
  with no world involved at all. The conformance suite was structurally
  blind to it because its factory never reopens a store.
- **RR-GQWRLD** — `GraphQuery.World` was plumbed into the struct and then
  dropped, so a world-scoped list silently degraded to unscoped for
  exactly the ACL-gated principals: drafts leaked and `otherwise: exclude`
  stopped excluding. pgstore was safe only via a temporary refusal that
  PR-C removes, so it had a live path to postgres.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **Metamodel declaration + load-time validation** — PASS. `pointers:`,
  `worlds:`, mandatory `otherwise:`, reserved `default` name, pointer
  grammar; invalid names fail the boot.
- **Resolution is a FUNCTION (at most one prime per world/entity)** —
  PASS. Asserted by the conformance suite's `AtMostOnePrimePerEntity` on
  all three backends, and structurally in pgstore by `DISTINCT ON (id)`.
- **Principal-independent resolution; ACL never participates in fallback**
  — PASS, pinned THREE ways: arch-lint, a package scan with an exemption
  list, and a behavioral test. Each verified non-vacuous by deliberately
  breaking it. All three are needed — arch-lint and the scan cannot catch
  a principal read off ctx, and the behavioral test alone would stay green
  for its fixtures if someone added a gate.
- **Provenance on the prime** — PASS. `worldreader.Resolved.Via`
  distinguishes unscoped / chain / fallback-default / excluded.
- **Pushdown in SQL, not a per-row Go filter** — PASS.
  `DISTINCT ON (id) ... ORDER BY id, rank`; neutering it fails all 11
  world conformance cases on real postgres.
- **Default world costs nothing** — PASS. The default-world SQL is
  byte-identical to the pre-worlds shape (`WHERE pointer = ''`, no
  `DISTINCT ON`, no extra binds), and `EXPLAIN` shows an identical plan
  at identical cost.
- **Wiring-site binding, no world parameter (Q10)** — PASS. Surfaces are
  constructed over their world; there is no `?world=`/`--world`.
- **Decorator/pushdown parity** — PASS, and the test is the empirical
  proof of its own necessity: reverting the fix fails the ACL-COMPOSED
  case while the AllowAll case stays GREEN, so a parity test covering
  only the unrestricted principal would have passed throughout the window
  RR-GQWRLD was live.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-WAVXD1

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed — the one transitional marker
      (`TODO(TKT-WAV8XP-PR-C): remove` on `Capabilities.Worlds`) was
      honored: PR-C DELETED the flag rather than setting it, because
      world resolution is part of the store contract, not an optional
      capability. A transitional flag that outlives its window becomes a
      permanent conformance opt-out.
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** Four-PR stack.
PR-A https://github.com/sourcehaven-bv/rela/pull/1393 (20/20 green),
PR-B https://github.com/sourcehaven-bv/rela/pull/1399 (20/20 green),
PR-C https://github.com/sourcehaven-bv/rela/pull/1402 (18/18 green,
including Postgres Backend running the now-unconditional world
conformance suite on a clean runner),
PR-D (this one).
