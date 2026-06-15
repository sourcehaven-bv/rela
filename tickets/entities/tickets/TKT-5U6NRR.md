---
id: TKT-5U6NRR
type: ticket
title: Expose request principal to Lua runtime (rela.principal) for write-path authorship
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

Lua write-path scripts (automations triggered on entity create/update) can call
`rela.create_relation(...)` and the writes are correctly **attributed** to the
caller's principal in the audit log — the Principal flows into the runtime via
`parentCtx` (`internal/lua/runtime.go` `WithContext`/`callerCtx`, and
`internal/script/executor.go`). But the script **cannot read who the current
user is**: the `rela` table exposes `create_relation`, `create_entity`, `args`,
`params`, `today`, etc. (runtime.go ~645-730) — there is **no `rela.principal` /
`rela.user`** field.

This blocks the canonical "stamp authorship server-side" pattern. Concretely,
the wanted flow is: on ticket create, an automation runs a Lua script that links
the **submitter** to the ticket via a `created-by` relation, so the submitter
(and only they) can later read their own ticket via ACL — without the client
ever being able to forge the edge (a client-writable `created-by` would be a
self-grant escalation). The script needs `me = <current user>` to use as the
edge's `from`. Today the only user-ish tokens are
`{{user.name}}`/`{{user.email}}` which come from **git config**
(`internal/automation/template.go`), i.e. the server process owner — wrong for a
multi-user web app where the submitter is the `X-Rela-User` principal.

## Scope

- Expose the request principal to Lua as a read-only table, e.g.
`rela.principal = { user = "...", tool = "..." }`, populated from
`principal.From(r.parentCtx)` (the Principal struct is `{User, Tool}` —
`internal/principal/principal.go`). Set it where the other late-bound `rela.*`
fields are populated; fall back to the `unknown/unknown` zero value
`principal.From` already returns when unstamped.
- (Optional, same wiring) add a `{{principal.user}}` automation interpolation
token in `internal/automation/template.go` so declarative `create_relation`
actions can also reference the actor — pick whichever the demo needs; the Lua
field is the primary ask.

## Out of scope

- The ACL policy / `created-by`-confers-read modeling (that's project metamodel,
already supported by the resolver — see the read-side ACL work TKT-VQGN/VMD8).
- Read-path principal exposure: not needed for authorship stamping and the read
path must stay free of user-supplied Lua (CLAUDE.md).

## Acceptance

- A write-path Lua automation can read `rela.principal.user` and it equals the
`X-Rela-User`-derived principal (not the git user); verified by a test that
stamps a principal on ctx, runs a script that returns `rela.principal.user`, and
asserts the value.
- Unstamped/CLI context yields the documented `unknown` fallback, not an error.
- A `created-by` edge created by such a script is attributed to that principal
in the audit log (already true; pin it with a test).
