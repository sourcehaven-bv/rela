---
id: IMPL-SJQJ5C
type: implementation-checklist
title: 'Implementation: Reject an unmatched verified principal''s writes (unmatched_principal policy key)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Feature implemented
- [x] Unit tests written
- [x] Integration tests written (real router, multi-write-path)
- [x] Edge cases handled

**What changed (reject-only; provision parked):**

1. `internal/acl/policy.go` — `UnmatchedPrincipal` key + `Unmatched{Anonymous,
Reject,Provision}` constants; `knownPolicyKeys` entry;
`validateUnmatchedPrincipal` (enum + the lookup-required load invariant),
extracted so `Validate` stays under the gocognit line. Exported
`PrincipalPropertyLookupEnabled` wrapper.
2. `internal/acl/request.go` — `WithUnmatchedVerified` / `UnmatchedVerifiedFrom`
ctx flag (mirrors the `WithRequest` idiom).
3. `internal/acl/declarative.go` — the reject branch in `AuthorizeWrite`, before
role evaluation; `provisionWarn sync.Once` for the reserved value.
4. `internal/dataentry/router.go` — `attachACLRequest`/`resolvePrincipalEntity`
take `jwtVerified` (from `a.jwtGate != nil`); set the flag on a genuine no-match
while the lookup is enabled.

## Manual verification

- [x] Each AC verified
- [x] Evidence documented

| AC | Test | Result |
|---|---|---|
| AC1 | `TestReject_AnonymousModeAllowsWrite`, `TestUnmatchedPrincipal_AnonymousDoesNotReject` | PASS |
| AC2 | `TestReject_DeniesEveryWritePath` (CRUD update, CRUD create, sync — through the REAL router) | PASS |
| AC3 | `TestReject_ReadStillAllowed` | PASS |
| AC4 | `TestReject_HeaderPrincipalUntouched`, `TestUnmatchedPrincipal_RejectIgnoresUnflagged` (scheduler) | PASS |
| AC5 | `TestUnmatchedPrincipal_RejectRequiresLookup` (6 cases) | PASS |
| AC6 | `TestUnmatchedPrincipal_ValidatesEnum` (unknown → load error; provision reserved) | PASS |
| AC7 | 403 discloses only the rule (RR-E12CN2 — docs corrected to match) | PASS |
| AC8 | existing `isUnstamped`/`ForPrincipal` tests unmodified + passing; matched/header untouched | PASS |

**The anti-bypass test is fault-injected.** Disabling the `AuthorizeWrite`
reject branch makes CRUD update (200), CRUD create (201), AND sync (200) all
succeed — the test catches every one with "reject is bypassed on this path".
Restored → pass. This is the systemic guard the design review demanded: the sync
path returning 200 under injection proves it genuinely reaches authz and is
genuinely gated (RR-BZQ049).

**Coverage-by-construction pinned:** `TestReject_ActionSharesWriteAuthz` asserts
Lua-action writes use the same `entityManager`;
attachment/rename/clone/relation/ conflict-resolve all reach
`Declarative.AuthorizeWrite` (verified in review), pinned at unit level by
`TestUnmatchedPrincipal_RejectDeniesFlaggedWrite`.

## Quality

- [x] Follows project patterns (policy-key idiom from asserted_role_assignments;
ctx-flag idiom from WithRequest)
- [x] No silent failures — misconfig is a load error

- `go build ./...` — clean
- `just test` (full) — pass
- `go test -race ./internal/acl ./internal/dataentry` — clean
- `just lint` — 0 issues; `just lint-md` — 0 issues
- `just arch-lint` — OK
- `just coverage-check` — PASS (76.6%)
- `generate-docs` diff — clean (docs edited at source)
