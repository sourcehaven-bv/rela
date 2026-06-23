---
id: TKT-8FSBGB
type: ticket
title: 'Sync 1/5: shared canonical entity/relation serializer + content hash'
kind: enhancement
priority: medium
effort: m
status: done
---

Sub-ticket of TKT-WE01O5 / FEAT-NJ9FEN. **Foundation — everything else depends
on this.**

## Problem

There is no shared serializer producing identical bytes for the same logical
entity/relation across fsstore and pgstore. fsstore reflows the body (goldmark +
80-col wrap) and orders keys by `schema.PropertyOrder`
(`fsstore/markdown.go:384,399`); pgstore stores raw structured columns and never
renders markdown (`pgstore/entity.go:203,506`). Hashing storage bytes on each
side would never match.

## Scope

- NEW `internal/canonical/` package: `Hash(entity.Entity) string` and
`Hash(entity.Relation) string` (sha256 hex) over a deterministic byte form.
- Schema-independent key ordering; normalized value types — watch the JSONB↔YAML
number/slice coercion (`pgstore/entity.go:538 normalizeJSONNumbers`).
- ONE body-canonicalization rule applied identically on both sides (decide:
canonicalize/reflow in the hash fn so fsstore's reflow vs pgstore's raw content
converge).
- Both backends compute the hash by feeding a reconstructed `entity.Entity`/
`entity.Relation` through this fn — NOT by hashing stored bytes.

## Acceptance

- **Linchpin test**: the same logical entity created via fsstore and via pgstore
yields an identical hash. Table-driven over property types: ints, floats,
whole-floats, lists, nested, multiline body, unicode, empty.
- Relations hashed by their logical content too.
- `fsstore/echo.go:46 hashContent` (on-disk-bytes, fsnotify echo dedup) left
untouched — not reusable for this.

## Notes

This is the highest risk in the whole feature (canonical-hash divergence → every
push 412s). Build the cross-backend equivalence test first. No new external
libs.
