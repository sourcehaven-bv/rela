---
id: TKT-1YBNQJ
type: ticket
title: 'faces_removed drafts an empty migration: no step moves rows back when a type loses its faces'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

`faces_removed` is classified `TierMigration` with the message "rows at <faces>
belong to no declared face, and the type's single state is a coordinate none of
them occupies", but no step can answer it and `rela migrate gen` drafts `steps:
[]`.

This is the mirror of the gap `migrate_face` closed for adoption (BUG-TMGWIN /
FEAT-H2GSOJ). It is listed in `resolvingSteps` with an empty value and a written
reason, so the exemption is visible rather than silent; the guard test will fail
if anyone removes the entry without adding a resolving step.

## Why it is harder than the adoption direction

Adoption is a fan-out with one right answer per row: each row sits at the zero
coordinate and moves to the face its data names. Removal is a **fan-in**. An
entity may hold content at several faces, and dropping `faces:` leaves all of
them addressing nothing while the type's single state is the zero coordinate
none of them occupies.

So a step has to decide which face's content becomes the surviving row, and what
happens to the others. That is a merge, not a move, and the answer is
per-project: the newest? a named one? refuse when they differ?

## What it probably needs

A `merge_faces`-shaped step naming the winning face explicitly, e.g. `{entity:
article, keep: published}`, refusing when a row has content at a face that is
not the winner unless the operator says to discard it. Worth designing alongside
FEAT-JZCGZW, which has the same shape in reverse.

## Note

Originally filed against `bare_face_removed`. BUG-HC6I2T renamed the delta to
`faces_removed` and changed what it means: the zero coordinate is no longer a
face, so there is no longer a privileged row that survives by default. That made
this case harder, not easier — retitled and rewritten accordingly.
