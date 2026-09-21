---
id: RR-QW763Z
type: review-response
title: Read-side face on the Lua tables needs its no-redaction reasoning written down
finding: 'EntityToTable (internal/lua/runtime.go:1328) is shared by the gated rela.get_entity and the elevated admin.get_entity (elevation.go:292), and carries a `redacted` set for field-level ACL. Face is not a property, so visibility.Redact will not touch it — adding `face` publishes the entity''s content state unconditionally on whatever rows the row-gate let through. This is very likely not a leak: the row-gate decides whether the script sees the row at all, and if it sees a published row then ''this row is published'' is the coordinate it was fetched at, not a further secret. It is closer to `type`, which is already emitted. But the plan states no reasoning, and a future reviewer finding a new field on a read path with no redaction hook should find the argument written down rather than have to reconstruct it. No attack path was constructible; this is documentation debt on a security-relevant field.'
severity: minor
resolution: 'Accepted. The plan''s Security section now records the reasoning explicitly: Face is not a property so visibility.Redact does not cover it, deliberately; the row-level gate decides whether the script sees the row at all, and if it does, the coordinate it was fetched at discloses nothing beyond what the successful read already did. Structurally the same as `type`, which EntityToTable already emits unconditionally. The plan requires this reasoning be carried into the code comment at the emission site, so the next reviewer finds the argument rather than re-deriving it.'
status: addressed
---

## Finding

`EntityToTable` (`internal/lua/runtime.go:1328`) is shared by the ACL-gated
`rela.get_entity` and the elevated `admin.get_entity`
(`internal/lua/elevation.go:292`), and it already carries a `redacted` set for
field-level ACL.

`Face` is **not a property**, so `visibility.Redact` will not touch it. Adding
`face` therefore publishes the entity's content state unconditionally on every
row the row-gate allowed through, with no redaction hook and no per-principal
variation.

## Assessment: not a leak, but unargued

I could not construct an attack path, and I do not believe one exists. The
row-level gate decides whether the script sees the row at all; if it sees a
published row, "this row is published" is the coordinate the row was fetched at,
not an additional secret. It is structurally closer to `type`, which
`EntityToTable` already emits unconditionally.

This also is explicitly **not** a field-name-existence concern — `CLAUDE.md`
settles those, and face names are operator-authored config, which is why naming
them in error messages (as the plan proposes) is correct.

## Why raise it anyway

The plan states no reasoning at all for the read-side addition beyond
ergonomics. A future reviewer encountering a new field on a read path that has a
redaction mechanism and does not use it should find the argument written down
rather than have to re-derive it — and re-deriving it under time pressure is how
a genuine one gets waved through by analogy later.

This is documentation debt on a security-relevant field, not a defect.

## Fix

One sentence in the plan's Security section, carried into the code comment: the
face is the coordinate the row was read at, gated by the row-level decision, and
discloses nothing beyond what the successful read already did — so it needs no
redaction entry.
