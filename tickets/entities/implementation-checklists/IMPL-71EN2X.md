---
id: IMPL-71EN2X
type: implementation-checklist
title: 'Implementation: rela-docs phase 3 (Tier B): screenshot{} island — chromedp capture of the seeded data-entry SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `internal/docs/screenshot_test.go` (fail-loud paths, no browser), `internal/docscapture/{annotate,server}_test.go` (injection-safety, anchors, formURL, roleAssignee, copyProjectSchema, standUp)
- [x] Integration tests written — `internal/docscapture/capture_test.go` (browser-gated: real form capture → valid PNG, annotation success + unknown-anchor fail-loud); the example manual builds screenshot end-to-end
- [x] Happy path implemented — `screenshot{}` → temp project → server → chromedp → renderability gate → annotated PNG → `![]()`
- [x] Edge cases handled — renderability gate (broken form → BuildError), height cap, unknown field anchor, missing args, nil-Capturer, non-screenshot manual untouched
- [x] Error handling in place — every failure is a fail-loud `BuildError`/wrapped error; no graceful degradation

## Test Quality

- [x] Using fixture builders — `protoDir`, shared seed specs, `mustWrite` helper
- [x] No hardcoded values where object is in scope — assertions check PNG magic/size, role→user mapping from the fixture policy
- [x] Only specifying values that matter — seeds carry the minimum for the form to render
- [x] Interpolated values from objects — role assignee tested against the written acl.yaml
- [x] Property comparisons use original object — yes

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- **End-to-end capture** (`rela docs build`): the example manual `prototypes/data-entry/manual/tickets-manual.md` builds Tier-A + a real annotated `screenshot{}` → `manual.md` + `ticket-form.png` (144KB, valid PNG). Verified the captured image visually: the editable Edit-Ticket form with the ALLOWED TRANSITIONS lifecycle table, seed data, and arrows landing on the status + priority fields.
- **AC1 form capture / AC2 clip** — `TestCapture_Form` (valid PNG magic + size); clip path exercised by `captureAction`.
- **AC3 annotation** — `TestCapture_AnnotationAndFailLoud` (valid anchor succeeds).
- **AC4 role scoping** — `TestBuildRoleAssignee` (role→principal inversion, update-capable default); `TestStandUp_ServesSeededEntity` exercises the per-request resolver.
- **AC5 no browser / AC6 unknown anchor / AC8 renderability** — `TestScreenshot_NoCapturer_FailsLoud`, `TestScreenshot_MissingArgs_FailLoud` (no browser needed); `TestCapture_AnnotationAndFailLoud` unknown anchor; renderability gate in code (data-testid on error toast).
- **AC7 SPA not built** — `standUp` calls `CheckEmbeddedSPA` → BuildError.
- **AC9 injection-safety** — `TestAnnotateScript_InjectionSafe` (hostile `"`/`</script>`/U+2028 JSON-escaped).
- **AC10 non-screenshot unaffected** — `TestBuild_NoScreenshot_CapturerUntouched`; verified `rela docs build` of a typeref-only manual builds with Chrome hidden.
- **AC11 example manual** — committed + builds.
- Frontend: 1359 unit tests pass after the `data-testid` addition. `go test ./...` full suite green (75 pkgs). `just coverage-check` PASS (docscapture floor 40% documented; total 76.3%). `golangci-lint`/`just arch-lint`/`just lint-md` clean.

## Quality

- [x] Code follows project patterns — consumer-side `Capturer` interface (in `docs`, impl in `docscapture`, injected by CLI); the seed replays one `applySeed` against both stores; annotation anchoring reuses the existing `#field-<prop>` id (no SPA field-hook edit needed)
- [x] Checked for DRY opportunities — one `applySeed`, one `overlayJS` embed, shared chrome-discovery; no premature abstraction
- [x] No security issues introduced — annotation text is JSON-marshaled (never string-spliced JS, DR-C2); sandbox-first browser launch; temp project torn down on Close; server on 127.0.0.1 ephemeral; seed is fixture-only
- [x] No silent failures — fail-loud everywhere; the renderability gate closes the fail-OPEN hole the spike hit (DR-S4)
- [x] No debug code left behind — spike + probe files removed
