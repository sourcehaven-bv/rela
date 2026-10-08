---
id: BUG-1BXQDD
type: bug
title: Rename and delete errors reveal hidden entities and their relations
description: 'Renaming to an id held by a hidden entity answers entity already exists. A non-cascade delete is refused with entity has relations when only hidden neighbours remain. Rename reports RelationsUpdated including edges to hidden entities. The texts reach MCP and Lua and CalDAV and the CLI. Found in the security review of #1753 (Atlas TASK-RICN8).'
priority: medium
effort: s
why1: The write path builds its errors and counts from the raw store. authorizeCascadeRelations names the far endpoint of a denied edge even when the caller cannot read it. RenameResult.RelationsUpdated counts every incident edge. Callers pass err.Error() through to MCP and Lua and the CLI.
why2: entitymanager reads raw on purpose so writes see every row. Messages written for operators were reused on caller-facing surfaces without passing the read gate.
why3: The read-side rule that a hidden entity is nonexistent is enforced by visibility wrappers on read paths only. Write-path error text and result counts are not read paths so no wrapper covers them.
why4: Each surface patched its own leak. MCP counts visible relations itself. The data-entry 403 drops the wrapped message. No rule says what a write error may reveal so Lua and CalDAV and the CLI were never checked.
why5: There is no stated policy for write outcomes that touch hidden data. Some leak is unavoidable (a collision must refuse) and without a written boundary every leak looked either acceptable or unfixable.
prevention: 'docs/acl-security.md now states the write-outcome policy: a write colliding with hidden data reveals only that it collides. Write-path code that reports on incident relations judges them with edgeVisibility, which restates the read path rule (RR-2IK76Z), so errors and counts follow the same visibility as reads. Rename is limited to id_type manual. Regression tests in hidden_write_errors_test.go pin each case.'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---
