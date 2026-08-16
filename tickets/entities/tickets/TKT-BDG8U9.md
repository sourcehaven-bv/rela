---
id: TKT-BDG8U9
type: ticket
title: "Remote MCP part 2: serve the MCP endpoint over Streamable HTTP from rela-server"
kind: enhancement
priority: high
effort: l
tags:
    - needs-design
    - security
status: in-progress
---

## Description

Follow-up to TKT-UIR41P, which shipped the two prerequisites: the migration to
`modelcontextprotocol/go-sdk` v1.7.0 (spec revision `2026-07-28`) and the
ACL-gated read seam that every MCP read surface now goes through.

This ticket is the remaining half — actually exposing MCP over HTTP:

- Mount Streamable HTTP at **`/api/v1/_mcp`** inside the existing router, so it
  inherits `stampAuditPrincipal → requireVerifiedJWT → attachACLRequest`.
- Derive the principal **per request** instead of once at construction, and
  supply the gated read handles (`appbuild.Services.GatedReads()`) that
  TKT-UIR41P put in place.
- Expose **RFC 9728** Protected Resource Metadata plus a parameterised
  `WWW-Authenticate` challenge so spec-compliant MCP clients can discover the
  IdP.
- **Opt-in, off by default** — an upgraded server must not start serving MCP.
- Register a **vetted allowlist** of tools remotely (not a denylist): a new
  tool should be stdio-only until someone deliberately adds it. This is also
  the seam TKT-G3PPD needs.

## Why this is separate

TKT-UIR41P's work is coherent and independently valuable on its own: the SDK is
current, and the read path is gated and test-pinned even though `rela mcp` runs
with `NopACL`. Splitting keeps each PR reviewable as one idea, and keeps a
ticket from sitting in a non-terminal state while a large migration lands.

Everything below has **no stdio manifestation** — each only bites once there is
a network transport.

## Carried-over review findings (all still open)

- **RR-H8S10M** — `verifiedPrincipal` hardcoded `principal.ToolDataEntry`, so a
  remote MCP write would be audited as `data-entry`, failing the
  audit-attribution criterion. The naive fix (a `Principal` composite literal
  with `Tool: ToolMCP`) silently drops every asserted role, because
  `VerifiedFrom` is the only constructor that populates the unexported
  org/role/scope fields. *A tool parameter has already been threaded through
  `verifiedPrincipal` on the working branch; it needs the MCP call site.*
- **RR-PQ5UN1** — `acl.Request` memoises `globals` without synchronisation and
  is documented as not goroutine-safe, but `attachACLRequest` attaches ONE per
  HTTP request. A JSON-RPC batch may dispatch handlers concurrently over that
  shared request. Note the obvious test (`-race` with parallel calls) cannot
  fail: parallel HTTP requests each get their own `acl.Request`. The test must
  exercise concurrency *within* one request.
- **RR-P34E8J** — the RFC 9728 challenge is not satisfiable by adding a
  well-known endpoint alone. `jwtgate.go` emits `WWW-Authenticate` only when
  the configured header is literally `Authorization` (the default is
  `X-Auth-Assertion`), and never includes the `resource_metadata` parameter.

## Acceptance criteria

1. A deployed `rela-server` started with the MCP flag serves a working MCP
   endpoint over Streamable HTTP; a client can `server/discover`, list tools,
   and call a read tool.
2. With MCP not enabled the endpoint is absent — no behaviour change on upgrade.
3. Two callers with different ACL grants hit the same endpoint and each sees
   only their permitted entities, across tools, resources and prompts.
4. A write via remote MCP is audit-logged against the **requesting** principal
   with `Tool == mcp` (RR-H8S10M).
5. An unauthenticated request is refused with no fall-through to a spoofable
   header.
6. **Enabling remote MCP without verified JWT identity is refused at startup** —
   the CSRF exemption the endpoint needs is only sound while the JWT gate is
   active, and header-identity mode fails open to `unknown`.
7. 401 carries `WWW-Authenticate: Bearer … resource_metadata=…` and the
   well-known endpoint serves valid RFC 9728 metadata (RR-P34E8J).
