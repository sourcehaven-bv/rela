---
id: TKT-QIQO1W
type: ticket
title: MCP tools for comments
kind: enhancement
priority: medium
effort: m
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

## Problem

Comments can only be read and written through the data-entry HTTP API
(`/api/v1/_comments/{type}/{id}`). An agent working through MCP (for example
against Atlas) that wants to leave review remarks on entities has to find a
token and make raw HTTP calls outside its MCP session.

## Goal

MCP tools for the commentary layer, with exactly the gating of the HTTP routes:
`list_comments`, `add_comment`, `update_comment`, `delete_comment`.

Scope extended on 2026-10-11 (approved): `accept_comment_suggestion`, with the
same permission and face checks as `POST .../{commentID}/accept`.

Stdio `rela mcp` registers only `list_comments`; the four write tools exist only
on the remote server (decided on 2026-10-11).

The MCP docs are generated from `docs-project/` (GUIDE-mcp-server,
GUIDE-comments); edit those, not `docs/`.
