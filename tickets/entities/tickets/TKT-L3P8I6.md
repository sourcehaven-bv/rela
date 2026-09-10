---
id: TKT-L3P8I6
type: ticket
title: bare_face_changed can strand rows at an occupied coordinate, and no step enforces a check
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Repointing `bare_face:` between two faces that BOTH already exist is a different
problem from adopting the first one, and it is not covered by `confirm_face`
(FEAT-H2GSOJ).

When a type already has named-face rows, changing which face the zero coordinate
means can strand data. With `bare_face: published` → `bare_face: draft` and an
entity holding both a bare row and a named `draft` row:

- `StoredFace(task, "draft")` now resolves to the empty coordinate, so the bare
row impersonates `draft`;
- the pre-existing named `draft` row becomes unreachable through the
declared-face API, an orphan awaiting GC;
- `published` resolves to the coordinate `"published"`, which holds nothing.

`renameFaceStep.Run` detects exactly this collision and refuses it, naming the
entity. Nothing enforces the equivalent for a `bare_face` repoint.

## Current state

`confirm_face` is deliberately NOT required for `bare_face_changed`
(`resolvingSteps` in `internal/datamigration/file.go` lists it with an empty
value and a reason). Enforcing it would be wrong twice over: the step does not
look for occupied destinations, and it would break the legitimate
`rename_face`-based migrations that handle this case today.

So the delta is detected, `migrate gen` drafts a `confirm_face` skeleton for it
via the shared drafting case, and an operator who fills that in gets a
confirmation that does not check the thing that matters.

## What it probably needs

Either:

- a check in `confirm_face` that refuses when the newly-bare face already has
named rows, pointing at `rename_face`; or
- a distinct step for the repoint that carries `rename_face`'s collision
detection; or
- narrowing the generator so `bare_face_changed` drafts `rename_face` guidance
rather than a `confirm_face` skeleton.

The third is the cheapest and probably right: the drafting case is currently
shared with `bare_face_introduced` only because both mention `bare_face`, which
is not a good enough reason.

## Scope note

Found during the code review of BUG-TMGWIN rather than in the field. No known
occurrence; a type would need faces AND a repoint AND pre-existing named rows.
