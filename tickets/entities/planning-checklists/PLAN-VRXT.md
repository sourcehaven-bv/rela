---
id: PLAN-VRXT
type: planning-checklist
title: 'Planning: data-entry: per-request Principal from HTTP header'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** the data-entry server stamps every request with
`Principal{User:"unknown", Tool:"data-entry"}` because recording the server
process owner for every human web user would be misleading. The audit-log PR
(#763) introduced a `PrincipalResolver` func-type seam in
`internal/dataentry/router.go` specifically so a header-aware resolver could
replace the default later.

**Scope (in):**

- New constructor `HeaderPrincipalResolver(headerName string) PrincipalResolver` in `internal/dataentry/router.go`.
- New constructor `EnvPrincipalResolver() PrincipalResolver` that reads `$RELA_DATAENTRY_USER` (local-dev escape hatch).
- A composing helper `ChainResolvers(...)` that tries each in order, returning the first non-`"unknown"` User.
- CLI flag `--principal-header X-Forwarded-User` on `cmd/rela-server/main.go` (default empty → header disabled).
- `App.SetPrincipalResolver(...)` setter (matches the existing `SetSecurityConfig` shape) so the wiring is symmetric.
- Header value sanitization at the resolver: trim, length-cap (256 chars), strip C0/DEL control chars. (Reuses the policy `audit.Filesystem` already applies; documented same.)
- Docs:
  - `docs-project/entities/guides/GUIDE-audit-log.md` — replace the "data-entry user is unknown" prose with the new wiring + trust-boundary warning.
  - `docs/security.md` (hand-written, not generated) — slot under the "Audit logging" section: a paragraph on the trust boundary, when the header is safe to trust, and the env override for local dev.

**Scope (out):**

- OAuth / OIDC integration. Separate ticket.
- Multi-user authorization (ACL). Identity only; policy is a follow-up.
- Cookie / session storage. Stateless header-based attribution is the minimal viable step for proxied deployments.
- Per-request Tool override (request stays `Tool: "data-entry"`).
- Mutation of `Principal.Tool` from the header — the proxy controls *who*, not *which entry point*.

**Decisions (confirmed with user):**

1. **Config: CLI flag only.** `cmd/rela-server` takes `--principal-header X-Forwarded-User`. Default is empty (header reading disabled). Operators opt in explicitly; matches the rest of rela-server's CLI surface.
2. **Missing header → "unknown".** When the header is configured but absent on a request, fall through to "unknown" (or to `$RELA_DATAENTRY_USER` if set). Reasoning: a misconfigured proxy must not lock out legitimate users; audit just records "unknown" until the proxy is fixed.
3. **Sanitization: trim + 256-char cap + control-char strip.** Defense against header-injection that could corrupt the JSONL stream. Reuses the policy already applied at `audit.Filesystem`.

**Acceptance Criteria:**

1. **AC1: header populates Principal.User when configured.** `--principal-header X-User`, request with `X-User: alice` → audit record carries `principal.user="alice"`.
2. **AC2: missing header falls through to "unknown".** `--principal-header X-User`, request without the header → audit carries `principal.user="unknown"`, `principal.tool="data-entry"`. Request succeeds (no 401/403).
3. **AC3: empty header value falls through to "unknown".** `X-User: ""` → `"unknown"`. Whitespace-only same.
4. **AC4: header value is sanitized.** `X-User: "alice\nbob"` → audit carries `principal.user="alice bob"` (newline replaced); a `X-User` of 1000 chars truncates at 256 in the audit record.
5. **AC5: `$RELA_DATAENTRY_USER` env override wins.** Env set + header absent → audit uses env. Env set + header present → env wins. (Env is the local-dev escape hatch — it shouldn't be silently shadowed by an incoming header from a dev proxy.)
6. **AC6: no flag → defaultPrincipalResolver behavior unchanged.** Backwards-compatible: existing deployments continue to record `"unknown"`.
7. **AC7: Tool remains "data-entry"** regardless of header / env — the header controls User only.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing solutions:**

- **`net/http`** — `r.Header.Get(name)` is sufficient; no library needed.
- **Existing seam in `internal/dataentry/router.go`** — `PrincipalResolver func(*http.Request) principal.Principal` is the func type the middleware already accepts. New resolvers are 5-line constructors.
- **`audit.Filesystem.clean`** (`internal/audit/filesystem.go`) — same sanitization policy used for audit field values. We'll lift the logic into a tiny shared helper or just duplicate (4 lines) — the audit policy is package-private and importing it from dataentry would cross a domain boundary.
- **Reverse-proxy header conventions:** `X-Forwarded-User` (oauth2-proxy, Vouch, traefik forward-auth, many SSO proxies) is the most common; `Remote-User` (mod_auth) is older but still in use. We let the operator pick the name via flag.

**Codebase prior art (file:line refs):**

- `internal/dataentry/router.go:91-117` — the `PrincipalResolver` type, `defaultPrincipalResolver`, and `stampAuditPrincipal` middleware that the new resolver plugs into.
- `internal/dataentry/principal_test.go` — pattern for testing resolver wiring (uses `httptest.NewRequest` + a capture handler). The follow-up tests slot in next to the existing `TestStampAuditPrincipal_CustomResolver`.
- `cmd/rela-server/main.go:80-110` — existing flag parsing + `app.SetSecurityConfig(...)` wiring. `SetPrincipalResolver(...)` follows the same shape.
- `internal/audit/filesystem.go:175-220` — `clean()` / `truncateRunes()` / `isControlRune()` — the sanitization functions we'll mirror.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:**

1. **In `internal/dataentry/router.go`:**

   ```go
   // HeaderPrincipalResolver reads principal.User from headerName on
   // each request. Empty headerName disables the resolver (returns
   // the zero principal so a chained resolver can take over).
   //
   // Trust boundary: the header is only as trustworthy as the
   // reverse proxy that sets it. Operators serving data-entry
   // without a trusted proxy must not enable this resolver, or
   // anyone can spoof identity by setting the header on the wire.
   func HeaderPrincipalResolver(headerName string) PrincipalResolver {
       if headerName == "" {
           return func(*http.Request) principal.Principal {
               return principal.Principal{}
           }
       }
       return func(r *http.Request) principal.Principal {
           user := sanitizeHeaderValue(r.Header.Get(headerName))
           return principal.Principal{User: user, Tool: principal.ToolDataEntry}
       }
   }

   // EnvPrincipalResolver reads principal.User from $RELA_DATAENTRY_USER.
   // Returns the zero principal when unset — chain it with other
   // resolvers via ChainResolvers.
   func EnvPrincipalResolver() PrincipalResolver { /* ... */ }

   // ChainResolvers returns a resolver that tries each in order,
   // returning the first non-empty User. Tool is taken from the
   // first matching resolver; falls back to ToolDataEntry.
   func ChainResolvers(resolvers ...PrincipalResolver) PrincipalResolver { /* ... */ }
   ```

The flag-driven wiring chains env *first* (local-dev wins), then header, then
default.

2. **In `internal/dataentry/app.go`** — add `App.principalResolver PrincipalResolver` field + `SetPrincipalResolver(r PrincipalResolver)` method. `NewRouter` already takes the resolver; pass `a.principalResolver` if non-nil, else `defaultPrincipalResolver`.

3. **In `cmd/rela-server/main.go`** — new flag:

   ```go
   principalHeader := flag.String("principal-header", "",
       "HTTP header to read for audit Principal.User (e.g. X-Forwarded-User). "+
           "Default empty: do not read any header. "+
           "WARNING: the header is only as trustworthy as the upstream proxy.")
   ```

At startup, construct the chained resolver and
`app.SetPrincipalResolver(resolver)` before `app.NewRouter()`.

4. **Sanitization:** small helper `sanitizeHeaderValue(s string) string` in `router.go` (or a sibling file) — trim, cap at 256 chars (UTF-8 safe), replace C0/DEL with space. Returns "unknown" if the result is empty. The audit backend's sanitization runs separately as defense-in-depth; we duplicate here so the *Principal value seen by readers of the in-memory record* is also clean (Memory backend is used by tests).

**Files to modify:**

| File | Change |
|---|---|
| `internal/dataentry/router.go` | Add `HeaderPrincipalResolver`, `EnvPrincipalResolver`, `ChainResolvers`, `sanitizeHeaderValue`. |
| `internal/dataentry/router_test.go` (new) or extend `principal_test.go` | AC1-AC7 tests. |
| `internal/dataentry/app.go` | Add `principalResolver` field + `SetPrincipalResolver`. Thread through to `NewRouter`. |
| `cmd/rela-server/main.go` | Add `--principal-header` flag; construct + set resolver before `NewRouter`. |
| `docs-project/entities/guides/GUIDE-audit-log.md` | Replace the "User is unknown" prose under "data-entry" with the new behavior + trust-boundary warning. |
| `docs/security.md` | Add a paragraph under "Audit logging" about the trust boundary, the flag, and the env override. |

**Alternatives considered (rejected):**

- **OAuth/OIDC inline.** Too large for "small effort"; ticket would balloon. Header-based is the minimum step that unblocks proxied deployments.
- **Cookie / session.** Stateful; requires login UI; tied to a frontend story we haven't designed. Header is stateless and orthogonal.
- **Per-request Tool override.** Out of scope — `Tool: "data-entry"` correctly identifies the entry point regardless of who's behind the proxy. Future Tool diversification (e.g. an admin console) would be a separate Tool constant.
- **Both flag and env for `--principal-header`.** User-confirmed: flag only. Env path is only for User value, not header name.
- **Reject on missing header (401).** User-confirmed: fall through. Misconfigured proxy must not lock out users.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input sources & validation:**

- **HTTP header value** (`r.Header.Get(headerName)`) — operator-controlled at the proxy layer. Sanitization: trim, truncate at 256 chars (UTF-8 safe), replace `\x00-\x1f` and `\x7f` with space. No allowlist on the value itself — usernames vary widely (locale, dotless-i Turkish characters, etc.); the truncate + control-char strip is sufficient.
- **Header name** (`--principal-header` flag) — set once at server startup by the operator. Not user-controlled at runtime. We do *not* validate the header name — operators picking nonsensical names get nonsensical behavior, which is operator-visible.
- **`$RELA_DATAENTRY_USER`** — operator-controlled at process start. Same sanitization as the header value.

**Trust boundary (documented in security.md):**

> The `--principal-header` flag tells data-entry to trust an incoming
> HTTP header for the audit Principal. This is only safe behind a
> reverse proxy that *strips* the same header from inbound requests
> and *sets* it from an authenticated source (oauth2-proxy, Vouch,
> traefik forward-auth, etc.). A direct-to-data-entry deployment must
> not enable this flag — clients can spoof the header at will.

**Security-sensitive operations:**

- None new on the data-entry side — the header value flows into the audit log only. Audit is forensic; a wrong value in the User field shifts blame in the log but doesn't grant access.
- The flag's existence has a subtle CLI-discoverability risk: an operator might enable it on a non-proxied deployment. Help text and the security.md paragraph explicitly call this out.

**Error handling:**

- Header-not-present → "unknown" (no error).
- Header-malformed → sanitized (no error).
- `$RELA_DATAENTRY_USER` set to garbage → sanitized (no error).
- No path returns an error that exposes operator config.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test scenarios (one test per AC):**

| AC | Test |
|---|---|
| AC1 | `TestHeaderPrincipalResolver_PopulatesUser` — flag set, header present, capture-handler asserts `principal.User == "alice"`. |
| AC2 | `TestHeaderPrincipalResolver_AbsentHeaderFallsThrough` — flag set, no header, expect `"unknown"` + `200 OK`. |
| AC3 | `TestHeaderPrincipalResolver_EmptyHeaderFallsThrough` — `X-User: ""` and `"   "` cases. |
| AC4 | `TestHeaderPrincipalResolver_Sanitizes` — newline, tab, 1000-char username; verify truncation + replacement. |
| AC5 | `TestChainResolvers_EnvWinsOverHeader` — set env, set header, expect env value. And: env absent, header present → header value. |
| AC6 | `TestDefaultResolverUnchanged` — without `--principal-header`, identical behavior to today. |
| AC7 | `TestHeaderPrincipalResolver_ToolUnchanged` — Tool field is always `ToolDataEntry`. |

**Edge cases:**

- Header value is a single byte (`X-User: a`) → recorded verbatim.
- Header value is exactly 256 chars → unchanged.
- Header value is 257 chars → truncated to 256.
- Header value contains a NULL byte → replaced with space.
- Header value contains a multi-byte UTF-8 codepoint that straddles the 256-char boundary → truncation is rune-aware (UTF-8 safe).
- Header name is set but missing on the request → "unknown" (AC2).
- Header sent on a path that doesn't go through the middleware (static assets) → middleware applies to *all* routes per `NewRouter`'s outer wrap; documented behavior.
- Two header values (`X-User: alice; X-User: bob`) — `r.Header.Get` returns the first; documented.

**Negative tests:**

- `--principal-header ""` is the same as not passing the flag (covered by AC6).
- `$RELA_DATAENTRY_USER=""` → no override (whitespace stripping).
- Operator sets `--principal-header` to a header name that contains invalid HTTP chars → `r.Header.Get` returns "" (Go's normalization handles it); no panic.

**Integration test:**

- `internal/dataentry/principal_test.go` extends to drive a full router with a stub handler that asserts the ctx-stamped Principal. Not just the middleware in isolation — the whole stamp middleware + chained resolver + handler.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| Operator enables `--principal-header` on a non-proxied data-entry → spoofable identity in audit log | Help text + security.md explicit warning. Defense-in-depth: data-entry binds loopback by default; non-loopback warning exists already. |
| Header injection via newline / control char → corrupts JSONL audit stream | Resolver-side sanitization (this PR) + audit.Filesystem sanitization (existing). Two layers. |
| Operator confuses env override and header configuration | Help text on the flag explicitly names the env var; security.md paragraph treats them as a unit. |
| Per-test isolation: tests using `t.Setenv` for `RELA_DATAENTRY_USER` interfering with each other | `t.Setenv` resets per subtest; standard Go testing pattern. |

**Effort:** **s**. ~30-50 LOC of resolver code + tests, ~20 LOC of wiring, ~30
LOC of docs.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation impact:**

- [x] `docs-project/entities/guides/GUIDE-audit-log.md` — under "data-entry user is unknown" section: explain the flag, env override, trust boundary. Regenerate `docs/audit-log.md`.
- [x] `docs/security.md` (hand-written) — slot under "Audit logging": one paragraph on the trust boundary + flag + env override.
- [ ] `docs-project/entities/guides/GUIDE-data-entry.md` — small note in the audit section pointing to security.md for the flag. Regenerate.
- [ ] CLI help text — the flag's own `Usage:` text covers it.
- [ ] CLAUDE.md — no new patterns; the resolver seam was already documented in the audit-log section.

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** TBD — run /design-review before implementation.
