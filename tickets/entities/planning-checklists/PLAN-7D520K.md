---
id: PLAN-7D520K
type: planning-checklist
title: 'Planning: Add web URLs to MCP entity results'
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In scope: a `url` field on every entity an MCP tool returns (show,
create, update, list, search, mentioned_by, relation targets, trace and path
nodes, pile items), the server instructions, and the MCP guide. Out of scope: a
`/s/<space>` prefix, a configured public URL setting, and URLs on analyze
findings (they list ids, not entities).

**Acceptance Criteria:**
1. Over HTTP, `show_entity` returns `url` = `<origin>/entity/<type>/<id>` for the entity and its relation targets.
2. A faced row links as `<id>@<face>`.
3. Without a request origin (stdio), `url` is omitted.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small enhancement)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo precedent)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:** The calendar feed (`feedBaseURL` in
`internal/dataentry/feed_handler.go`) already derives deep links from the
request origin. `mail.yaml` `base_url` is the only configured public URL; it is
optional and mail-specific, so MCP does not read it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `Server.HTTPHandler` puts the request origin on the ctx.
One helper, `entityURL(ctx, type, ref)`, builds the link; every DTO builder in
`internal/mcp` calls it. No base on ctx means no `url`.

Alternatives rejected: a space prefix (a type can be created from several
spaces, and the space config is not loaded by MCP); reading `mail.yaml`
`base_url` (couples MCP to mail config and is unset on most deployments).

**Files to modify:** `internal/mcp/links.go` (new), `convert.go`, `server.go`,
`tools_piles.go`, call sites, the MCP guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The `Host` and `X-Forwarded-Proto` headers. A
host with characters outside `[a-zA-Z0-9.:[]-]` yields no base. The URL is built
only for entities the caller already reads, so it discloses nothing new.

**Security-Sensitive Operations:** None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Unit tests for `entityURL` and `requestBaseURL`; DTO tests
with a base on ctx; an HTTP test calling `show_entity` through `HTTPHandler`
with a proxied Host.

**Edge Cases:** Faced rows, path escaping, TLS and forwarded scheme.

**Negative Tests:** No base, empty or unsafe Host, missing type or id.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A proxy that rewrites Host would produce internal links. The
production proxy forwards the public Host (see `Server.HTTPHandler`). Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: one paragraph in GUIDE-mcp-server)

**Documentation Impact:** GUIDE-mcp-server (and the generated
`docs/mcp-server.md`) gains a paragraph on `url`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small additive field)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** None.
