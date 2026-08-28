---
id: REV-32TASY
type: review-checklist
title: 'Review: Operator customisation hooks: serve + inject custom.css/custom.js, @layer cascade fix, isCustomElement'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` — all packages pass
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK, no warnings
- [x] `just coverage-check` — PASS (77.4%, both package and total thresholds)
- [x] `npm run test:run` — 98 files / 1575 tests pass
- [x] `npm run typecheck` — clean
- [x] `npm run lint` — 0 errors (92 pre-existing warnings, unchanged from baseline)
- [x] `npx playwright test` — 241 passed, 8 skipped, 0 failures

## Code Review

- [x] `/code-review` run (cranky-code-reviewer) — 8 findings, all recorded as review-responses
      and all addressed:
  - RR-CR-REGEX (critical) — regex corrupted CSS with braces in comments/strings/nesting.
    **Independently reproduced** before fixing. Replaced with a postcss parse/walk/stringify.
  - RR-CR-IMPORT (critical) — `@import`/`@charset` were wrapped inside the layer, where browsers
    drop them. Now hoisted above the layer by construction.
  - RR-CR-TESTGAP (significant) — no test asserted output validity, which is why the above were
    invisible. Added a 14-case adversarial table asserting parse-validity, stable round-trip, and
    declaration preservation.
  - RR-CR-DOUBLEREAD (significant) — existence check read up to 4MB per file per request; now
    stats. Pinned by TestCustomAssetExists_MatchesOpen.
  - RR-CR-LAYERTEST (significant) — drift test was simultaneously too weak and too strong; replaced
    with a depth-aware scanner plus 8 tests of the guard itself.
  - RR-CR-TOCTOU (minor) — accepted, documented in godoc.
  - RR-CR-ETAG (minor) — deferred with rationale (3.4KB shell, must reflect per-request state).
  - RR-CR-SLOTUNUSED (minor) — docs corrected: `<rela-slot>` is reserved, not yet emitted.
- [x] All critical/significant findings fixed and re-verified

## Acceptance Verification

- [x] **AC1** PASS — serves when present, 404 when absent; traversal rejected. Go tests
      (13 traversal vectors incl. `CUSTOM.CSS`, NUL, symlink escape) + live-server probe.
- [x] **AC2** PASS — `TestSPAShellInjection` asserts the stock shell is **byte-identical** to the
      embedded original; per-variant tag presence and `Content-Length` correctness asserted.
- [x] **AC3** PASS — verified in a real browser (Playwright + manual): operator CSS wins an
      equal-and-lower-specificity tie against a route chunk loaded after client-side navigation.
- [x] **AC4** PASS — `RelaSlot.test.ts` (SFC fixture, per the documented trap) asserts no
      "Failed to resolve component" warning; element renders inert.
- [x] **AC5** PASS — `relaEditor.test.ts` 15 tests green; e2e apps suite green.
- [x] **AC6** PASS — `docs/customisation.md` carries the tier table, the verbatim disclaimer, both
      gotchas, the `!important` inversion, and the trust-model comparison.

## Notes

Pre-existing flake, NOT introduced here: `EnvironmentTeardownError` in `DynamicForm.test.ts`
(vitest worker RPC teardown race) — reproduced on unmodified `develop` (1 of 5 runs).
