---
id: TKT-6KLUT3U
type: ticket
title: Point-in-time (as-of) reads of a world
kind: enhancement
priority: high
effort: xl
started: "2026-11-16"
completed: "2026-11-18"
status: done
---

## Description

GRC gap item 5. An auditor asks what the statement of applicability looked like
on 31 December. A request must be able to read a world as it stood at a given
instant, and every read surface must show that state.

- The request names the instant with `?as_of=` (an RFC 3339 timestamp with an explicit zone), and needs the `history:read` permission. As-of requests are read-only; every write is refused.
- The store answers entity, header, graph-query and relation reads from version history at the instant, in SQL on pgstore and sqlitestore. fsstore and memstore refuse with a clear "not supported" error.
- Gating: today's ACL decides access (decision entity). Row gating goes through `internal/visibility` and reads today's edges, so access lost since fails closed. `visible:` redaction follows the history endpoint's fail-closed rules.
- Entities created after the instant, deleted after it, renamed after it and faces published after it behave correctly.
- Coverage: lists, kanban, calendar, gantt, dashboard cards, entity detail, documents, search. Verify each view follows faces and worlds with a test.
- Docs with an audit example.
