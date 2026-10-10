---
id: DOCS-BR8D2N
type: docs-checklist
title: 'Docs: field write gate on MCP and scheduled Lua'
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: `gated()` and `fieldGated()` explain why cascade and elevation drop the gate; `WriteGate` notes the typed-nil return.
- [x] Function/type docs if public API: `FieldGated`, `appbuild.FieldGatedEntityManager`, `FieldWriteGate`, `affordances.CheckFieldWrite`, `WriteGate`, `FieldWriteError`, with `Nil:` contracts.

## Project Documentation

- [x] ~~README updated~~ (N/A: no README-level feature)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting rule; the rule is documented in docs/acl-security.md)
- [x] ~~Help text accurate~~ (N/A: no CLI changes; the CLI option was removed before merge)

## External Documentation

- [x] Changelog entry added: docs/acl-security.md section "Field write grants on MCP and scheduled scripts" carries the upgrade note for scheduled tasks.
- [x] ~~API docs updated~~ (N/A: the 403 rule ids are unchanged)
