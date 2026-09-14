---
id: DOCS-VYANSN
type: docs-checklist
title: 'Documentation: faces.<name>.messages.notice (TKT-NLWZLX)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on the new exported field
- [x] Comments explain WHY, not just what

`metamodel.FaceMessages.Notice` carries the decision record: why a second key
exists rather than a relaxed guard on `read_only` (who the sentence is about —
reader vs document), that a writable-by-definition face can never satisfy
`read_only`, that the two are independent, and that the page renders `notice`
first. `v1.FaceMessages.Notice` carries the short wire form. In the SPA,
`noticeNote` records both guards it deliberately does NOT have and marks its
`worldAbsent` term as belt-and-braces rather than the mechanism; `bannerNotes`
records the ordering rationale; the `.banner-note` CSS records that jsdom cannot
see it and how it was verified by hand.

## Project Documentation

- [x] `docs/metamodel.md` — table row for `faces.<name>.messages.notice`,
plus prose on which key to pick and a worked YAML example
- [x] `docs/content-states.md` — the faces section now introduces both
message keys with an example declaring one of each, and the web-app behaviour
list says a notice renders regardless of write permission with `notice` first
when both apply
- [x] `docs/data-entry.md` — worlds section, same behaviour note
- [x] ~~`docs/cli-reference.md`~~ (N/A: no command changes)
- [x] ~~`CLAUDE.md`~~ (N/A: no new pattern — this follows TKT-5SZG2L's
established face-chrome shape at every layer)
- [x] ~~`README.md`~~ (N/A: not a project-level change)

The operator-facing question the docs must answer is "which of the two keys do I
want", so every place `read_only` was already described now states the
reader-vs-document distinction rather than just listing a second key.

## External Documentation

- [x] ~~API reference~~ (N/A: `/_schema` is described by
`docs/data-entry.md`, which is updated; there is no checked-in OpenAPI or
JSON-schema document covering `faces`/`messages` — the `internal/openapi`
generator covers entity data only, confirmed during review)
- [x] ~~Migration notes~~ (N/A: purely additive. An existing project
declaring no `notice` serves a byte-identical `_schema`, and `ShapeProjection`
excludes `Messages`, so no data migration is triggered)
