---
id: PLAN-773SN0
type: planning-checklist
title: 'Planning: Reject reserved system:* principals at the API boundary (system:scheduler impersonation)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: a reserved-prefix predicate in `internal/principal`; rejection of `system:*`
as an acting user on every HTTP-derived principal source (header, env, verified
assertion — the last covering both the SPA/API and remote MCP via the JWT gate);
403 + security log; a guard so a reserved name cannot be persisted as a
provisioning stub's `principal_property`.

OUT: how the ACL resolves or grants `system:scheduler` (matching the
`assignments:` entry is correct behaviour for the real scheduler); the
migration's `read: ["*"]` grant; `run_as:` in `schedules.yaml`; the CLI/stdio
MCP path (operator-shell trust boundary, as for `db migrate`).

**Acceptance Criteria:** see the ticket — 8 criteria, each mapped to a test
scenario under Test Plan below.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — effort `s`, single well-understood boundary check; the
codebase survey below was sufficient and no approach question remained open.

**Existing Solutions:**

- No library involved: this is a namespace reservation over an internal
identity, not a parsing or crypto problem.
- **Prior art in-tree — `sanitizeUser`** (`internal/dataentry/router.go:715`) is
the established input filter for principal names from HTTP, already shared by
the header, env and verified-assertion sources. The new check is the same kind
of filter at the same boundary, so it should live with it rather than in a new
layer.
- **Prior art — `isUnstamped`** (`internal/acl/request.go:268`) already rejects
two reserved-ish sentinel names (`""`, `"unknown"`) at the ACL boundary. It
demonstrates the pattern of a name that is structurally invalid as an acting
identity; it does not extend to `system:*` because those names ARE valid for the
scheduler.
- **Reserved-namespace precedent generally**: prefix reservation (`system:`,
`webhook:`) already exists implicitly in this codebase — `webhook.go:166` mints
`"webhook:" + claims.Event`. The prefix convention is established; only the
enforcement is missing.
- Concepts reviewed: `authorization`; DEC-O59WM4 (scheduler identity and why it
is fixed and grantable); DEC-ZBI39P (capability vs identity separation — this
ticket is squarely on the identity side).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **`internal/principal`** — add the namespace and its predicate:

   ```go
   // ReservedPrefix namespaces identities that only rela's own internal
   // entry points may assert. UserScheduler and UserProvisioner are its
   // current members.
   const ReservedPrefix = "system:"

   // IsReserved reports whether user names an internal identity. Request-path
   // entry points MUST reject a reserved user: these names are grantable in
   // acl.yaml, so accepting one from the wire is identity spoofing.
   func IsReserved(user string) bool {
       return strings.HasPrefix(strings.TrimSpace(user), ReservedPrefix)
   }
   ```

Trim before the prefix test so a leading-space variant cannot slip past;
`sanitizeUser` already trims, but `IsReserved` must be correct standalone since
`provision.go` will call it on an unsanitized path.

2. **Reject at the HTTP boundary.** Two explicit call sites, because the single
`sanitizeUser` chokepoint cannot express "reject loudly" — it signals unusable
by returning `""`, which callers treat as fall-through, and decision 2 requires
a 403 rather than a downgrade to the next resolver:

   - `ChainResolvers` (`router.go:710`) — covers header + env. On a reserved
user, do not fall through to the next resolver; surface the rejection.
   - `verifiedPrincipal` (`router.go:606`) — covers the deprecated resolver and,
critically, the production JWT gate (`jwtgate.go:161`), which **re-stamps after
the chain**. Returning `ok=false` here already routes to `g.deny(w,r)` with a
log line (`jwtgate.go:163`), which matches the required behaviour; the log
message needs to distinguish "reserved" from "unusable after sanitization".

Mechanism for the chain path: `PrincipalResolver` returns a bare `Principal`
with no error channel, and a reserved user must not fall through. Simplest
change that preserves the existing shape is to have `stampAuditPrincipal`
(`router.go:743`) check the resolved principal and respond 403 directly, rather
than threading a new error type through every resolver. That keeps the resolver
signature untouched and puts the HTTP response where the other middleware
responses live.

