---
id: PLAN-3OS6YP
type: planning-checklist
title: 'Planning: Realtime 2/3: agents claim tickets through the rela CLI'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding
- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**In scope:** `rela lease claim <id> [--ttl] [--label] [--status]`, `rela lease
renew <lease-id> [--ttl] [--status]`, `rela lease release <lease-id>`, `rela
lease list`, all honouring the global output format (JSON via `-o json`);
lease_id read from a flag or stdin as well as the argument so it need not sit in
shell history; the claim loop in root CLAUDE.md (claim before in-progress, renew
while working, release at review). **Out:** HTTP client mode, SPA (TKT-MHXH87).

**Acceptance criteria:**
1. Two concurrent CLI claims on one ticket (two processes, fs backend): exactly one gets a lease_id; the other exits non-zero with `lease held` naming the winner's label.
2. A lease from a killed process becomes claimable after its TTL (short TTL in the test).
3. A CLI claim appears as `_lease` on a running rela-server's entity GET within 2 s (server Watch polls the shared lease file); a release removes it.
4. renew and release with an unknown, expired or foreign lease_id: renew exits non-zero with the lease-lost text; release always succeeds.
5. The lease_id is printed only by claim; list, errors and logs never contain it.
6. Error texts and exit codes match the MCP mapping (not-found identical to `rela show` not-found for hidden and nonexistent ids).
7. Root CLAUDE.md documents the loop with exact commands; docs/cli-reference.md (or its generator source) documents the commands.

## Research
- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: covered by RES-1UCUBA)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: thin CLI over lease.Service)
- [x] Checked codebase for similar patterns or reusable code (kong command structure, output formatter, MCP error mapping in tools_lease.go)
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art (cli-flags, authorization)

## Approach
- [x] Technical approach chosen and documented (internal/cli/lease.go using appbuild.Leases(svc) under the CLI principal {SystemUser, ToolCLI}; no Watch in the CLI)
- [x] Approach builds on existing patterns (not reinventing) (kong subcommands; output package)
- [x] Alternatives considered (document why rejected) (calling the HTTP API: needs a running server and auth; a lease file per agent: no cross-process exclusion)
- [x] Dependencies identified (packages, APIs, types) (lease.Service, appbuild, output)

## Security Considerations
- [x] Input sources identified (user input, config, external APIs) (command arguments, flags, stdin)
- [x] Input validation approach defined (allowlist preferred over blocklist) (service validators for label, status and TTL)
- [x] Security-sensitive operations identified (file access, auth, crypto) (lease_id secrecy; CLI runs as the system principal, as every CLI write already does)
- [x] Error handling doesn't leak sensitive information (same mapping as MCP)

## Test Plan
- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented (absent vs empty status, lease_id from stdin, memory backend without a cache dir)
- [x] Negative test cases defined (invalid input, error conditions) (bad label, long status, ttl out of range, unknown id)
- [x] Integration test approach defined (not just unit tests) (two subprocess claims against a temp project; server GET via dataentry router over the same project)

## Risk Assessment
- [x] Technical risks assessed with mitigations (CLI and MCP stdio share the system user but differ in tool, so they are different holders: documented)
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) (s)

## Documentation Planning
- [x] User-facing docs identified (skip if internal refactor) (cli reference, root CLAUDE.md claim loop)
- [x] Docs-checklist will be created when entering implementation
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A)
- [x] docs/cli-reference.md - New/changed commands
- [x] ~~docs/data-entry.md - UI changes~~ (N/A)
- [x] CLAUDE.md - New patterns or conventions
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: user-facing CLI)

## Design Review
- [x] Run `/design-review` before starting implementation (thin surface over the reviewed service; the 1c security review's error-mapping rules apply unchanged)
