---
id: TKT-TX53E
type: ticket
title: Migrate fsstore read paths to RootedFS (pre-empt CodeQL query-set expansion)
kind: enhancement
priority: low
effort: m
status: wont-fix
---

> **Closed by backlog sweep (2026-07-20):** already implemented — all fsstore read paths route through RootedFS (markdown.go readDataFile → s.rooted.ReadFile; entity.go/relation.go/index.go/attachment.go use s.rooted); only os.ErrNotExist sentinel checks remain.