3. **Provisioning guard** — in `maybeProvision`/`buildStubEntity`
(`provision.go:74,133`), refuse to build a stub whose `principal_property` would
be a reserved name. This is defence in depth: with step 2 in place a forged
`system:*` never reaches provisioning, but the invariant "a reserved name never
becomes durable graph state" should not depend on an upstream check. The
internal `system:provisioner` stamp is the *actor*, not the join key, so it is
unaffected.

**Alternatives considered:**

- **Guard inside `sanitizeUser` only** — the true single chokepoint (all three
sources pass through it). Rejected: it can only return `""`, i.e. silent
fall-through, which decision 2 rules out. Worth a comment there pointing at the
real check so the next reader does not assume it covers this.
- **Guard inside `principal.With`** — genuinely one point, but `With` is used by
the scheduler, provisioner and CLI, which legitimately need `system:*`. It would
need a source discriminator threaded through every caller; far more invasive
than two boundary checks.
- **Guard in `internal/acl` (`ForPrincipal`/`computeGlobals`)** — tempting since
`policy.Assignments[m]` is where privilege is actually conferred, but the ACL
cannot distinguish the real scheduler from a forged one; it has no notion of
request provenance. Rejecting there would break the scheduler outright.
- **Allowlist rather than blocklist** — the general preference, but inapplicable:
the set of legitimate human usernames is open (any IdP subject). The closed set
here is the reserved namespace, so a prefix denial IS the tight specification.

**Files to modify:**

