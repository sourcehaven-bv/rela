---
id: TKT-7R0ABK
type: ticket
title: Version snapshots do not record which face they captured
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

`store.VersionMeta` carries `Version`, `Op`, `PrevID`, `Type`, hashes,
attribution and `Origin`, but no face. `store.VersionInput` (the synchronous
capture shape) DOES carry one, with the note that it "participates in the
content hash, so two faces holding identical bytes do not dedup against one
another" — so the face reaches storage and is then not readable back on a
snapshot.

This became visible in BUG-HC6I2T. Restoring a DELETED entity (`restoreRecreate`
in `internal/dataentry/history_restore.go`) builds a fresh entity from the
snapshot and creates it. For a type declaring faces there is now no coordinate
to create it at: the route addresses a bare id, and the snapshot does not say
which face it captured. The manager refuses with `ErrFaceRequired`, which is the
fail-closed answer, but it means a deleted face cannot be resurrected at all.

Restoring onto a LIVE entity is unaffected: `restoreOntoLive` addresses a row
that already exists and keeps its face.

## What to build

Add `Face entity.Face` to `store.VersionMeta` so a snapshot reports its
coordinate, and thread it through the readers (`HistoryReader`, the pg queries,
the sweep's projection). The value is already stored — per-state versions landed
in migration `0012_per_state_versions.sql`, which put `face` in the
`entity_versions` primary key — so this is a read-path gap rather than a capture
gap. Confirm that before designing anything.

Then `restoreRecreate` passes `snap.Face` into `CreateOptions`, and the history
route can offer a per-face restore.

## Watch for

The lineage walk is fenced by `[lo,hi)` vseq ranges precisely so a
rename/id-reuse does not merge two entities' histories. Adding a face to the
read shape must not widen that: a face is part of a row's identity, so history
for `POL-1@concept` must not fold in `POL-1@vastgesteld`.