8. Concurrency within a single JSON-RPC batch is safe and test-pinned
   (RR-PQ5UN1).

## Design notes carried forward

- **Stateless mode is required** for protocol `2026-07-28` in the go-sdk, and
  it returns 405 on GET/DELETE, rejects server→client requests, and delivers
  notifications only within an in-flight request. `2026-07-28` is reachable
  ONLY via `server/discover`; the legacy `initialize` path caps at `2025-11-25`.
- **Do not wire the store-event bridge to the remote transport.** The watcher's
  `resources/list_changed` has no stateless-HTTP equivalent; remote clients
  re-read on demand. Already documented in `internal/mcp/server.go`.
- **`/api/v1/_mcp` needs the `isCSRFExempt` treatment** (as `/api/sync/` and
  `/api/v1/_feeds/` have), because a non-browser MCP client sends no `Origin`.
  `middleware_security.go` states the exemption is structurally sound exactly
  when rela verifies a bearer token itself and requires it — which is AC #6.
- **Not tenant isolation.** `principal.OrgID()` is audit attribution only;
  nothing in `internal/acl` evaluates it (TKT-RP3X3Q). Docs must not imply
  otherwise.

## Implementation notes (verified against the code, 2026-08-16)

Read of `internal/dataentry` + the go-sdk before writing anything. Corrections
and load-bearing facts:

- **`verifiedPrincipal` has NO `tool` parameter on any branch.** An earlier note
  claimed the threading was already done on a working branch; it is not — checked
  both `tkt-uir41p-remote-transport` and this one. `router.go:588` hardcodes
  `principal.ToolDataEntry` at line 609, and has exactly two callers
  (`router.go:566`, `jwtgate.go:161`). `principal.ToolMCP` already exists
  (`principal.go:339`) and `VerifiedFrom` already takes `tool` positionally, so
  RR-H8S10M is a parameter-threading job, not a redesign.

- **`isAPIPath` already covers `/api/v1/_mcp`** (`router.go:234`), so registering
  on the `inner` mux inherits `stampAuditPrincipal → requireVerifiedJWT →
  attachACLRequest` with no middleware change. The mount is genuinely a
  registration, as designed.

- **The go-sdk seam is `getServer func(*http.Request) *Server`**
  (`streamable.go:232`). This is what makes a per-request principal possible at
  all: the callback sees the request, so it can build a server bound to that
  request's ctx principal and gated read handles. No SDK change needed.

- **AC #6 is not optional — it is the whole safety story.** In `identityHeader`
  mode `requireVerifiedJWT` is never wrapped (`router.go:219`) and the terminal
  resolver returns `User: "unknown"` (`router.go:427`). Combined with the CSRF
  exemption the endpoint needs, that is an unauthenticated remote write surface.
  A declarative-ACL deployment fails closed (`acl.ErrUnstampedPrincipal` rejects
  `unknown`), but a NopACL deployment does not. `validateIdentityFlags`
  (`main.go:211`) is the precedent to copy — a pure function refusing an unsafe
  combination at startup, with the reasoning in its doc comment.

- **Do NOT reuse `newMCPServices`** (`internal/cli/mcp_wiring.go:42`): it
  hardcodes `acl.NopACL{}`. Its justification ("the filesystem is the trust
  boundary") inverts exactly for a remote caller, who has no filesystem access
  and for whom the ACL is the ONLY boundary.

- **RR-PQ5UN1 is real and has a cheap fix.** `acl.Request.Globals`
  (`request.go:86`) is an unsynchronised read-modify-write of
  `globals`/`globalsLoaded`, and `attachACLRequest` attaches ONE per HTTP
  request while an MCP POST is many logical operations. `ForPrincipal` does no
  graph traffic (`request.go:55`), so a per-tool-call Request is the fix rather
  than a mutex.

## References

- TKT-UIR41P — the migration + gating half (shipped)
- TKT-G3PPD — MCP tool-list intersection (sibling; the allowlist is its seam)
- DEC-RG878 — transport-layer intersection, default-deny on writes
- `docs/acl-security.md` — the MCP read-gap entry to update once remote lands
