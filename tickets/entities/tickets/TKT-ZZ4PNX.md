---
id: TKT-ZZ4PNX
type: ticket
title: 'Inbox 1: inbox read with waiting entries'
kind: enhancement
priority: high
effort: m
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

## Description

Slice 1 of the per-principal inbox (RES-B0MJAZ): one place where a person or an
agent sees every session that waits on them, across channels.

- `Service.Inbox` lists derived "waiting on you" entries: sessions whose current state has `waiting_for` roles the caller holds (4/6 `waiting_on_me`). It finds them with one ACL-gated graph query for entities whose role property names the caller, then fetches their channels in one batch. Creator roles need a `created_by_user` column.
- `GET /api/v1/_inbox` and MCP `list_inbox`, usable with `watch`.
- Gate before any cap, so `truncated` is not an existence oracle; never cache across principals; budget test on the list path.
- Docs in `docs/realtime.md` and the MCP guide.

Out of scope (own tickets): read markers and unread counts (TKT-2B2AIA),
mentions and replies (TKT-6RXAC0), SPA inbox and live (TKT-9YRRM5), mail and
desktop notifications (TKT-Z3PS9U, later), cross-process live (TKT-YK30D1). No
`awaits:` key.

Requested by Jeroen via another session for the agent-runner demo.
