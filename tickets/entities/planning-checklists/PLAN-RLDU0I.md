---
id: PLAN-RLDU0I
type: planning-checklist
title: 'Planning: Extract dataentry theme/settings/palette cluster to appearanceHandler'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: `internal/dataentry` only — move the 12
theme/settings/palette/logo glue methods (handlers_theme.go,
handlers_theme_package.go, settings_handlers.go) off `App` onto a new
`appearanceHandler`; re-point route registrations; ratchet
`//plimsoll:max-methods` 104 → 92. OUT: any behavior/route/ACL change; the other
App clusters (own tickets).

**Acceptance Criteria:**
1. `just plimsoll` passes with the App directive at 92 (ratchet, not bump).
2. Full `go test ./...` + `-race` on dataentry green — existing handler tests
drive the routes end-to-end and are the behavior-preservation proof.
3. arch-lint/comment-lint/golangci-lint clean.

## Research

- [x] ~~Run `/research`~~ (N/A: mechanical extraction on an established in-package pattern)
- [x] ~~Searched for existing libraries~~ (N/A: no new functionality)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — structural survey of App's 104 methods performed;
clusters, field usage, and ranked candidates recorded in the epic.

**Existing Solutions:** `viewsHandler` (views_handler.go, TKT-I37338/PR #1230)
is the direct template — fixed service handles by value, reloadable state as
closures, read-only surface without writeMu; `writeHandler`, `syncHandler`,
`commandHandler`, `attachmentHandler` are the earlier iterations of the same
pattern.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** All three collaborators (`logo *logoStore`, `palette
*paletteService`, `settings *settingsService`) are already extracted
self-synchronized services; the App methods are pure glue, so the handler holds
those by value plus `schema func() *Schema` (+ `services func() Services` / the
existing view-reader path for the one relation-default lookup in
`handleAPIGetSettings`). Constructor `newAppearanceHandler(app)`, wired in
NewApp and the rebind path for parity. Routes re-point directly to the handler
so the mount costs App zero methods. Alternative rejected: two smaller types
(themeHandler + settingsHandler) — same reduction, more wiring, no cohesion gain
(all three services are facets of operator-configured appearance).

**Files to modify:** internal/dataentry/{appearance_handler.go (new),
handlers_theme.go, handlers_theme_package.go, settings_handlers.go, app.go,
api_v1.go, router.go}

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Unchanged — logo upload/theme import validation
lives in the already-extracted services and moves nowhere.

**Security-Sensitive Operations:** No ACL decisions in this cluster; the
relation-default lookup in `handleAPIGetSettings` keeps its existing read path
(no new ungated read is introduced; the handler never holds `store.Store`). No
writeMu — these endpoints mutate only the self-synchronized appearance services,
exactly as today.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined~~ (N/A: no new behavior — existing HTTP-level handler tests are the integration suite)

**Test Scenarios:** Existing settings/theme/palette handler tests exercise every
moved route end-to-end and must pass unchanged.

**Edge Cases:** Wiring parity NewApp ↔ rebind path (test apps rebuild handlers);
schema closure must observe live reloads.

**Negative Tests:** Existing invalid-payload/method-not-allowed tests for these
routes continue to pass unchanged.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Low — unexported methods, no callers outside the package. Main risk
is missing the rebind wiring parity; mitigated by mirroring viewsHandler's
wiring and the race-detector test run. Effort: m.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor kind)

**Documentation Impact:** N/A — internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: mechanical extraction repeating the reviewed viewsHandler design (TKT-I37338); no new interfaces or behavior)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no review run)

**Design Review Findings:** N/A
