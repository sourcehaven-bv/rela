---
id: IMPL-NVXPNJ
type: implementation-checklist
title: 'Implementation: Reject reserved system:* principals at the API boundary (system:scheduler impersonation)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `TestIsReserved` (18-case table),
  `TestIsReserved_CoversEveryReservedConstant`, `TestVerifiedPrincipal_RejectsReserved`
- [x] Integration tests written (test full flow, not just units) — driven through
  the assembled router / real JWT-gate composition, not the resolver functions in
  isolation. `TestRequireVerifiedJWT_RejectsReservedSubject` is the one that
  matters: the gate re-stamps AFTER `stampAuditPrincipal`, so a resolver-only
  test would have passed against a bypassable implementation.
- [x] Happy path implemented — header/env/assertion rejection, 403 + log
- [x] Edge cases from planning handled — case sensitivity, whitespace padding,
  bare `system:`, near-miss names, unicode lookalike colon, control chars
- [x] Error handling in place (errors surfaced, not swallowed) — the rejection IS
  the surfaced error (403 + WARN log); nothing degrades silently on `/api/`

**Mutation-tested.** Each guard was individually neutralized and the suite
re-run to prove the tests actually catch its removal:

| Guard removed | Tests that failed |
|---|---|
| `principal.IsReserved` → `return false` | all 8 dataentry + both principal tests |
| only the `verifiedPrincipal` check | the 3 JWT-gate/MCP tests (the bypass this ticket nearly shipped) |
| only the `maybeProvision` check | `TestMaybeProvision_RefusesReservedSubjectDirectly` |

## Test Quality

- [x] Using fixture builders or factories for test data — reused the existing
  `newGateHandler` / `gateVerifier` harnesses and `provisionApp`; the latter was
  parameterized as `provisionAppWithSubject` rather than duplicated
- [x] No hardcoded values in assertions when object is in scope — assertions
  reference `principal.UserScheduler` / `principal.UserProvisioner`, never the
  literal strings, so renaming a constant cannot leave a stale test passing
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded — the
  `reservedUsers` table drives every rejection test
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran a real `rela-server` against a throwaway project (`/tmp/relaverify`) with
`--principal-header X-Remote-User` and an `acl.yaml` that models the actual
hazard — `everyone: read: []`, `system:scheduler: scheduler-system` with
`read: ["*"]` — plus one seeded ticket titled "Secret ticket".

**The vulnerability reproduced first.** A binary built with `IsReserved` forced
to `return false` (pre-fix behavior):

```
alice (everyone: read:[]):  {"data":[], ... "total":0}
forged system:scheduler:    {"data":[{"id":"TKT-001","_title":"Secret ticket", ...
```

A spoofed header read an entity the ordinary user cannot see. Real disclosure,
not a theoretical one.

**Same request against the fixed build:**

```
alice (everyone: read:[]):  {"data":[], ... "total":0}   [HTTP 200]
forged system:scheduler:    {"type":".../reserved_principal","title":"Reserved
  principal","status":403,"detail":"'system:scheduler' is an internal identity
  and cannot be asserted by a request."}   [HTTP 403]
```

Per-criterion results:

| AC | Result |
|---|---|
| 1 | `X-Remote-User: system:scheduler` → **403**, structured `reserved_principal` body |
| 2 | `system:provisioner` → **403** |
| 3 | Validly signed assertion with reserved `sub` → **denied** (automated; the gate harness is the only way to drive a real signature) |
| 4 | `system:anything` → **403**; bare `system:` covered in tests |
| 5 | Reserved subject on `MCPPath` → **denied** (automated) |
| 6 | Scheduler in-process identity unaffected — full `internal/scheduler` suite green |
| 7 | No stub provisioned for a reserved subject; internal `system:provisioner` write still works |
| 8 | `level=WARN msg="rejected reserved principal asserted by request" user=system:scheduler path=/api/v1/tickets method=GET remote_addr=127.0.0.1:49765` |

Non-regression, manually confirmed: `alice` → 200, `System:Scheduler` → 200
(case-sensitive by design), `my-system:scheduler` → 200.

**RR-T15E constraint verified live:** `GET /` carrying the forged header returns
**200** and serves the SPA. This is why the rejection is scoped to `/api/`
rather than applied unconditionally in `stampAuditPrincipal` — a misconfigured
proxy stamping a reserved name on every request must not render the UI as raw
JSON and lock the operator out of the surface needed to fix it.

## Quality

- [x] Code follows project patterns (check similar code) — `writeV1Error` for the
  structured 403 (matching every other 403 in the package); `slog` with the same
  `path`/`method`/`remote_addr` keys the JWT gate already uses; the `/api/`-vs-SPA
  split mirrors `attachACLRequest`'s existing RR-T15E scope rule
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced — the change only narrows access. A false
  positive locks out a `system:`-shaped IdP subject (loud, logged, recoverable);
  a false negative would be escalation, and the prefix rule makes that require a
  deliberate code change
- [x] No silent failures (errors logged AND returned) — every rejection both
  logs at WARN and returns 403/deny. The one deliberate non-error path (non-API
  downgrade) is documented at the call site and pinned by a test
- [x] No debug code left behind — verification project, temp binaries and all
  mutation backups deleted; `git status` shows only the intended files

**Also run:** `go test ./...` (green), `just lint` (0 issues), `just arch-lint`
(no warnings), `just plimsoll` (clean), `just coverage-check` (PASS, 78.1%).
Docs were edited in `docs-project/entities/` — the *source* — and regenerated via
`scripts/generate-docs.sh`, since `docs/*.md` carry an auto-generated banner and
would otherwise have been overwritten.
