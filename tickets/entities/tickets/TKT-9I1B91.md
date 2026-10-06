---
id: TKT-9I1B91
type: ticket
title: 'Desktop ACL: confine the scheduler and MCP clients'
kind: enhancement
priority: medium
effort: xl
status: backlog
---

## Description

Implement the desktop access-control direction in
docs/architecture/desktop-acl.md (DEC-QNZSC6): confine the scheduler and MCP
clients, which act for the user, while the window stays `owner`.

This is the umbrella ticket; split it along the order of work before planning:

1. A principal per entry point (scheduler, notifications), recorded in
version history.
2. Capability bundles per principal: `ai`, `mail`, `http`, `commands`,
named secrets, as per-principal Lua `ReadDeps` / `WriteDeps`.
3. The declarative ACL on the desktop: window `owner` fast path, built-in
roles (`owner`, `reader`, `navigate`), assignments in app settings keyed by
document ID and place.
4. MCP in the app: stdio bridge over a local socket, native pairing dialog,
`list_documents`, a `document` argument on every tool, open documents only.
5. Undo by principal and an activity indicator.

## Acceptance criteria

- A scheduled script without the `http` capability has no `http` binding.
- A scheduler without a role cannot write; with a role, it cannot write
beyond it, including through cascades.
- A copied document does not inherit the original's grants.
- An MCP client sees only documents granted to it, and every write it makes
is recorded with its principal.
