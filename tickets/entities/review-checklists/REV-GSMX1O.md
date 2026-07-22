---
id: REV-GSMX1O
type: review-checklist
title: 'Review: rela-docs phase 3 (Tier B): screenshot{} island — chromedp capture of the seeded data-entry SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test ./...` green (77 pkgs); browser-gated `internal/docscapture` tests pass with Chrome; frontend 1359 unit tests pass after the DynamicForm/Toast edits
- [x] Lint clean — `golangci-lint` 0 issues; `just arch-lint` OK (`docscapture` component + `chromedp` vendor; `docs` block unchanged); `just lint-md` 0 issues
- [x] Coverage maintained — `just coverage-check` PASS (docscapture documented floor 40% — its core is browser-gated; total 76.3%)

## Code Review

- [x] Ran `/code-review` (cranky-code-reviewer, verified against real code + chromedp/cdproto source) — found **2 critical + 3 significant + minors**, all addressed
- [x] All critical review-responses addressed — RR-8GQ5T2 (seed staleness), RR-8QID3G (renderability gate dead code)
- [x] All significant review-responses addressed — RR-GZ6ZW4 (tall capture blank), RR-9NTWBJ (swallowed Chrome error), + S1 dissolved (RR-A49KYW)
- [x] Self-reviewed the diff — scoped to `internal/docscapture` (new), `internal/docs/{screenshot,seed}.go` + runtime/module wiring, `internal/cli/docs.go`, the SPA `data-testid` edits, arch-lint/coverage config, prototype fixture fix, guide + example

**Review Responses:** RR-8GQ5T2, RR-8QID3G (critical, addressed); RR-GZ6ZW4,
RR-9NTWBJ (significant, addressed); RR-A49KYW (minors/S1, addressed). No
critical/significant left open.

The reviewer confirmed as solid: the consumer-side `Capturer` interface keeps
`internal/docs` browser-free (arch-lint enforced); the `__SPECS__` splice bug is
genuinely fixed (one occurrence remains); annotation injection-safety is real
and tested; the sandbox-first browser launch cleans up partial construction.

## Acceptance Verification

- [x] Each acceptance criterion tested (see IMPL-71EN2X for the AC→test map)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 form capture / AC2 clip: PASS (TestCapture_Form + captureAction clip path)
- AC3 annotation: PASS (TestCapture_AnnotationAndFailLoud)
- AC4 role scoping: PASS (TestBuildRoleAssignee + TestStandUp per-request resolver)
- AC5 no browser / AC6 unknown anchor / AC7 SPA not built: PASS (screenshot_test fail-loud + annotation unknown anchor + CheckEmbeddedSPA)
- AC8 renderability gate: PASS (TestCapture_UnrenderableEntity_FailsLoud — fails loud in ~3s, not a blank PNG, not a 30s timeout)
- AC9 injection-safety: PASS (TestAnnotateScript_InjectionSafe)
- AC10 non-screenshot unaffected: PASS (TestBuild_NoScreenshot_CapturerUntouched)
- AC11 example manual: PASS (builds end-to-end)
- Plus the two review-found bugs guarded: TestCapture_SeedGrowsAcrossIslands (C1), TestCapture_UnrenderableEntity_FailsLoud (C2)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-SHOT)
- [x] User-facing documentation updated — the Screenshots section in the guide + the example manual figure
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-SHOT

## Final Checks

- [x] Commit message explains the why — the feat + fix commits state the design + the review finding each resolves
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — `rela docs build` captures screenshots end-to-end; guide + example demonstrate it

## Pull Request

- [x] ~~Run `/pr`~~ (done-before-PR gate: PR runs AFTER this ticket is `done`, via `/pr`)
- [x] ~~All CI checks pass~~ (verified locally: full `go test ./...`, lint, arch-lint, lint-md, coverage green; the browser-gated tests need the Chrome CI job)
- [x] ~~PR URL documented below~~ (recorded when `/pr` opens it)

**PR:** https://github.com/sourcehaven-bv/rela/pull/1186
