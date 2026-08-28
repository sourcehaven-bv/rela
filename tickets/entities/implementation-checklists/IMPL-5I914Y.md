---
id: IMPL-5I914Y
type: implementation-checklist
title: 'Implementation: CalDAV deployment documentation'
status: done
---

<!-- @managed: claude-workflow v1 -->

Docs-only ticket; the code-shaped items are N/A with reasons.

## Development

- [x] ~~Unit tests written for new code~~ (N/A: documentation only, no code)
- [x] ~~Integration tests written~~ (N/A: documentation only)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place~~ (N/A: documentation only)

`docs/caldav.md`, linked from the README docs table. The "happy path" is the
zero-to-working-account walkthrough; the "edge cases" are the documented failure
modes — macOS reporting every setup failure as one useless message, the required
`https://` prefix, the three-request discovery sequence and which step to check
when the account connects but shows no lists.

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: documentation only)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The guide documents a topology that was actually built and run, not a
hypothetical one: Reminders → Pratique (TLS, PAT) → rela, with a real macOS
account syncing to-dos two-way.

Claims were verified against source rather than assumed:

- every `rela-server` flag in the guide checked present in `cmd/rela-server/main.go`
- Pratique's TLS confirmed merged on its `develop` (PR #4, "Terminate TLS
in-process: cert files + optional ACME"), satisfying AC4
- the JWKS path **corrected** to `/.well-known/pratique/jwks.json` after an
earlier draft placed it under the mount prefix (`internal/web/web.go:639`)
- the startup rejection of an unreachable completion value, and the 403 on an
unconfigured delete, confirmed as real code paths

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: prose)
- [x] No security issues introduced
- [x] ~~No silent failures~~ (N/A: documentation only)
- [x] No debug code left behind

Matches the existing guide style (`docs/postgres-backend.md`,
`docs/server-security.md`). States plainly that rela performs no CalDAV-specific
authentication or transport check, and why — AC3 — rather than leaving a reader
to assume a boundary that does not exist. No secrets in the examples: the PAT is
referenced, never shown, and the DSN stays env-only.
