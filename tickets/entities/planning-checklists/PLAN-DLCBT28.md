---
id: PLAN-DLCBT28
type: planning-checklist
title: 'Planning: Remote MCP toggle and tool filter in .rela/config.yaml'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a `remote_mcp:` block in `.rela/config.yaml` that turns
rela-server's HTTP MCP endpoint on or off, combined with `-mcp` and `RELA_MCP`;
`remote_mcp.tools.allow/deny` to limit the endpoint's tools; docs. Out: scoping
or disabling `lua_eval` for SaaS (the saas-provisioning-hardening work; the
remote endpoint has no Lua tools anyway); filtering resources and prompts;
tenant-mode specifics.

**Acceptance Criteria:**
1. The endpoint is served only when no source says off and at least one says on.
2. `-mcp=false` and `RELA_MCP=0` say off; another RELA_MCP value stops startup.
3. Allow, deny and deny-over-allow filter tools/list, calls and watch replays.
4. An unknown tool name or a misspelled config key stops startup.
5. docs/mcp-server.md and docs/server-security.md state the precedence.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small config change)
- [x] ~~Searched for existing libraries~~ (N/A: config plumbing)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal setting)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `validation:` in `.rela/config.yaml` with the
`RELA_VALIDATION` override (internal/appbuild/validationmode.go) is the
precedent for a deployment setting. All MCP tools register through `addTool`
(internal/mcp/toolspec.go), the one place a filter needs to hook.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `project.Config.RemoteMCP` with a tri-state `Enabled
*bool` and `Tools{Allow,Deny}`, decoded strictly. cmd/rela-server resolves flag
(a tri-state `toggleFlag`), env and config in `resolveRemoteMCP`; an off wins.
`mcp.WithToolFilter` skips filtered tools in `addTool` and drops them from the
watch replay set; names are checked against a fixed catalogue pinned to the
`newTool` calls by an AST test.

Alternatives rejected: `data-entry.yaml` (committed screen config, shared by
every deployment of a project; this is a deployment setting); `remotes.yaml`
(sync remotes, unrelated); a new `.rela/server.yaml` (a second file for one
block, when config.yaml already holds deployment settings); env-only (cannot be
pinned off against a later env change).

**Files to modify:** internal/project/config.go, internal/mcp/{toolfilter,
toolspec,tools,server}.go, internal/dataentrywire/mcp.go,
cmd/rela-server/{main,mcp}.go, .go-arch-lint.yml, tests, GUIDE-mcp-server,
GUIDE-server-security, GUIDE-cli-reference.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Operator-authored `.rela/config.yaml`,
`RELA_MCP`, `-mcp`. Unknown keys, a misspelled top-level `remote_mcp`, an empty
allow list, an unknown tool name and an unknown RELA_MCP value all stop startup.
`allow` is the recommended form because it stays closed on upgrade.

**Security-Sensitive Operations:** Exposing the MCP endpoint. Fail-safe
precedence (off wins). The existing JWT requirement is unchanged. The filter
narrows tools only; resources and prompts still read under the ACL, which is
documented.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1: TestResolveRemoteMCP (all 27 combinations),
TestPlanRemoteMCP. AC2: TestEnvToggle, TestToggleFlag. AC3: TestWithToolFilter,
TestWithToolFilter_FilteredToolIsUnreachable, assertToolFilterApplies over HTTP.
AC4: TestWithToolFilter_UnknownNameIsRefused, TestLoadConfig_RemoteMCP,
TestKnownToolNames_MatchesRegistrations.

**Edge Cases:** Null block, aliases, merge keys, whitespace in names, a tool
whose feature is off, Lua tool names, tool lists without the endpoint.

**Negative Tests:** Misspelled keys, `allow: []`, `RELA_MCP=true`, `-mcp=sure`,
unknown tool names.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** `RELA_MCP=true` used to mean off and now stops startup; deliberate,
documented. A `remote_mcp` mistake fails every reader of config.yaml;
documented. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-mcp-server, GUIDE-server-security,
GUIDE-cli-reference.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None separate; the code review (cranky reviewer)
covered the design and its findings are recorded as review responses.
