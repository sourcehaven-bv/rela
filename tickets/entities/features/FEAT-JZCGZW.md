---
id: FEAT-JZCGZW
type: feature
title: 'copy_face migration step: give an entity a second row on another face'
summary: A migration step that creates an additional named-face row alongside an entity's existing one. Distinct from migrate_face, which moves a row to exactly one face; this is for seeding a second content state (a draft copy of published content) during a migration.
description: Follow-up to FEAT-H2GSOJ. migrate_face moves each row to exactly one face, which is the right answer for adopting faces on existing data. copy_face is the different operation of giving an entity content at TWO faces at once. Its original rationale (that a row could not be moved at all, so per-row assignment had to be a copy) was removed by BUG-HC6I2T; what remains is the narrower and genuinely useful case of seeding a draft alongside published content. The open questions are what content the copy carries and whether a declarative step is the right shape at all, given the lua step already transforms properties.
priority: medium
status: proposed
---

## What changed

This was originally proposed because a row could not be moved off the zero
coordinate at all, so "give different entities different faces" had to be
expressed as a copy. **BUG-HC6I2T removed that constraint**, and FEAT-H2GSOJ now
ships `migrate_face`, which moves rows properly. The original motivation is
gone.

What remains is narrower: giving an entity content at **two** faces at once —
seeding a `draft` copy alongside `published` content, so editors have somewhere
to work without touching what readers see.

## Sketch

```yaml
- copy_face:
    entity: article
    from: published
    to: draft
```

Every `article` with a `published` row gains a `draft` row with the same
content. The source is untouched.

## Open questions

- **Is a declarative step the right shape?** A copy is only useful if the copy
differs from the source in some way, and the moment it does, the operator wants
a transform — which the `lua` step already provides, and which a fixed-shape
step cannot express. A verbatim copy may be too narrow to earn a step of its
own.
- **What happens when the destination is occupied?** `rename_face` refuses a
collision unless the content is identical (its crash-convergence contract). A
copy should reuse that rather than re-derive it.
- **Is this migration work at all?** Seeding a draft state looks more like
something an automation or a one-off script does than a schema migration. A
migration exists to make stored data fit a new schema; content that was never
there is not a conformance problem.

That last question is the one to settle first. If the answer is no, close this
rather than build it.

## Priority

Lowered: no known caller. The case that motivated the original filing is handled
by `migrate_face`.
