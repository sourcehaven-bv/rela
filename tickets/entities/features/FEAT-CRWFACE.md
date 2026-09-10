---
id: FEAT-CRWFACE
type: feature
title: worlds.<name>.create — the face a create issued from a world lands in
summary: A faced type has no default row, so a create must name a face. The create form cannot, so the create button is broken for every faced type. A world names the face creates from it land in.
description: 'BUG-HC6I2T made a create name its face, correctly: a faced type has no zero-coordinate row to default to. But no client can supply one. The create form is generic and reachable from several places, so which state a new entity starts in belongs to the workflow that opened it, not the field layout — and CreateEntity carries no face. Result: POST from the UI answers face_required for every faced type, and the dry-run that gates the form fails the same way, so the form renders no property fields at all. worlds.<name>.create names the face; the client sends the world it created from and the server resolves it.'
priority: high
effort: s
status: in-progress
---

## Problem

Two symptoms, one cause. On a type declaring faces:

- **The create button fails.** `POST /api/v1/beleids` answers 422
  `face_required`. The form sends no face because `CreateEntity` has no field
  for one.
- **The form renders no property fields.** It dry-runs on mount to get field
  affordances; `ValidateCreate` enforces the same face rule
  (`manager.go:876`), so the dry-run 422s and the form falls back to
  relations and content only.

An unfaced type is unaffected, which is what made this look like a
metamodel problem rather than a missing client capability.

## Why the client cannot name the face

The same form is reachable from several places, and which state a new entity
starts in is a property of the workflow that opened it. `create_world`
already carries exactly that fact — its doc comment describes this ISMS
scenario verbatim — but it only chose which world the form OPENED in. The
write still went to the zero row.

## Decisions

**Not derived from `select[0]`.** An ISMS world heads its ADOPTED face, so
deriving the create face from the chain would publish by the act of
creating — the defect BUG-HC6I2T fixed.

**Rides the body, not `?world=`.** `attachWorld` refuses a world on every
write, because a chain can answer with a FALLBACK and a write must name the
row it changes. `create:` names one declared face directly, so that
objection does not apply.

**A new key rather than `edits:`.** `edits:` is documented as parsed-but-
unused for the Step 4 copy kernel and means a different thing: where an edit
lands, not where a create starts.

**A faceless type ignores the world.** It has one state and no name for it,
so a world-bound list keeps working for the faceless types on it.

## Not covered here

`apiwire/v1/responses.go:352` argues the affordance a client needs is a
gated "which faces may I create" query over the copy definitions (Ruling 9).
That may still be the right long-term shape; this is the minimal fix that
makes the button work.
