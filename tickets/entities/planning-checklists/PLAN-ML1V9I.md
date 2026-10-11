---
id: PLAN-ML1V9I
type: planning-checklist
title: 'Planning: OAuth token binding and Basecamp reference connector'
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: token store service (sealed KV on sqlite/postgres, desktop keychain), Go-side OAuth refresh from connections.yaml, rela.oauth, tokens capability, integration: principals, http.encode_query, rela token CLI, desktop Connections settings, examples/basecamp. Out: webhooks, sealing-key rotation, rich-text descriptions, comments and attachments.

**Acceptance Criteria:** AC1-11 in the Plan below, as revised by R1-R13.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: codebase survey by an Explore agent; design choices made with the user in session)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** secrets.yaml and keychainSecrets, state.KV backends, lock.Locker (pg advisory), Lua capabilities, golang.org/x/oauth2 considered (Launchpad style is non-standard; refresh needs rotation-safe persistence under a lock, so a small Go refresher was chosen).
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** See Plan > Design and the R1-R13 revisions.

**Files to modify:** See Plan > Files.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** connections.yaml (validated at load: https token_url, style enum, secret names), token names (allowlist grammar), CLI stdin token input, token endpoint responses (parsed strictly, expires_in bounded).

**Security-Sensitive Operations:** Sealing (AES-256-GCM with scoped AAD and key id), refresh token never exposed to Lua, single refresher under a shared lock, integration: prefix reserved against request spoofing, desktop places trust, errors free of URLs and secrets.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See Plan > Test plan.

**Edge Cases:** Rotation vs non-rotating providers, cancelled ctx after refresh, concurrent refresh across pools, stale invalidate, revoked consent, 429, vanished todos, wrong key, row moved across tenants.
<!-- List specific edge cases and expected behavior. Consider:
- Empty/null/missing values
- Boundary values (0, -1, MAX_INT)
- Special characters, unicode, null bytes
- Concurrent access
- Resource exhaustion
-->

**Negative Tests:** Ungranted token name, no store configured, wrong key, tampered ciphertext, reserved principal from requests, argv token input, untrusted desktop place.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** See Plan > Risks. Effort: l.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [x] docs/metamodel.md - New metamodel features
- [x] docs/cli-reference.md - New/changed commands
- [x] docs/data-entry.md - UI changes
- [x] CLAUDE.md - New patterns or conventions
- [x] README.md - Project-level changes
- [x] N/A - Internal change, no user-facing docs needed
-->

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 26 review responses linked via has-review-response, all addressed in plan revisions R1-R13.

## Plan


## Problem
A connector must call an OAuth API without a human present. OAuth refresh
tokens rotate: each refresh may return a new refresh token and invalidate the
old one. So the connector must store a new token at run time, and two processes
must never refresh the same connection at once, or one of them holds a dead
token. `secrets.yaml` is operator-authored and read-only, so it cannot hold a
rotating token. Background actions also cannot yet run as a connector
principal, and no reference connector exists to prove the FEAT-XYQMUB design.

## Scope
In:
1. `internal/tokenstore`: a token store service with per-host implementations.
   Server, CLI and scheduler: sealed (AES-256-GCM) rows in `state.KV`.
   Desktop: the OS keychain.
2. Lua `rela.oauth.access_token(name, refresh_fn)`: returns a valid access
   token; refreshes under a per-connection lock only when needed.
3. Capability `tokens: [names]` on schedules, automation actions and
   data-entry actions. Fail-closed like `secrets:`.
4. Principal namespace `integration:<name>`: reserved, accepted as `run_as` on
   background actions and scheduled tasks.
5. `http.encode_query(table)` (the Lua HTTP client has no URL encoding).
6. CLI `rela token set|status|delete <name>`; desktop settings section to paste
   a token.
7. `examples/basecamp/`: consent shell script, connector scripts (pull on a
   schedule, push from a background action), schema, acl.yaml, schedules.yaml,
   README. Tested in place against a stub Basecamp API on sqlite.
8. Docs: Lua guide (tokens, integration principals), CLI reference, scheduled
   tasks, ACL guide; the SM20FG sync section switches to `integration:`.

