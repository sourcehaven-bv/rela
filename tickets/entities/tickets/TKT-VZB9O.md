---
id: TKT-VZB9O
type: ticket
title: Trim and split root CLAUDE.md into nested instruction files
kind: enhancement
status: wont-fix
---

> **Closed by backlog sweep (2026-07-20):** substantively done — nested CLAUDE.mds exist (frontend/, internal/dataentry/, internal/entitymanager/), the consumer-side-interfaces essay moved to docs/architecture/, the @managed block was trimmed, and root CLAUDE.md dropped ~974 → 608 lines. The exact ~440-line target was aspirational; further trimming can ride future doc changes.

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
