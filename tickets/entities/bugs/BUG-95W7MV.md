---
id: BUG-95W7MV
type: bug
title: Tracer and analyze skip faced types
description: trace, find_path and analyze duplicates/unique/gaps/orphans query default rows only, so faced types are silently absent.
priority: medium
effort: m
why1: The tracer and the analysis checks read each node with GetEntity(id) or scan with a zero EntityQuery, both of which return only the face-less row. A faced type has no such row, so it is skipped.
why2: 'Those call sites predate faces: they were written when an entity was one row, and the store API still lets a caller read without naming a face.'
why3: Stage 1 of DEC-NPZICR moved the request-bound readers onto addresses but left whole-graph readers (tracer, analyze) on the zero-face API because they have no request world.
why4: 'Nothing forced those readers to choose a coverage: the zero value of EntityQuery silently means the default world, and the zero-face guard only counts reads, it does not fail them.'
why5: The store API makes the face optional, so a missing face is not a compile error but a silent narrowing of every whole-graph report.
prevention: Analysis and trace state their coverage per check (family, every face, per face), the zero-face allowlist shrinks for these files, and Stage 2 PR 6 and PR 8 make the face mandatory on reads and queries.
status: done
---

## Problem

Reported by the face-awareness inventory, not yet verified. The tracer (`trace`,
`find_path`; `tracer.go:100,156,200`) and `analyze` duplicates, unique, gaps and
orphans (`analysis.go:149-304`) skip faced types or mishandle them, because they
query default rows only.

## Expected

Each analysis states which faces it covers (all faces, or a world) and includes
faced types. A faced type is never silently absent from a report.
