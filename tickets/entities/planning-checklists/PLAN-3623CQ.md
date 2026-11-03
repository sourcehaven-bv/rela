---
id: PLAN-3623CQ
type: planning-checklist
title: 'Planning: Realtime 2/1c: lease MCP tools, HTTP routes, _lease on responses'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding
- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**In scope (1c-MCP):** MCP tools claim_entity, renew_lease, release_lease,
list_leases; `lease` on show_entity; lease wake in the watch tool;
mcp.Deps.Leases wiring for stdio and remote MCP; optional Type on
lease.ClaimRequest with Family-first ordering; GUIDE-mcp-server.md. **Out:**
HTTP routes, `_lease` on responses, live hub (TKT-4YPG4Q); SPA (TKT-MHXH87); CLI
(TKT-B610IP).

**Acceptance criteria:**
1. claim_entity on a hidden id and a nonexistent id returns text byte-identical to show_entity's not-found text.
2. Read without update returns the forbidden text with no holder fields.
3. Held returns `lease held:` with user, tool, label, since, expires_at, status.
4. renew_lease after expiry, revocation, delete, delete+recreate returns the same lease-lost text; release_lease always returns released:true.
5. show_entity carries `lease{user,tool,label,since,status}` only when held (no expires_at); absent otherwise.
6. A watch on show_entity returns when the lease is released.
7. lease_id never appears in show_entity, list_leases, error texts or captured logs (including renew/release error paths).
8. Golden tool list updated; tool descriptions mark status as untrusted holder text.
9. With Deps.Leases nil, no lease tools are registered and show_entity has no lease.
10. Claim with a Type that differs from the family type returns not-found before WriteTarget/authorize.

## Research
- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: covered by RES-1UCUBA)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-repo MCP tool surface)
- [x] Checked codebase for similar patterns or reusable code (comment tools with optional Deps, entityReadFailed, update_entity ambiguity text, watch ChangeSource loop)
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art (mcp-api, authorization)

## Approach
- [x] Technical approach chosen and documented (.ignored/phase2-plan.md "1c design" 1c-MCP and the folded security review)
- [x] Approach builds on existing patterns (not reinventing) (Deps optional collaborator like Comments; tools_*.go layout)
- [x] Alternatives considered (document why rejected) (Option-based wiring: would stay bound to the startup assembly; adapter pumping fake store events into watch: needs a goroutine per watch)
- [x] Dependencies identified (packages, APIs, types) (lease.Service, appbuild.Leases, mcp Deps, cli/mcp_wiring.go, cmd/rela-server/mcp.go, arch-lint grant mcp->lease)

## Security Considerations
- [x] Input sources identified (user input, config, external APIs) (tool arguments: id, ttl_seconds, label, status, lease_id)
- [x] Input validation approach defined (allowlist preferred over blocklist) (service validators: TTL bounds, label allowlist, status rune/character rules, lease_id shape)
- [x] Security-sensitive operations identified (file access, auth, crypto) (read gate, update authorization, lease_id secrecy)
- [x] Error handling doesn't leak sensitive information (not-found text identical to show_entity; held only after read+update; no lease_id in logs)

## Test Plan
- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented (renew with absent vs empty status, reload swapping Deps, nil Leases)
- [x] Negative test cases defined (invalid input, error conditions) (hidden/nonexistent/forbidden/held/lost/limit/invalid)
- [x] Integration test approach defined (not just unit tests) (MCP server over a temp project with acl.yaml and real lease service; watch test with a real release)

## Risk Assessment
- [x] Technical risks assessed with mitigations (typed-nil interface in Deps: assign only when non-nil; watch loop change: existing watch tests stay green)
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) (m)

## Documentation Planning
- [x] User-facing docs identified (skip if internal refactor) (GUIDE-mcp-server.md in docs-project, regenerated into docs/)
- [x] Docs-checklist will be created when entering implementation
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: CLI in TKT-B610IP)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: user-facing MCP tools)

## Design Review
- [x] Run `/design-review` before starting implementation (security design review of 1c: 7 minor/nit findings, folded into the plan; recorded as review responses)
