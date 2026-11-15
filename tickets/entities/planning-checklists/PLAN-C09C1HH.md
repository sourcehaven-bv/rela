---
id: PLAN-C09C1HH
type: planning-checklist
title: 'Planning: Tenant mode as the SaaS profile: scope features that are unsafe in a shared process'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: make `-tenant-mode` the SaaS switch for shell `commands:`,
`/api/git/sync`, process-wide `secrets.yaml` and `ai.yaml`, Lua `http.*` to
internal addresses, unbounded Lua memory and `lua_eval`; apply the same profile
in the CLI when it opens a tenant; document it in `docs/server-security.md`.
Always-refused Lua destinations and memory bounds apply in every mode. Out:
webhook HMAC, persisting the mail outbox across eviction, running flows of an
evicted tenant (documented as known residue).

**Acceptance Criteria:** The ticket has no separate section; these are written
from its description and design decisions.
1. In tenant mode every shell command is refused (403
`commands_disabled_in_tenant_mode`) and the UI lists none.
2. In tenant mode `/api/git/sync` answers "git not configured".
3. In tenant mode Lua, flows, the token broker and connections cannot read
`secrets.yaml` or `ai.yaml` unless `-tenant-shared-host-config` is set;
request-path scripts included.
4. In tenant mode Lua `http.*` refuses loopback and private addresses unless
`-lua-http-allow` names them; link-local, metadata and multicast are refused in
every mode, including through NAT64 and IPv4-compatible forms.
5. Lua memory is bounded in every mode, and `string.format` cannot exceed
the cap.
6. A tenant's remote MCP lists no Lua tool; an unauthenticated
`POST /hooks/<id>` gets 401.
7. The CLI applies the same profile when it opens a tenant, and fails closed.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: effort m; the choices are recorded in the ticket's design decisions)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: the filter uses the standard library `net/netip` and a dialer `Control` hook; no library search was recorded)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: not recorded for this ticket)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A. Context from RES-D54281 (multi-tenant SaaS shape).

**Existing Solutions:** the `commands:` authorizer seam in
`internal/dataentry/commands.go`, the host config and seams in
`internal/appbuild`, the Lua sandbox and `http.go` client, the remote MCP
switch, and the webhook auth path.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** See TKT-3RWQ1PK "Design decisions". Rejected
alternatives recorded there: an allowlist flag outside tenant mode (would
suggest a restriction that does not exist), loopback blocking in every mode
(breaks documented single-team use), a method on `App` for git sync (method
cap).

**Files to modify:** `cmd/rela-server/{main,serve,tenantmode}.go`,
`internal/appbuild/{appbuild,seams,withheld,tenantprofile}.go`,
`internal/lua/{bounds,http,httppolicy,runtime,context,sandbox}.go`,
`internal/dataentry/{app,commands,configure_wiring}.go`,
`internal/dataentrywire/wire.go`, `internal/script/runtime.go`,
`internal/cli/tenant.go`, `docs-project` server-security, Lua and postgres
guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** tenant-authored Lua scripts (destination checked
at connect time in the dialer, after DNS; strict policy is an allowlist via
`-lua-http-allow`, a bad entry stops startup); operator flags and env
(`RELA_LUA_HTTP_ALLOW`, `RELA_TENANT_SHARED_HOST_CONFIG`, refused outside tenant
mode); `string.format` arguments (Lua 5.1 conversion before Go fmt).

**Security-Sensitive Operations:** shell execution (denied), git sync
(disabled), operator secrets and AI keys (withheld, mail excepted), outbound
HTTP from Lua (SSRF filter), memory use (bounded). Blocked-address errors do not
name the address (`TestClassifyHTTPError_BlockedHidesAddress`).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. `TestCommandExecTenantModeRefused`, `TestBuildCommandAuthorizer_TenantMode`.
2. `TestDisableGitSync`.
3. `TestWithWithheldHostConfig`, `TestWithWithheldHostConfig_SurvivesReassembly`,
`TestServices_RequestScriptsSeeWithheldHostConfig`,
`TestNewWriterRuntime_RequireHostConfig`.
4. `TestAddrPolicy_Permits`, `TestEmbeddedIPv*`, `TestLuaHTTP_Policy`,
`TestLuaHTTP_PolicyRefusesBeforeConnect`, `TestParseHTTPAllowList`.
5. `TestLuaBounds_*`, `TestBoundedStrFormat_*`.
6. `TestTenantMode_SaaSProfile` (postgres integration).
7. `TestApplyTenantProfile`, `TestTenantProfile_ApplyFailsClosed`,
`TestOpenProject_TenantModeFailsClosed`.

**Edge Cases:** DNS names resolving to internal addresses, NAT64 and
IPv4-compatible IPv6, environment proxies in strict mode, `%[n]` and `*` in
format strings, registry overflow, a second `SetHTTPPolicy` call, schema reload
keeping the withheld config.

**Negative Tests:** bad `-lua-http-allow` entry, tenant-only flags outside
tenant mode, nil host config in tenant mode, oversized format output, table
arguments to `%s`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Breaking single-team Lua scripts that call local services (mitigated:
loopback and private stay reachable outside tenant mode). Process-global Lua
policy set more than once (mitigated: equal second call accepted, different one
refused). A path that builds Lua deps without the host config (mitigated:
`script.RequireHostConfig` fails closed in tenant mode). Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (not created yet: docs were updated on the branch; the docs-checklist is created before done)

**Documentation Impact:** `docs/server-security.md`, `docs/lua-scripting.md` and
`docs/postgres-backend.md` (generated from `docs-project` guides).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (not run: the ticket was written alongside the implementation; the code was reviewed by the rela-security-reviewer agent and an automated reviewer instead)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** No design-review findings. Security review findings,
all addressed: RR-A2FK6GR, RR-N5ZY6L4, RR-0Z8GN02, RR-1IBQECU.
