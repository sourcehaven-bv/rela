---
id: DEC-QNZSC6
type: decision
title: Desktop ACL confines automation and MCP, not the user
context: The desktop wires NopACL because the user owns the machine. Scheduled scripts with AI and MCP clients now write the graph for the user, and nothing limits them.
consequences: Each entry point becomes a principal with user-granted roles and capabilities stored on the machine. Lua deps are built per principal. MCP runs inside the app behind a bridge. Same-user malware is out of scope.
date: "2026-10-06"
status: proposed
---

## Decision

On the desktop, the ACL limits what acts for the user, not the user:

- The window user is always `owner`.
- Every other entry point (scheduler, MCP client, later deep links and
intents) is its own principal, with no rights until the user grants them.
- The document's `acl.yaml` defines roles. The desktop app's user assigns them
to principals, and grants side-effect capabilities (AI, mail, HTTP, commands,
secrets) separately. Grants live in app settings, keyed by document ID and
place, never in the document.
- MCP is one server hosted in the app, reached through a stdio bridge over a
local socket. Every tool takes a `document` argument.

## Why

Automation that writes the graph (AI in scheduled scripts, MCP agents) can do
damage by mistake or through prompt injection. Limiting it needs a principal per
entry point and a grant the document cannot set itself. Capabilities are
enforced as per-principal Lua bundles, because the ACL covers only entities and
relations.

## Details

docs/architecture/desktop-acl.md
