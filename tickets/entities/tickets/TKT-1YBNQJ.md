---
id: TKT-1YBNQJ
type: ticket
title: bare_face_removed still drafts an empty migration, the same gap confirm_face closed for adoption
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

`bare_face_removed` is classified `TierMigration` with the message "rows at the
zero coordinate belong to no declared face", but no step can answer it and `rela
migrate gen` drafts `steps: []`. This is the residue of BUG-TMGWIN:
`confirm_face` closed the *adoption* direction (`bare_face_introduced`, and
`bare_face_changed` which reuses the same drafting case), and this is the
remaining sibling in the same switch (`shapecompare.go:212-226`).

Verified against a scratch project: removing `faces:`/`bare_face:` from a type
that has rows generates a file whose only content is `steps: []`, which then
applies and reports the schema in sync.

## Why it is lower severity than the adoption case

Going faced → flat is the safe direction. The bare rows keep their content and
become the type's only rows again; what is lost is the *meaning* attached to the
bare coordinate, not the data. The adoption case silently changed what every row
claimed to be, which is why it was fixed first.

The named-face rows are the real question here. They do not disappear when the
schema stops declaring faces — they become undeclared stored faces, which is the
drift the GC eventually sweeps.

## What it probably needs

Not a new step kind, most likely. The honest answer is either:

- a `confirm_face` variant that acknowledges the removal (the rows' state is now
nothing), or
- generator guidance pointing at `drop_entities` for the named-face rows, if the
operator means to discard them.

Worth deciding rather than leaving the delta unanswerable, since an unanswerable
`TierMigration` delta is exactly what
AM-migration-delta-kinds-have-resolving-steps is meant to make impossible. That
guard, once written, will fail on this.
