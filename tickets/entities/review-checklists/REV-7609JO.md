---
id: REV-7609JO
type: review-checklist
title: 'Review: Design tokens: spacing, radius, typography and elevation scales for the data-entry SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

Full CI run on the PR (run 31131482774) — **every job passed**: Test, Frontend,
Lint, Architecture, Build, E2E, Fuzz, God-object lint, Postgres Backend, Docs,
Demos, Lint Markdown, Vulnerability Check, CodeQL, and all 6 Cross-Compile
targets. Locally also run with the CI flags: `go test -race -shuffle=on` ok,
`golangci-lint run` 0 issues.

The only failing job was `Rela Tickets`, which is a workflow gate rather than a
code defect: it requires the PR to carry its work-item entities (fixed in
24acccc1) and every referenced ticket/checklist to be out of an in-flight
status. That is what this checklist closes.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-YQ89D0 (critical), RR-D5THES (significant), RR-2V3B2U
(significant), RR-7JFJUB (minor) — all `addressed` in 27bc6ded.

The review earned its keep. RR-YQ89D0 found that the contract test I had written
to guard the Go/CSS typography boundary compared Go against literals in the same
Go file and never opened `scales.css` — verified by changing the CSS to 20px
against Go's 18px and watching it pass green. RR-D5THES found the migration was
not value-preserving as the commit claimed: 15 of 40 radius declarations had
silently changed. Both are fixed and re-verified.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **PASS** — scales defined in `styles/scales.css`, a sibling of `tokens.css`
respecting its theme-only boundary. `TestAppTokensCSSInSyncWithFrontend` still
green, so the Go copy of `tokens.css` is untouched.
2. **PASS** — the type scale reuses the frozen `--font-size-{sm,base,lg,xl}`
names/values. SPA-only steps sit outside the ramp naming (`--font-size-dense`),
so they cannot be mistaken for contract members.
3. **PASS** — `TestFrozenTypographyContractMatchesSPA` reads both files off
disk and compares them to each other. Negative-tested in both directions
(CSS-side drift fails, Go-side drift fails) and confirmed whitespace- tolerant.
`TestAppCSSSource` additionally pins the negative side: apps must not define
`--font-size-dense`.
4. **PASS** — 9 components migrated. On those surfaces radius literals went
9 -> 0 distinct and font-size 15 -> 7 (the 7 being deliberately off-scale
sizes). 126 radius/font + 72 gap/shadow declarations now use tokens.
5. **PASS** — screenshots of detail, list and kanban in both themes; verified
in-browser that `--shadow-sm` differs light (`#00000014`) vs dark (`#0000004d`),
so the `:root.dark` branch is real rather than dead.
6. **PASS** — frontend 1462/1462, Go tests ok, E2E passed in CI.

**The headline claim — "visual output unchanged" — is verified, not asserted.**
A checker resolves every migrated token back to a value and diffs it against
`develop`: **197 declarations, 0 changed** (re-run after the rebase onto the
four commits that landed mid-flight).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing
surface — no config, CLI, API or metamodel change. The documentation this needed
is contributor-facing and lives in `frontend/CLAUDE.md`.)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A, as above)

`frontend/CLAUDE.md` gained a "Design tokens: two files, different contracts"
section: which file holds what and why they are separate, the frozen
`--font-size-*` contract with an explicit warning not to "simplify" the test
back into a Go-only assertion, why SPA-only sizes stay outside the ramp naming,
and why token values are chosen to be value-preserving.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1281
