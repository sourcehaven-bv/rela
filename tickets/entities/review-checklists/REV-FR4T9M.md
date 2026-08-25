---
id: REV-FR4T9M
type: review-checklist
title: 'Review: Focus rings are a hardcoded indigo that ignores the theme, and vanish entirely in High Contrast'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Frontend: **1839 tests / 114 files** pass (up from 1832 — 7 new guards).
`vue-tsc` clean, ESLint **0 errors**.

Go: `just lint` **0 issues** (two `modernize` findings and three `misspell`
hits — the repo enforces US spelling and this ticket is about colours — were
fixed, not suppressed). `just comment-lint`: **no findings across 10352
comments**. `go build ./...` clean; `internal/dataentry` and
`internal/dataentryconfig` both pass.

Coverage: the Go change is 3 map entries plus a test, so no package floor
moves. The frontend has no coverage enforcement by design.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** [[RR-FRC1SP]] (critical), [[RR-FRC4PL]] (critical),
[[RR-FRC2GD]] (significant), [[RR-FRC3AB]] (significant), [[RR-FRC5DP]]
(significant), [[RR-FRC6DR]] (significant) — all addressed.

**Two review rounds, and the second was necessary.** Round 1 found that the
forced-colors rule lost the cascade and did nothing ([[RR-FRC1SP]]) and that
the opaque ring abutted its own accent border ([[RR-FRC3AB]]). Round 2, run
against the fixed tree, found a *worse* defect the first pass had not reached:
the palette path dropped all three tokens, so palette-configured custom apps
rendered no focus ring at all ([[RR-FRC4PL]]).

Every finding was reproduced independently before being fixed — the
specificity defeat in a CSS engine, the palette gap by executing
`deriveTheme`, the duplicate declaration by grep. Two reviewer claims did not
survive checking and are recorded as such rather than actioned: the guard
DOES catch a known literal on a continuation line (the narrower reading was
wrong), though probing further found a real adjacent gap — a *novel* colour in
a multi-line ring did slip through, which is now fixed and mutation-verified.

Diff contains no unrelated changes. Demo-data writes from live verification
were reverted; the tree holds only the intended files.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 (token follows theme) — PASS.** Verified in-browser before the code was
  written and again after: `--focus-ring` resolves `#4772fb` light / `#6f93ff`
  dark from a single `:root` declaration. Extended past the original AC by
  [[RR-FRC4PL]]: it must hold on the palette path too, now pinned by
  `TestPaletteCarriesEveryDefaultToken`.
- **AC2 (no literals remain) — PASS.** Zero live `rgba(99,102,241,…)` or
  `rgba(239,68,68,…)` outside comments and dead `var()` fallbacks. The guard
  was generalised twice during review and is mutation-verified at each shape.
- **AC3 (forced-colors) — PASS, with a stated limit.** The rule now wins the
  cascade (verified in Chrome against a scoped `outline: none` inside
  `@layer rela`). NOT verified: rendering in a real Windows High Contrast
  session, which needs an environment this tooling cannot reach. Said plainly
  rather than claimed.
- **AC4 (no other visual change) — PASS with one deliberate exception.** The
  ring is visibly bolder everywhere; that was the user's explicit choice once
  the contrast measurements showed the old ring at 1.13:1 against a 3:1
  requirement. All four states were checked in-browser in both themes.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** [[DOCS-FR6K1P]]

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
