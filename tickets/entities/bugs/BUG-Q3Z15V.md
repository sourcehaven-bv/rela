---
id: BUG-Q3Z15V
type: bug
title: Conflict list and detail endpoints skip the read gate
description: GET /api/v1/_conflicts and GET /api/v1/_conflicts/{path} return entity ids, property values and content of conflicted files with no row or field gating. Found during the security review of BUG-Y34ZSZ.
priority: high
effort: s
why1: The conflict list and detail handlers served every conflicted file after checking only the sync permission and path containment.
why2: A conflicted file cannot be parsed by the store, so the handlers read it directly and the resolver's row read was never on their path.
why3: The read gate is applied by visibility wrappers around store reads, and nothing required a handler that bypasses the store to ask the gate itself.
why4: BUG-T6HNKS accepted the coarse sync-permission gate as sufficient because conflicts only arise on local file storage.
why5: No route-level test asserts that every served read of entity data answers the uniform 404 for a hidden row.
prevention: 'The conflict routes gate the identities a file names through the same row gate and redact visible: fields; regression tests pin the list, the detail and the resolve. AM-served-writes-resolve-through-read-gate tracks a route-enumerated check.'
started: "2026-11-04"
completed: "2026-11-05"
status: done
---

## Summary

`handleV1Conflicts` lists every conflicted file with its entity type and id.
`handleV1ConflictRoutes` (GET) returns the ours/theirs value of every property
and both content sides. Neither consults the read gate or `visible:` redaction;
the only check is that the path stays inside the project root.

A principal who cannot read an entity can therefore learn that it exists and
read its values while the file has git conflict markers. Only the fs backend
with git sync produces such files.

The resolve POST is gated (`authorizeConflictResolve`).

## Fix direction

Gate each conflicted file on the entity (or relation endpoints) through the
visibility reader: omit rows the principal cannot read from the list, answer the
uniform 404 on the detail, and redact hidden fields on both sides.

Found by the rela-security-reviewer on BUG-Y34ZSZ (GitHub #1774).