Out: inbound webhooks for pulls (follow-up ticket; Basecamp webhooks are
unsigned and need their own auth design); key rotation of the sealing key
(re-run consent); syncing Basecamp rich-text descriptions; comments and
attachments; generic OAuth flows inside rela (consent runs outside).

## Design

### Token store service
```go
package tokenstore
type Token struct {
    Refresh   string
    Access    string
    ExpiresAt time.Time // zero: unknown, treat as expired
}
type Store interface {
    Get(ctx, name Name) (Token, error) // ErrNotFound
    Put(ctx, name Name, t Token) error
    Delete(ctx, name Name) error
}
```
`Name` is a validated type: `[a-z0-9][a-z0-9_-]{0,62}`, refused not escaped.
Consumers declare their own narrow interface (Lua binding: Get/Put; CLI: all).

Implementations:
- `tokenstore.Sealed{KV state.KV, Key}`: one KV row per token at
  `tokens/<name>`. Value is version byte + nonce + AES-256-GCM ciphertext of
  the JSON token, with AAD `rela-token/v1/<name>` so a row cannot be moved to
  another name. On postgres the row lives in the tenant's `state_kv`, so all
  nodes share it; on sqlite it is in `rela.db`; on fs it is `.rela/tokens/`.
- Key source: `token_key` in `.rela/secrets.yaml` (base64, 32 bytes), else env
  `RELA_TOKEN_KEY`; secrets.yaml wins, like `smtp_password`. No key: the store
  is not configured; the binding raises "token store not configured" and the
  CLI errors with how to generate one (`openssl rand -base64 32`).
- Desktop: `keychainTokens`, entries under a `rela-tokens` service keyed by the
  document id and name, beside the existing keychain secrets. No sealing key
  needed; the keychain is the protection.
- Wiring: `appbuild.WithTokenStore(func(state.KV, HostConfig) (tokenstore.Store, error))`;
  the default builds `Sealed` from the host's secrets. `lua.WriteDeps` gains
  `Tokens`. Read-only runtimes (MCP, documents, sync automations) get none.

### Single refresher
`lua.WriteDeps` gains a `lock.Locker` (`lock.For(store)`): pg advisory lock on
postgres (cross-node), in-process mutex elsewhere (single process). Key
`oauth-refresh/<name>`. The lock is held across the refresh HTTP call. That is
not a `store.Tx`, so the "no slow I/O in a Tx" rule holds. The binding refuses
to run inside a Tx (`store.ContextInTx`).

### Lua binding
```lua
local tok, err = rela.oauth.access_token("basecamp", function(refresh_token)
  local resp = http.post(...)
  return {access_token = ..., refresh_token = ..., expires_in = 1209600}
end)
```
1. Get; if `Access` is set and `ExpiresAt` is more than 60 s away, return it
   (no lock).
2. Else acquire the lock, Get again (another node may have refreshed), repeat
   the check.
3. Else call `refresh_fn(refresh_token)`. It must return a table with
   `access_token` (non-empty string), optional `refresh_token` (kept if
   absent: providers that do not rotate) and optional `expires_in` (seconds,
   1..1 year; absent: 1 hour).
4. Put, release, return the access token.
On a refresh_fn error or bad return: nothing is stored, the lock is released,
and the binding returns `nil, err`. The refresh token is only ever passed to
the callback; no binding returns it. Errors never echo token values.
`rela.oauth.invalidate(name)` clears `Access` (for a 401 from the API) so the
next call refreshes.

Capability `tokens: [names]` (metamodel.Capabilities): `access_token` and
`invalidate` raise for a name not granted. `TrustedCapabilities` (rela script)
grants all. `rela.oauth` is registered only when the runtime has a store.

### Integration principals
`principal.IntegrationPrefix = "integration:"`, reserved like `system:`:
request entry points (proxy header, JWT, MCP) reject it via `IsReserved`.
Name grammar as tokenstore.Name. Accepted as `run_as` for background actions
(`validRunAs`) and scheduled tasks. Audit records `integration:basecamp`. ACL
`assignments:` may name it.

### HTTP helper
`http.encode_query({k = v, ...})`: keys sorted, values strings or numbers,
`url.Values.Encode`. Raises on other types.

### CLI and desktop
- `rela token set <name>`: reads a refresh token (or JSON
  `{refresh_token, access_token, expires_in}`) from stdin, never from argv.
