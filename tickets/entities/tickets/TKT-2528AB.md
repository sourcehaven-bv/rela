---
id: TKT-2528AB
type: ticket
title: entity.Ref and one gated resolver in internal/visibility
kind: enhancement
priority: high
effort: l
status: ready
description: 'Stage 1 of RES-Y6JA37: a typed face address and a single resolver (explicit/world/family) that always applies row and face gates; dataentry, mcp and lua migrate onto it.'
---

## Description

Stage 1 of RES-Y6JA37 / DEC-NPZICR.

- Add `entity.Ref{ID, Face}`: the typed address of one face row. Parse and format it at the boundaries (`ParseStateRef`/`FormatStateRef`). A faceless entity's `Ref` has the implicit face `""`; the wire form stays `ID`.
- Add one resolver in `internal/visibility` with three modes: an explicit `Ref`, a world, or the whole family. It returns a row that has passed the row gate (bare id) and the face gate, redacted once, with the uniform not-found for denied, missing and type-mismatched.
- Migrate `internal/dataentry` (replacing `entityRef`, `getVisibleRef`, `faceReadable` call pairs and `bareEntityID`), `internal/mcp` and `internal/lua` reads onto it. Shrink the Stage 0 guard allowlist accordingly.

Bugs fixed on top of this: BUG-CTUW2N, BUG-BZQQDP, BUG-4SYAA6, BUG-FYEEVX,
BUG-8J3LSB (if not already fixed in Stage 0).

## Acceptance

- No dataentry, mcp or lua read-out path calls the store directly for a single entity; write-prep reads stay raw, per CLAUDE.md. The guard allowlist keeps only those write-prep entries in those packages.
- A handler cannot obtain a row without the face gate having run.
