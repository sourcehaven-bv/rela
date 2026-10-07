---
id: TKT-Q6AKUN
type: ticket
title: Index markdown id mentions as backlinks
kind: enhancement
priority: medium
effort: xl
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

## Description

rela renders an inline code span whose whole content is an entity id (for
example `TKT-123`) as a link to that entity. Only the forward direction exists:
the mention is resolved when A is rendered. Nothing records it, so B cannot show
which entities mention it.

Index these mentions so they behave like relations for reading: given A mentions
B, B can list the entities whose body mentions it.

## Open questions (resolve in planning)

- Storage: a derived index (like search) or real relations of a built-in type.
- Which surfaces show backlinks: data-entry entity page, API, MCP, CLI, tracer.
- Whether relation bodies count as a mention source.
- ACL: a backlink reveals that a hidden entity mentions B; it must be row-gated.