- `rela token status <name>`: present or not, access expiry; never values.
- `rela token delete <name>`.
- Audit: `token-set`/`token-delete` records (name only), operator-shell trust
  like `history-purge`.
- Desktop settings: a "Connections" section to paste a refresh token by name
  and delete it.

### Basecamp reference connector (examples/basecamp/)
- `consent.sh` (curl only): prints the Launchpad authorize URL, reads the
  pasted code, exchanges it, pipes the refresh token into `rela token set
  basecamp`.
- Schema snippet: type `todo` with `basecamp: {type: external_ref, system:
  basecamp, sync: true}`, title, `due` (date), `done` (boolean).
- `basecamp-lib.lua`: token (via `rela.oauth.access_token` with the Launchpad
  refresh call), API calls with User-Agent and paging via the `Link` header,
  429 handling via `retry_after`, mapping todo <-> fields.
- `pull.lua` (schedules.yaml, `run_as: integration:basecamp`, every 5 min):
  lists the todolist's todos (open and completed), for each: find by ref,
  merge against `sync/basecamp`, create or write, push conflicts nowhere
  (records them in a `sync_conflict` property), tag only on a complete,
  conflict-free report (the SM20FG loop).
- `push.lua` (background action on todo create/update, `run_as:
  integration:basecamp`): merge, PUT/POST the push fields, completion via the
  completion endpoint, set the ref on create, tag with the pre-HTTP token.
- Config (account id, project id, todolist id, API base) from secrets so a test
  can point the base at a stub.
- `acl.yaml`: role for `integration:basecamp` with field grants on the synced
  fields and `tag:sync`.
- Test (`internal/appbuild`, sqlite tag): runs the shipped files in place
  against an httptest Basecamp stub: token refresh and rotation stored, pull
  creates, user edit pushes, remote edit pulls, conflict recorded, fixed point
  within two rounds, 401 triggers invalidate and one retry.

## Acceptance criteria
1. Sealed store round-trips; wrong key, tampered ciphertext and a row moved to
   another name fail to open; ciphertext holds no plaintext.
2. No key: binding raises "not configured"; CLI explains.
3. access_token: cached access returned without calling refresh_fn; expired
   calls it once and stores the rotated refresh token; absent refresh_token in
   the return keeps the old one; error from refresh_fn stores nothing.
4. Two concurrent callers (two pg pools) call refresh_fn exactly once.
5. Name not granted by `tokens:` raises; runtimes without the capability have
   no grant; read-only runtimes have no `rela.oauth`.
6. `integration:basecamp` accepted as run_as on schedules and background
   actions; rejected from proxy header, JWT and MCP; audited.
7. encode_query encodes reserved characters and sorts keys.
8. CLI set/status/delete; status never prints values; set refuses argv input.
9. Desktop keychain store passes the same tokenstore conformance tests (fake
   keychain).
10. Basecamp stub test passes all scenarios above.
11. consent.sh passes shellcheck and a stubbed run.

## Test plan
1-3 tokenstore unit tests + `tokenstoretest.RunAll` conformance shared by
Sealed (mem KV, statesql, pg) and keychain; 4 pg DB-gated test; 5 lua tests;
6 principal, scheduler config, metamodel and dataentry tests; 7 lua http test;
8 cli tests; 10 sqlite appbuild test; 11 shellcheck in CI if present, else a
bats-free shell test with curl stubbed via PATH.

