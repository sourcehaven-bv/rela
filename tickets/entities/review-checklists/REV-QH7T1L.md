---
id: REV-QH7T1L
type: review-checklist
title: 'Review: Custom apps: sandboxed-HTML extensions served in the data-entry SPA via a REST-API bridge'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./internal/...` → 60 packages ok, 0 fail;
frontend `vitest run` → 1085 passed; e2e apps.spec → 5/5)
- [x] Lint clean (frontend `npm run lint` → 0 errors; `go vet` clean;
`just arch-lint` OK after adding the `nethtml` vendor allowance)
- [x] Coverage maintained (`just coverage-check` → PASS; new files
apps.go 81.6%, apps_handler.go 80%, validate.go 81.4%, config.go 97.2%)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Critical: RR-U6W39V (csp_origins validation), RR-8R0W0E
(meta-CSP comment bypass — verified repro, fixed with a real tokenizer).
Significant: RR-L4TT3L (appSdk tests), RR-L29M1S (handshake ev.source check),
RR-BA1YCP (iframe :key
+ AbortSignal). Minor: RR-G2AYPR, RR-CRVARI, RR-7D2WOP, RR-PI5G9K.
All 9 addressed. (Earlier design-review RRs RR-ZOLWMD/RBAZSX/YLG57K/68HMJP also
addressed in the planning phase.)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 (injected <meta> CSP + headers) — PASS (TestHandleV1App,
TestInjectCSPMeta).
- AC2 (bad/unknown id → 400/404) — PASS (TestHandleV1App).
- AC3 (traversal-resistant load) — PASS (TestLoadAppHTML_Traversal).
- AC4 (app acts only as the user) — PASS (relaBridge.test.ts + e2e
"app reads through the bridge" under the real server).
- AC5 (closed allow-list, no passthrough) — PASS (relaBridge.test.ts
unknown_method / path-like).
- AC6 (iframe isolation + bridge-only) — PASS (e2e "iframe sandboxed without
allow-same-origin" + appSdk handshake tests + CSP assertions).
- AC7 (relations linkable via bridge) — PASS (e2e "app writes a relation").
- AC8 (cross-origin write → 403) — PASS (e2e "cross-origin write rejected").
- Plus: comment-bypass regression — PASS (TestInjectCSPMeta_CommentBypass +
appSdk.test.ts).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs written
inline this phase — docs/data-entry.md "Custom apps" section,
docs/data-entry/api-reference.md `_apps/{id}`, internal/dataentry/CLAUDE.md apps
rules. A formal docs-checklist adds no coverage here.)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A — see above)

**Docs Checklist:** N/A (docs completed inline)

## Final Checks

- [x] Commit message explains the why, not just what (pending commit)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use (example app +
docs + `rela.*` SDK reference)

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending /pr -->