- `internal/principal/principal.go` — `ReservedPrefix`, `IsReserved`.
- `internal/dataentry/router.go` — reject in `stampAuditPrincipal`; reject in
`verifiedPrincipal`; comment on `sanitizeUser`.
- `internal/dataentry/jwtgate.go` — distinguish the reserved-principal log line.
- `internal/dataentry/provision.go` — stub guard.
- Tests: `internal/principal/principal_test.go`,
`internal/dataentry/principal_test.go`, `jwt_principal_test.go`,
`asserted_identity_test.go`, `provision_e2e_test.go`.
- Docs: `docs/server-security.md`, `docs/scheduled-tasks.md`,
`docs/acl-security.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Source | Location | On a reserved value |
|---|---|---|
| Proxy header (`--principal-header`) | `router.go:468` | 403 + log; no fall-through |
| `$RELA_DATAENTRY_USER` | `router.go:490` | 403 + log |
| Verified JWT `sub` | `router.go:606` → `jwtgate.go:161` | `ok=false` → `g.deny` + log |
| Webhook `claims.Event` | `webhook.go:166` | Unaffected — mints `webhook:`, a different namespace. Confirm it cannot produce `system:` (it concatenates a fixed prefix, so it cannot). |
| Scheduler `run_as:` | `internal/scheduler` | Unaffected — operator config, in-process, inside the trust boundary |

On blocklist-vs-allowlist: the reserved namespace is the closed set, so denying
it is the precise rule; an allowlist of permitted usernames is not expressible
against an arbitrary IdP.

**Security-Sensitive Operations:**

- The whole change IS the auth boundary. Failure direction is toward *less*
access: a false positive locks out a `system:`-shaped IdP subject (visible,
loud, recoverable); a false negative is privilege escalation. The prefix rule is
chosen so the dangerous direction requires an explicit code change.
- Error text names the *reserved* status but never echoes policy contents.
Per CLAUDE.md the config is not a secret, so naming the rule is correct and aids
the operator debugging a proxy misconfiguration.
- Log lines carry the attempted name, source and remote address — all
already-known or attacker-supplied values, no secret material.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|---|---|
| 1 | Integration: router with `HeaderPrincipalResolver`; `X-Remote-User: system:scheduler` → 403, handler never invoked, ctx principal is not `system:scheduler` nor `unknown` |
| 2 | `t.Setenv(RELA_DATAENTRY_USER, "system:provisioner")` → 403 |
| 3 | **Validly signed** JWT (existing test-key harness in `jwt_principal_test.go`) with `sub: system:scheduler` → denied. Guards against the `requireVerifiedJWT` re-stamp bypass |
| 4 | Table over `system:scheduler`, `system:provisioner`, `system:future`, `system:` |
| 5 | Request to `/api/v1/_mcp` with a reserved asserted subject → denied (asserts `toolForPath` path is covered) |
| 6 | Unit: scheduler in-process stamp still yields a working ACL request for `system:scheduler` |
| 7 | `provision_e2e_test.go`: forged reserved subject creates **no** stub; and the existing internal-provisioner test still passes |
| 8 | Assert one log record per rejection with source + remote addr |

Integration approach: exercise through the assembled router (`NewRouter` +
`httptest`) as the existing `asserted_e2e_test.go` and `provision_e2e_test.go`
do, not just the resolver functions in isolation — the JWT-gate re-stamp bug is
only visible end-to-end.

**Edge Cases:**

- `"System:Scheduler"` — case. Decide explicitly: prefix match is
case-sensitive, so this is NOT reserved and resolves as an ordinary user with no
scheduler grant (ACL lookup is exact). Document; do not case-fold, which would
over-reject.
- `" system:scheduler"` / trailing space — `IsReserved` trims, so reserved.
- `"system:"` bare — reserved (prefix matches).
- `"systemscheduler"`, `"my-system:scheduler"` — NOT reserved; must still work.
- Unicode lookalikes (e.g. fullwidth colon) — not the ASCII prefix, so not
reserved; they also match no `assignments:` key, so no escalation.
- Control chars — `sanitizeUser` turns them to spaces first; `"system:\x00sch"`
becomes `"system: sch"`, still reserved by prefix.
- Length cap: a 256-rune truncation cannot manufacture a `system:` prefix.
- Empty / whitespace-only — existing behaviour unchanged (falls through).

**Negative Tests:**

- Every reserved variant must fail **closed** with 403 — explicitly assert the
principal did not silently become `"unknown"`, since that is the tempting wrong
implementation.
- Assert an ordinary user is unaffected (no regression in the common path).
- Assert the internal scheduler and provisioner paths still succeed.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| An existing deployment's IdP issues a `system:`-shaped subject; that user is locked out after upgrade | Accepted per decision 1. Loud 403 + log names the reason, so diagnosis is immediate. Call out in the release note / `docs/server-security.md`. Fails toward less access, never more |
| Guard added only to the resolver chain, bypassed by the JWT gate's re-stamp | AC3 exists specifically to catch this; test uses a real signed assertion |
| Over-broad rejection breaks the webhook `webhook:` namespace | Different prefix; webhook path builds its principal directly and is explicitly out of the guarded path |
| `PrincipalResolver` has no error channel, tempting a signature change that ripples | Handle the response in `stampAuditPrincipal` instead; resolver signature untouched |

**Effort:** `s` — small, well-localized change; most of the work is test
coverage across four entry paths.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/server-security.md` — document the reserved `system:` namespace and
the upgrade note about a colliding IdP subject.
- [x] `docs/scheduled-tasks.md` — note that `system:scheduler` is assertable
only in-process, so the grant cannot be reached from the API.
- [x] `docs/acl-security.md` — the `system:provisioner` section should state the
same boundary guarantee.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel surface changed)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no command changed)
- [x] ~~`docs/data-entry.md`~~ (N/A: no UI change)
- [x] ~~`CLAUDE.md`~~ (N/A: the rule lives in the `IsReserved` godoc, where
  someone adding a new entry point will actually encounter it)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: effort-`s`
  boundary check with a single decided approach; the design questions that
  would have surfaced — prefix vs constants, rejection mode, CLI scope — were
  put to the user directly and answered before implementation. A code review
  runs in the review phase.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None from a formal design review (skipped, above).
Two findings from the codebase survey materially changed the plan and are
recorded in the ticket:

1. **A guard in the resolver chain alone is insufficient** — `requireVerifiedJWT`
   re-stamps the principal AFTER `stampAuditPrincipal`, so a check only in
   `ChainResolvers` is bypassed whenever the gate is installed (the production
   config). Drove the second call site in `verifiedPrincipal` and AC3.
2. **A forged subject could be persisted** — `buildStubEntity` writes the
   subject verbatim as a stub's `principal_property`. Added the
   `maybeProvision` guard and AC7.

A third constraint surfaced during implementation and changed the design after
planning: **RR-T15E**. `stampAuditPrincipal` runs on EVERY request, so an
unconditional 403 would render the SPA shell and static assets as raw JSON under
a misconfigured proxy — locking operators out of the surface needed to fix it.
The rejection is therefore scoped: 403 on `/api/`, identity stripped to the
anonymous default elsewhere. Pinned by
`TestStampAuditPrincipal_ReservedOutsideAPIDegradesNotErrors`.
