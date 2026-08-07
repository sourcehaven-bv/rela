---
id: PLAN-TTFS8U
type: planning-checklist
title: 'Planning: Auto-provision a user entity for an unmatched verified principal'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Re-scoped to REJECT-ONLY after design review.** The original combined
> (reject + provision) plan hit two criticals; `provision` is split out and
> parked in `.ignored/provision-unmatched-principal-design.md`. This checklist
> now covers only `unmatched_principal: anonymous | reject`.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented with specific test scenarios

**Scope (reject-only):** the `unmatched_principal` policy key with `anonymous`
(default, byte-identical to today) and `reject` (deny an unmatched verified
principal's WRITES on the data-entry path). `provision` is a **reserved** third
value — accepted at load, behaves as anonymous + a one-time warn until its own
ticket. OUT: provision (parked), rejecting reads, org enforcement, isUnstamped
changes.

**ACs:** AC1-AC8 on the ticket.

## Research

- [x] ~~/research~~ (N/A: design decided with the user; two Explore passes +
a design review resolved the architecture)
- [x] Checked codebase for patterns / prior art

**Prior art / seams (verified file:line):**
- **The choke point:** every entity write on the data-entry path funnels through
`entitymanager` → `acl.ACL.AuthorizeWrite` (`manager.go` authorizeAndAudit,
declarative.go:172). Confirmed for CRUD, sync (`sync_handlers.go:134`
ApplyEntity), Lua-action (`actions.go:80` → entitymanager), and **attachments**
(`handlers_attachment.go:391` `AuthorizeWrite`). **git-sync is NOT an entity
write** (`handlers_git.go:86` — a git commit) and is correctly out of the
entity-ACL surface.
- **`asserted_role_assignments`** (TKT-RP3X3Q) — the idiom for a new `Policy` key
  + `knownPolicyKeys` + `Validate` enum check.

## Approach

- [x] Technical approach chosen
- [x] Builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**The seam (resolved — RR-BZQ049):** reject is a pure GATE (deny a write, no
write of its own), so it lives at the shared write-authorization point, not
per-handler. Carry the fact, not the transport:

1. **`internal/acl/policy.go`** — `UnmatchedPrincipal string
yaml:"unmatched_principal"` on `Policy`; `"unmatched_principal": true` in
`knownPolicyKeys`; `Validate` enum-checks {"", anonymous, reject, provision} and
rejects `reject` (or `provision`) without both `principal_property` and
`user_entity_type` set (else "unmatched" is undefined — the lookup is disabled
and everything looks unmatched, RR-9THKDO). `""` ≡ anonymous.

2. **A ctx flag for "verified-JWT unmatched."** The dataentry middleware is the
only place that knows BOTH facts: `a.jwtGate != nil` (wiring state — the
reliable "JWT is the identity source" signal, NOT a per-Principal marker, since
JWT and header both stamp Tool=data-entry, RR-9THKDO) AND that
`resolvePrincipalEntity` found no entity (`RawUser==""` after a lookup that was
enabled). In `attachACLRequest`, when the gate is installed, the lookup is
enabled, and no entity matched, mark a typed flag on ctx (a new
`acl.WithUnmatchedVerified(ctx)` / `acl.UnmatchedVerifiedFrom(ctx)` — internal
to acl, so entitymanager stays transport-agnostic). This is a READ-path stamp of
a boolean fact; it performs no write.

3. **`AuthorizeWrite` enforces reject.** In `Declarative.AuthorizeWrite`
(declarative.go:172) — reached by EVERY entity write — if
`policy.UnmatchedPrincipal == reject` AND the ctx carries the unmatched-verified
flag, return `Decision{Allow: false, RuleKind: "unmatched-principal", ...}`
before the normal role evaluation. Header/CLI/scheduler never set the flag →
never rejected (AC4). No `writeMu`, no manager-learns-transport, no per-handler
drift. The existing deny→403 rendering (`writeForbiddenIfACLDenied`) surfaces it
uniformly across CRUD/sync/action/attachment.

4. **`provision` reserved.** `Validate` accepts it; the enforcement point treats
it as anonymous and logs `slog.Warn` once ("unmatched_principal: provision not
yet implemented; behaving as anonymous"). Keeps the vocabulary stable so the
provision ticket doesn't churn the key.

**Files:** `internal/acl/policy.go` (+ tests), `internal/acl/declarative.go`
(the reject branch), `internal/acl/request.go` or a new small ctx-flag file (+
tests), `internal/dataentry/router.go` (set the flag in attachACLRequest), docs
source entities.

**Alternatives considered:** per-CRUD-handler hook (rejected — misses sync/
action/attachment, RR-BZQ049); a write-only middleware (heavier than needed for
a gate); blanket `isUnstamped` change (rejected — hits
scheduler/header/loopback).

## Security Considerations

- [x] Input sources / validation (allowlist enum in Validate)
- [x] Security-sensitive operations
- [x] Error handling doesn't leak

- **The flag is set only after signature verification + a real no-match.** It is
an internal ctx boolean, not attacker-influenceable.
- **Data-entry-path scope is the boundary.** Set only when `a.jwtGate != nil`.
A test pins that a scheduler/header/CLI principal is never flagged and never
rejected (AC4/AC8).
- **`reject`'s 403 leaks nothing** — generic forbidden; does not distinguish
"no IdP account" from "no graph entity" (AC7).
- **Fail-loud config.** `reject` without `principal_property`/`user_entity_type`
is a load error (AC5), not a silent per-request misbehaviour.
- **Reads unaffected** — documented posture: a graph-is-authority deployment
still lets an unknown read what its asserted roles allow; blocking reads is a
separate, larger choice (out of scope).

## Test Plan

- [x] Scenarios per AC
- [x] Edge cases
- [x] Negative cases
- [x] Integration

| AC | Test |
|---|---|
| AC1 | key absent / `=anonymous`: unmatched verified principal's write proceeds exactly as today. |
| AC2 | `=reject`: unmatched verified principal's write is 403 — **across CRUD, sync, AND Lua-action** (the anti-bypass test; drive each through the real router). Attachment covered by the same AuthorizeWrite. Matched principal's write proceeds. |
| AC3 | `=reject`: a GET by the unmatched principal is unaffected (anonymous read succeeds per its asserted roles). |
| AC4 | scheduler / header-mode / CLI / MCP principals are NOT flagged and NOT rejected (a scheduler write proceeds under `=reject`). |
| AC5 | `reject` without `principal_property` + `user_entity_type` → load error. |
| AC6 | unknown value → load error; `provision` accepted at load, behaves as anonymous + one warn. |
| AC7 | reject's 403 body carries no IdP/entity distinction. |
| AC8 | `isUnstamped` / shared `ForPrincipal` untouched (their existing tests pass unmodified). |

**The systemic anti-bypass test (AC2)** is the important one — echoing this
feature's history: drive an unmatched verified assertion through the REAL
NewRouter to a CRUD write, a sync write, AND a Lua-action write, and assert all
three 403 under `=reject`. This is the test whose absence would let a future new
write path silently reopen the bypass. Fault-inject it by removing the flag-set
or the AuthorizeWrite branch and confirm it fails.

**Edge:** lookup-disabled + reject → load error (not "everything rejected");
matched principal never flagged; `provision` value → warn-once + anonymous.

## Risk Assessment

- [x] Technical risks + mitigations
- [x] Security risks (above)
- [x] Effort estimated

**Risks:**
1. A future write path that does NOT go through `entitymanager.AuthorizeWrite`
would bypass reject (as git-sync is out-of-surface today). Mitigation: the AC2
multi-path test + the fact that AuthorizeWrite is the documented sole
write-authz point (CLAUDE.md). Any new entity-write that skips it is already a
bug independent of this feature.
2. The ctx flag must not leak into a context that outlives the request or into a
non-data-entry path. Mitigation: set only in attachACLRequest; it is a
request-scoped ctx value, gone when the request ends.

**Effort: s.** One policy key, one ctx flag, one `AuthorizeWrite` branch, tests.
No system principal, no migration, no write — all of that is in the parked
provision half.

## Documentation Planning

- [x] User-facing docs identified

- `docs-project/entities/guides/GUIDE-acl-security.md` — the `unmatched_principal`
key: the modes, the fail-closed default, the write-only reject scope + the
reads-still-allowed posture, and that `provision` is reserved/not-yet. **Edit
the source entity, not generated `docs/`** (lesson from TKT-RP3X3Q).
- No metamodel/CLI/UI surface change.

## Design Review

- [x] Ran `/design-review` before implementation
- [x] All critical/significant findings addressed or deferred

**Findings:** RR-BZQ049 (seam — addressed for reject, provision-half parked),
RR-9THKDO (guard/predicate — addressed: flag from wiring state + load
invariant), RR-9XBIJZ + RR-64WDUD (provision-only — deferred to the parked doc).
The review is what drove the split; reject ships with its seam resolved and
verified against code.
