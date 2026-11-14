---
id: TKT-3RWQ1PK
type: ticket
title: 'Tenant mode as the SaaS profile: scope features that are unsafe in a shared process'
kind: enhancement
priority: high
effort: m
status: in-progress
---

## Description

Tenant mode serves many orgs from one process. Several features are safe for one
customer but not for a shared process: shell `commands:`, `/api/git/sync`,
process-wide `secrets.yaml` and `ai.yaml`, Lua `http.*` to internal addresses,
unbounded Lua memory, and `lua_eval`. This ticket makes `-tenant-mode` the one
SaaS switch that turns these off or scopes them, and documents it in
`docs/server-security.md`.

The Lua SSRF filter and memory bounds apply in every mode; tenant mode adds the
strict destination policy.

## Design decisions

- **Lua HTTP.** Tenant mode sets the strict policy with `lua.SetHTTPPolicy`
  at startup. `-lua-http-allow` (`RELA_LUA_HTTP_ALLOW`) reopens named CIDRs,
  IPs and `host:port` entries; a bad entry stops startup. The flag is refused
  outside tenant mode, because there the policy is not strict and an
  allowlist would suggest a restriction that does not exist.
- **Shell commands.** Tenant mode uses its own command authorizer, which
  denies every command. The exec route answers 403 with code
  `commands_disabled_in_tenant_mode`. The same authorizer resolves the
  command list, so the UI offers no buttons. Startup logs the count.
- **Git sync.** `dataentry.DisableGitSync` clears git on each tenant's app
  before the router is built, so `/api/git/sync` answers "git not
  configured". A package function, because `App` is at its method cap.
- **Host config.** `appbuild.WithWithheldHostConfig` wraps the host config:
  secrets and `ai.yaml` return `lua.ErrHostConfigWithheld`. Lua
  `rela.secrets.*` raises and `ai.*` returns `not_configured`, both naming
  `-tenant-shared-host-config`. Flows, the token broker and connections see
  the same error. Mail reads the unwrapped config, because its SMTP password
  lives in `secrets.yaml` and mail is a shared operator service.
  `-tenant-shared-host-config` (`RELA_TENANT_SHARED_HOST_CONFIG=1`) shares
  them; it is refused outside tenant mode.
- **lua_eval over remote MCP.** The remote MCP switch is unchanged. A test
  pins that a tenant's remote MCP lists no Lua tool.
- **Webhooks.** A test pins that an unauthenticated `POST /hooks/<id>` in
  tenant mode gets 401. No HMAC work.
- **Scheduler, mail, flows, CLI.** Verified and documented with existing
  tests. Known residue, documented only: the mail outbox is in memory per
  resident tenant, so queued mail can be lost on eviction; flows of an
  evicted tenant wait until it is resident again.
- **CLI tenant profile.** `appbuild.TenantProfile` holds the shared part of
  the profile: the strict Lua HTTP policy and the withheld host config.
  rela-server builds it from its flags. The CLI builds it from
  `RELA_LUA_HTTP_ALLOW` and `RELA_TENANT_SHARED_HOST_CONFIG` in
  `openProject`, so every command that opens a tenant applies it: the
  scheduler, stdio MCP with `lua_eval`, and the data commands. Applying the
  policy fails closed: it is an error unless `lua.CurrentHTTPPolicy` reports
  a strict policy afterwards. `lua.SetHTTPPolicy` accepts a second call with
  an equal policy, because one CLI process may open a tenant more than once.
  `ForReassembly` keeps the withheld host config, so stdio MCP's schema
  reload does not drop it.
- **Lua format and NAT64.** `string.format` parses the Lua 5.1 format itself
  and formats each conversion with its own argument. Go's `%[n]` and `*` are
  refused, so one large argument cannot be repeated past the 16 MiB cap. The
  always-on address check also reads the IPv4 address inside local-use NAT64
  (`64:ff9b:1::/48`) and IPv4-compatible (`::/96`) addresses.
- **Every mode.** The always-refused Lua destinations (link-local, metadata,
  multicast) and the Lua memory bounds apply in every mode. Loopback and
  private addresses stay reachable outside tenant mode, because the Lua
  guide documents that single-team deployments call local services.
- **Request-path scripts.** The data-entry App builds its own Lua deps for
  actions, document renders and webhook scripts. `dataentry.SetLuaHost`,
  called from `dataentrywire.Services`, gives it the host config and seams
  of its services (`appbuild.LuaHost`), so they see the withheld config too.
  `TenantProfile.ApplyLuaPolicy` calls `script.RequireHostConfig`, so in
  tenant mode `script.NewWriterRuntime` refuses deps without a host config
  instead of reading the shared `.rela` directory. The fallback stays for
  the CLI, the desktop and tests, where `.rela` is the project's own.
- **Format arguments.** `string.format` converts each argument as Lua 5.1
  does before Go's fmt sees it: numeric verbs take a number, `%s` takes
  `tostring` of the value with `__tostring`, `%q` a string or number.
  Anything else is an argument error, so a table cannot print Go internals
  and the size estimate covers every string. No case of the gopher-lua
  comparison test changed.
- **No hex of strings.** With the conversion, `%x` and `%X` refuse a string
  that is not a number, which removes the 2x hex expansion of a large
  string.
