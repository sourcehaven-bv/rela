---
id: DOCS-14QHDU
type: docs-checklist
title: 'Docs: Lua write bindings can name a face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols have doc comments
- [x] Non-obvious decisions explain WHY, not WHAT
- [x] `commentlint` gate clean

`parseWriteOpts` records why a present-but-non-table argument raises (the
silent-drop shape of BUG-HC6I2T), and why key *presence* rather than emptiness
decides — a deliberate divergence from the HTTP path, which treats `""` as
absent.

`requireRelationFaceFor` records the asymmetry with its entity-side twin: it
rejects a wrong face and never demands one, because a relation with a zero tail
writes a real addressable edge whereas a faced entity create has no row at all.
It also names the callers that rely on that (RR-HQUW7V) and the cascade-host
path it does not cover.

`EntityToTable`'s `face` records why it carries no redaction entry: the
row-level gate already decided whether the script sees the row, so the
coordinate it was read at discloses nothing further.

`internal/datamigration/luastep.go` records why its mirror deliberately does
**not** gain `face` — a migration step cannot choose which row it rewrites.

## Project Documentation

- [x] User-facing guide updated
- [x] Generated docs regenerated and committed

**`docs/` is a build output** (RR-NP9T1J). Edited the sources and ran `just
docs`:

- `docs-project/entities/guides/GUIDE-lua-scripting.md` — the three signature
rows, a new "Writing to a content state (face)" section with the options table
per binding, and an "Addressing a face on the other write bindings" table
covering the asymmetry between `update_entity` (selects the face),
`delete_entity` (deletes the whole family) and `delete_relation` (marked as a
**known defect, BUG-YVU8CP**, so nobody builds on it).
- `docs-project/entities/guides/GUIDE-content-states.md` — a script names a
face directly, and why not through a world.

Both state that an entity create **requires** a face on a faced type while a
relation create does not, which is the distinction RR-HQUW7V turned on.

`just docs-check` passes, so the CI gate the design review predicted would fail
is confirmed green.

## External Documentation

- [x] ~~Release notes~~ (N/A: carried by the PR description; the one
behaviour change — `create_relation`'s 4th argument must now be a table — is
recorded in AC 8 with its justification)
- [x] ~~API reference~~ (N/A: no HTTP surface changed)
- [x] ~~Migration guide~~ (N/A: no schema or data shape changed; faceless
projects see identical behaviour)
