---
id: IMPL-6LZROW
type: implementation-checklist
title: 'Implementation: Permission-based navigation filtering (UX: hide menu entries a user cannot use)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Changes:

- `dataentryconfig/config.go` — `NavigationEntry.Permission`.
- `dataentryconfig/validate.go` — reject `permission:` on a group entry.
- `dataentry/views_handler.go` — `permitsNavEntry` (closed switch mirroring
`authorizeCommand`), `viewsHandler.aclImpl` + `currentACL`, filtering and the
empty-group drop in `handleV1Sidebar`.
- `dataentry/app.go` + `test_helpers_test.go` — wire the late-bound `aclImpl`
closure at both construction sites.
- `frontend/src/types/config.ts` — `permission?` (plus the `search?`/`settings?`
fields that were already missing from the TS mirror).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`sidebarShape` is a new helper because the existing `sidebarCountsByLabel`
records only items with a non-nil `Count` — it literally cannot express "this
entry is absent", which is the property every test here turns on.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Live `rela-server` against `prototypes/data-entry/project` with a gated `Admin
Only` nav entry (`permission: admin:read`) granted to alice only:

| Check | Result |
|---|---|
| alice (holder) sidebar | `Admin Only` present |
| bob (non-holder) sidebar | absent; all ungated entries intact |
| bob → `/api/v1/tickets` (the hidden entry's target) | **200** — filtering enforces nothing |
| `/_config` alice vs bob | byte-identical (md5) |
| bob's `/_config` | still lists the gated entry |
| No `acl.yaml` at all | entry shown; browser-verified in the sidebar with its count |

The 200 and the identical `/_config` are the two that matter: they demonstrate
this is presentation, not a boundary, and that nothing is concealed.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Notes:

- **A test caught a real wiring gap.** `test_helpers_test.go` builds its own
`viewsHandler` under a comment claiming it "mirrors production wiring"; it did
not pass `aclImpl`, so gated entries were hidden from *everyone* (the
fail-closed nil arm). Fixed at the helper so the comment stays true, rather than
working around it in the tests.
- **`TestMdCorpusRoundTrip` (in `internal/lua`) failed on my own planning
document**, not on code: a wrapped line began `403.`, which the markdown parser
reads as an ordered-list item, so render(parse(x)) != x. Reworded. Worth knowing
that every `tickets/**` file is parser-validated by the test suite.
- `just lint` flagged British spelling twice (`behaviour`) — Go code is
US-spelling-linted; the prose docs are not.
- Docs were edited in `docs-project/entities/guides/` and regenerated, per the
generated-file trap hit in TKT-M1AX6P.
