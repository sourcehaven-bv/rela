---
id: PLAN-7MUPPD
type: planning-checklist
title: 'Planning: Realtime 2/1b: lease service with access checks, rename/delete hooks, wake'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding
- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**In scope (1b):** lease.Service over the 1a Store:
Claim/Renew/Release/ListMine/Get-for-display, access checks, holder identity,
input validation, caps, rename/delete hooks through the alias fanout, Subscribe
wake channels, Watch(ctx) (version poll + expiry timer), appbuild wiring
(appbuild.Leases(s)), Watch started in rela-server, desktop and `rela mcp`.
**Out:** MCP tools, HTTP routes, `_lease` on responses (TKT-27XVYS), presence
and SPA (TKT-MHXH87), CLI (TKT-B610IP).

**Acceptance criteria:**
1. Claim on a hidden id and on a nonexistent id return the same not-found error (wrapping store.ErrNotFound); a faced type with a bare id returns *visibility.AmbiguousAddressError naming readable faces only.
2. Claim on a readable entity without update rights returns a permission error without holder details; with rights, a held entity returns *HeldError with user, tool, label, since, status.
3. Renew after read access is revoked, after delete, and after delete + recreate returns ErrLost and removes the lease; a recreated entity shows no holder.
4. Release returns the same result for an existing, foreign or unknown lease_id.
5. lease_id is 128-bit crypto/rand, returned only by Claim; the store receives only its sha256; a captured log of every operation contains no lease_id and no status text.
6. Validation: TTL 30 s..1 h (default 15 min), status <=140 runes without control, CR/LF or bidi characters, label ^[A-Za-z0-9_.-]{1,32}$; violations are input errors before any store call.
7. Rename moves the lease, delete drops it, face delete leaves it; each wakes subscribers.
8. Subscribe: two subscribers both wake on one claim; wakes coalesce; cancel stops delivery. Watch wakes on a write by another Store instance (version change) and at the earliest expiry; Watch is not started by one-shot CLI commands.
9. NewService rejects nil collaborators.

## Research
- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: covered by RES-1UCUBA)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-process service over existing seams)
- [x] Checked codebase for similar patterns or reusable code (piles.Service hooks, gatedGraphReader WriteTarget/Resolve, mcp.WriteAuthorizer, changefeed.Subscription.Wake, scheduler.StartBackground lifecycle)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: lease semantics settled in RES-1UCUBA)
- [x] Reviewed relevant rela concepts for prior art (authorization, faces DEC-NPZICR, uniform 404)

## Approach
- [x] Technical approach chosen and documented (.ignored/phase2-plan.md "Service (1b)" and "1b seams")
- [x] Approach builds on existing patterns (not reinventing) (piles.Service, consumer-side interfaces, appbuild package accessor)
- [x] Alternatives considered (document why rejected) (ShareKey as holder: changes on token refresh; pushing lease events into changefeed.Bus: a polled change has no types; reaper goroutine: expiry is lazy, a timer suffices)
- [x] Dependencies identified (packages, APIs, types) (lease Store, visibility read gate, acl.ACL, principal, appbuild, cmd/rela-server, cmd/rela-desktop, internal/cli)

## Security Considerations
- [x] Input sources identified (user input, config, external APIs) (id/address, ttl, label, status, lease_id from MCP/HTTP/CLI callers)
- [x] Input validation approach defined (allowlist preferred over blocklist) (label allowlist, rune-bounded status with character rejection, TTL bounds, address parsing via entity.ParseAddress)
- [x] Security-sensitive operations identified (file access, auth, crypto) (read gate + update authorization, crypto/rand lease ids, hashing)
- [x] Error handling doesn't leak sensitive information (uniform not-found; permission error without holder; generic store-full; no lease_id/status in logs)

## Test Plan
- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented (renew exactly at expiry, claim by the same identity re-entering, faced entity with one face, rename onto an id with a lease)
- [x] Negative test cases defined (invalid input, error conditions) (hidden/nonexistent/denied/ambiguous, invalid ttl/label/status, foreign lease_id)
- [x] Integration test approach defined (not just unit tests) (service over kvlease with a real policy ACL and gated reader built through appbuild in a temp project)

## Risk Assessment
- [x] Technical risks assessed with mitigations (goroutine leak from Watch: owned by caller ctx and tested; rela mcp lifecycle change: cancel on Serve return; alias fanout ordering: leases after piles, errors logged like others)
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) (m)

## Documentation Planning
- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: no surface yet; docs land with TKT-27XVYS)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no user-facing change)
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] N/A - Internal change, no user-facing docs needed

## Design Review
- [x] Run `/design-review` before starting implementation (covered by the TKT-6QC5EL design reviews, whose 23 findings define this slice; seams verified by a code survey)
