---
id: REV-ZRFXAZ
type: review-checklist
title: 'Review: Replace emoji with an SVG icon set; add icon: to navigation, kanban columns and swimlanes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

Go tests ok (`dataentryconfig`, `dataentry`); `golangci-lint` **0 issues**;
frontend **1504/1504** (94 files); `vue-tsc` clean; ESLint 0 errors; `just
docs-check` and `markdownlint-cli2` pass.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5ESHXG (critical, addressed), RR-NV753I (significant,
addressed), RR-OX9WFS (significant, addressed), RR-GTOQCF (minor, addressed),
RR-VESDBZ (minor, wont-fix with reasoning).

The review earned its keep twice over.

**RR-5ESHXG** is the one that matters: every sidebar icon rendered **24×18**, a
4:3 horizontal stretch, in a PR whose entire purpose was visual consistency.
Lucide emits width/height as *presentation attributes*, which CSS overrides — so
my `width: 24px` beat the 18px width while height stayed 18. It survived because
a squashed circle still reads as a circle at 18px, and jsdom does not lay out
SVG so no test could see it. The reviewer measured it in real Chrome.

**RR-NV753I** found that the drift test I was proud of could silently stop
covering a name: its guard only fired on a *total* parse failure, never a
partial one, so a spread / nested literal / commented-out entry made a name
invisible while the test kept passing.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all 11 PASS — per-criterion evidence in IMPL-DJ9Y71.
Post-review additions:

- **AC 3 (icons follow the theme)** — now also verified *square*: nav 18×18,
kanban 16×16, measured in Chrome. Was 24×18 before the fix.
- **AC 10 (allowlist pinned both ways)** — the test is now
**mutation-tested**: a nested literal fails with "parsed 16 names but
ValidIconNames has 15", a commented-out entry with "parsed 14". Both were silent
passes before.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` — DOCS-9ARY8L
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Three things I'd want a reader to know, all recorded at their site in code:

1. Never set `width` or `flex-basis` on a Lucide icon — both reproduce the
stretch. Size via `:size`, space via `margin`.
2. `go run` embeds the SPA at compile time, so a config-only edit live-reloads
but a Go type change needs a restart. I measured a stale build twice while
chasing the stretch; comparing the served asset hash to the on-disk one is the
quick tell.
3. The reviewer's suggestion to *generate* one allowlist from the other is the
right end state and would delete the parser entirely. Deliberately not done here
— it is a build-pipeline change, not an icon change. Worth a follow-up.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] ~~All CI checks pass~~ (BLOCKED, not skipped: this PR targets
      #1282's branch, and ci.yml only triggers on PRs to main/develop, so
      CI cannot run until #1282 merges and this retargets. The full
      pipeline was run locally instead — see Automated Checks above.)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1285

Stacked on #1282, so it targets `feat/field-span-layout-TKT-5V8704` rather than
`develop` — **CI will not run until #1282 merges and this retargets**, the same
constraint #1282 had against #1281. The full CI pipeline was run locally in the
meantime (race tests, golangci-lint, the frontend job sequence, docs-check).
