---
id: REV-3H4Q3L
type: review-checklist
title: 'Review: Next-action layer Phase 0'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full `./internal/...` green; pgstore
      additionally run against live PostgreSQL under `-race`
- [x] Lint clean (`just lint`) — 0 issues; `just arch-lint` and
      `just plimsoll` clean
- [x] Coverage maintained (`just coverage-check`) — package floors satisfied

Frontend: typecheck clean, 1709 tests, 0 lint errors.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed — RR-9Z9V07, RR-HO25O0
- [x] All significant review-responses addressed — RR-RV39AZ, RR-KK413X,
      RR-3UZR29
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-9Z9V07 (critical), RR-HO25O0 (critical), RR-RV39AZ
(significant), RR-KK413X (significant), RR-3UZR29 (significant), RR-7BI6XC
(minor), RR-R2CNC7 (minor) — all addressed.

The two criticals were genuine and neither was visible from the feature's own
tests:

- **RR-9Z9V07** — suggestion messages leaked `visible:`-hidden property
  values. Reproduced with a failing test before fixing. The root cause is the
  one the root CLAUDE.md warns about: `executeQuery` returns raw entities and
  every consumer is individually responsible for redacting on the way out.
  `/_search` remembers; this layer forgot.
- **RR-HO25O0** — the property pushdown changed results on a path `/_search`
  and scope navigation share. My first fix was still wrong (it narrowed away a
  typed match), which the tests caught.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **Source resolution engine** — PASS. Band-order evaluation with
   short-circuit (pinned by a test asserting a lower band is never *queried*),
   engine-owned candidate bound, stable-random pick.
2. **Operator-defined bands** — PASS. Ordered list, referenced by id,
   validation rejects unknown references and unknown prominence values.
3. **User-state service** — PASS. Three backends (mem, KV, postgres), all
   passing one conformance suite.
4. **Conformance suite** — PASS. 32 subtests per backend; the postgres run is
   DB-gated and verified against live PostgreSQL with zero skips.
5. **Render surface** — PASS. Three prominence tiers verified in a browser.
6. **Affordances** — PASS, and extended: `pick_one` (Phase 1) resolves options
   from a query at render time.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-BHFCLM

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: pushing and
      opening a PR is the maintainer's call — not taken autonomously.)
- [x] ~~All CI checks pass~~ (N/A: no PR yet. CI's constituent targets were run
      locally — lint, arch-lint, plimsoll, full Go suite, the postgres suite
      with `-race`, frontend typecheck/tests/lint — all green.)
- [x] ~~PR URL documented below~~ (N/A: no PR yet)

**PR:** not created. The work sits on `feat/next-action-phase0`.
