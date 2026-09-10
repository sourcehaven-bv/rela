---
id: FEAT-JZCGZW
type: feature
title: 'copy_face migration step: give selected entities a second row on a named face'
summary: A migration step that creates a named-face row alongside an entity's bare row, for entities selected by a property value. This is the copy operation that per-row face assignment actually requires, since the store keeps every entity on its bare coordinate and a row can never be moved off it.
description: 'Follow-up to FEAT-H2GSOJ. confirm_face states which face the existing bare rows become, which is all a move-based step could ever do given the store''s row-family invariants. Giving different entities different faces needs a second row per entity rather than a reassignment, so it is a copy: create a named-face row alongside the bare row, for entities matching a condition. The open design questions are what content the new row carries (a duplicate of the bare row, or a transform) and what the bare row then means, since it remains the family head and keeps whatever bare_face names.'
priority: medium
status: proposed
---

## Why this is a separate step

`confirm_face` (FEAT-H2GSOJ) states which face the existing bare rows become.
That is the whole of what a move-based step could do, because the store's
row-family invariants (TKT-DOFYR1) keep every entity on its bare coordinate:

- a family's default row cannot be deleted while a sibling face remains, and
- a named-face row cannot exist without a default row ("headless").

So "put these rows on `draft` and those on `published`" is not a reassignment at
all. It is a request for a **second row** on the named face, alongside the bare
row that necessarily stays. That is a copy, and it deserves its own step rather
than being smuggled into a confirmation.

## Sketch

```yaml
- copy_face:
    entity: article
    property: status
    face: draft
    when_values: [draft, in-review]
```

Entities whose `status` is `draft` or `in-review` gain a `@draft` row; their
bare row is untouched and still carries whatever `bare_face:` names.

## Open design questions

These are the reason this is not built yet, not incidental details.

- **What content does the new row carry?** A verbatim duplicate of the bare row
is the obvious default and is probably right for adoption. A transform (drop
some properties, change a status) is more useful and much harder to specify
declaratively.
- **What does the bare row then mean?** It remains the family head and keeps the
`bare_face:` identity. For an entity that is conceptually "only a draft", the
result is a published row and a draft row with the same content, which may not
be what the operator wanted. This needs to be stated plainly in the docs, or the
step will produce a surprise.
- **Idempotence.** Re-running must converge. A destination row holding identical
content is the previous run's copy; anything else is a genuine collision.
`rename_face` already has this contract (`sameContent`) and it should be reused
rather than re-derived.
- **Selection syntax.** `when_values:` on a single enum keeps the parse-time
exhaustiveness story that made `confirm_face` safe. A general `where:` predicate
through `internal/predicate` is more expressive but gives up the closed value
set, so unmatched rows become discoverable only by running it.

## Prior art in tree

`renameFaceStep.Run` already does create-at-coordinate with a collision check
and a crash-convergence test. A copy is that minus the delete, which is the
easier half — the difficulty here is entirely in the semantics above, not the
mechanics.
