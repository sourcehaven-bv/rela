---
id: TKT-VZB9O
type: ticket
title: Trim and split root CLAUDE.md into nested instruction files
kind: enhancement
status: backlog
---

## Description

The root `CLAUDE.md` had grown to ~974 lines / ~41 KB, loaded into every
session's context. Reduce it by relocating content to where it is loaded
on-demand:

- Subsystem rules → nested `CLAUDE.md` (`internal/entitymanager`,
`internal/dataentry`) that auto-load only when editing that area.
- The consumer-side-interface design essay → `docs/architecture/`, with
worked examples kept in godoc on the real types.
- Trim the `@managed: claude-workflow` block at its plugin source (drop the
generic Python test-writing section; replace the `metamodel.yaml`- duplicating
automation reference with a pointer).

Root `CLAUDE.md` drops to ~440 lines with breadcrumbs to the relocated content.
Also syncs pending `claude-workflow` plugin content (the `/research` workflow
and refreshed `tickets/` templates).