## Files
New: internal/tokenstore (+ tokenstoretest), internal/lua/oauth.go,
internal/cli/token.go, cmd/rela-desktop token section, examples/basecamp/*.
Changed: metamodel/capabilities.go, lua/capabilities.go, lua/deps.go,
lua/http.go, lua/runtime.go, principal, metamodel/automationjob.go,
scheduler/config.go, appbuild (wiring, WithTokenStore), audit ops, guides.

## Risks
Lock held during HTTP pins a pg connection (refresh is rare; HTTP timeout
bounds it). Losing the sealing key loses tokens (re-run consent; documented).
A provider that rotates refresh tokens and the process dying after the HTTP
call but before Put loses the new token (re-run consent; documented; rare).
Basecamp API drift breaks only the example (stub-tested).

## Effort
L.

## Design review revisions (binding; they override the sections above)

R1 (findings 14, 3, 7, 12, 21) Refresh moves into Go. A project file
`connections.yaml` (operator-authored config, not secret) declares each
connection: `basecamp: {token_url, client_id_secret, client_secret_secret,
style: rfc6749|launchpad, user_agent}`; client id/secret are secret NAMES read
via HostConfig.Secrets(""). Lua gets `rela.oauth.access_token(name)` and
`rela.oauth.invalidate(name, rejected_access_token)`; no binding or callback
ever sees the refresh token. Go does: read, lock (60 s acquire timeout),
re-read, POST form-urlencoded body (never query), parse, Put on
`context.WithoutCancel` with its own timeout and one retry, release via defer.
Refresh HTTP errors are reported without URL or body. 400/401 invalid_grant
records `needs_consent` (name + time) and later calls fail fast until
`rela token set`. `expires_in` accepts number or numeric string; 0 or absent =
1 hour. Tests: cancelled ctx after the HTTP response still stores the token;
error text from a closed port contains no secret; blocked waiter times out
without refreshing.

R2 (finding 2) invalidate is compare-and-clear under the same lock: clears
Access only if it still equals the rejected token. `rela token set`/`delete`
take the lock too.

R3 (findings 1, 13) Desktop tokens reuse `keychainSecrets` with a `token/`
name prefix, so the places trust check applies; refresh and access token are
separate keychain items within `maxSecretBytes`. Test: untrusted place with a
copied document id gets "not trusted". consent.sh can print the refresh token
(no terminal echo of input) for pasting into the desktop Connections section.
The CLI's sqlite-lock error names the stop/set/start sequence.

R4 (finding 6, 18) Sealed store only on sqlite and postgres builds (and
desktop keychain). fs and memory tiers: not configured, matching "bare fs is
not supported for sync". Docs carry a tier table.

R5 (finding 5) One locker per Services, shared with attachments
(`SharedBase.attachmentLocker`), never `lock.For` per runtime.

R6 (findings 8, 9, 25) `lua` declares `OAuthTokens{AccessToken, Invalidate}`;
appbuild supplies it (tokenstore + locker + refresher). `rela.oauth` is
registered whenever the capability grants a token; without a configured store
it raises a typed "token store not configured". `NewSealed(kv, key, scope)`
rejects nil kv and wrong key length. Capabilities: `Tokens []string` plus
`AllTokens` (not YAML-settable); `Any()` counts tokens; `Fields()` call sites
updated; docs build gets no tokens; `tokens:` refused at load where it would do
nothing. Bindings live in an `oauthBindings` type (plimsoll). New arch-lint
component `tokenstore` (deps: state).

R7 (finding 10) Loop prevention stays convergence-based (feature decision), not
principal-based: push.lua first compares ours with the base locally and makes
no HTTP call when nothing differs. Stub test counts push-side HTTP calls after
a pull (expected zero once the pull tagged).

R8 (finding 11) 429: pull ends the run without moving tags; push returns an
error so RetryBounded backs off. `retry_after` filled from the header on 429
and 503. Stub scenario.

R9 (findings 4, 24) Push GETs the todo, re-sends every writable field it does
not own (description, assignee_ids, completion_subscriber_ids, starts_on,
notify), compares updated_at before PUT; README states the remaining window.
The stub PUT clears omitted fields like Basecamp.

R10 (finding 15) Extend `principal.IsReserved` to `integration:`; one shared
run_as validator for scheduler and metamodel (integration grammar; system: only
system:automation / system:scheduler where they apply).

R11 (findings 16, 17, 19) AAD `rela-token|v1|<scope>|<name>` with scope = pg
schema or project id; header carries a key id (HMAC of the key) so a wrong key
reports as such; status reports decrypt status. Name refuses Windows reserved
names. `token_key` read only from global secrets, refused in any
`capabilities.secrets` list.

R12 (findings 20, 23) Connector checks Link URLs share scheme, host and account
prefix with the API base; uses flat routes, grant_type parameters,
authorization.json with product bc3, two paged lists (pending + completed),
vanished todos recorded in sync_conflict, ETag on list GETs, User-Agent with
contact (stub enforces).

R13 (findings 22, 26) Guard uses `store.InTx`; AC tweaks folded in.
