---
id: DOCS-JQAWR8
type: docs-checklist
title: 'Docs: direction and target_type on relation-cardinality gates'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported types and functions have godoc
- [x] Non-obvious decisions explained with WHY, not just what
- [x] Nil contracts stated where they are not expressible in the type

`validation.Graph`, `Related` and `Direction` (`internal/validation/graph.go`)
carry the two contracts a future change is most likely to break: **one element
per edge** (ids may repeat, because relation identity is `(From, FromFace, Type,
To)` — collapsing to a set would silently re-scope every gate counting
face-tailed edges), and **`Resolved` is the fail-closed signal** (an unreadable
far end must stay distinguishable from an absent edge, or a `max: 0` gate passes
precisely because it could not look).

`validationgraph.RelatedEntities` documents why the query keys on `EntityID` and
never `From` — `store.RelationQuery` honors `Direction` for
`EntityID`/`EntityIDs` only, so the obvious spelling returns outgoing edges
while claiming to be incoming. The package doc says why the package exists at
all (neither entry point into a `validation.Service` can otherwise share a
store-backed adapter).

`RelationConstraint.Direction` (`internal/metamodel/types.go`) records both
refusals and their reasons: symmetric relations (one stored row, so the two
endpoints would disagree) and the entity-level nature of incoming counts.

## Project Documentation

- [x] `docs/metamodel.md` updated
- [x] `docs-project/entities/guides/GUIDE-metamodel.md` updated (mirror)
- [ ] ~~CLAUDE.md~~ (N/A: no new cross-cutting convention — the seam follows
the consumer-side-interface rule already documented there)
- [ ] ~~docs/cli-reference.md~~ (N/A: no command changed)
- [ ] ~~docs/data-entry.md~~ (N/A: no UI surface)

The shipped "Relation Cardinality Validation" section gained both keys in its
field table, a worked example using the motivating shape (an incoming
`gaat_over` from a `taak`), and a "Direction and target type" subsection
explaining *why* `target_type` cannot be a `where` clause: `where` filters an
entity's **properties**, and a type is not a property.

Every load error is listed with the reason it is an error rather than a warning
— each would otherwise produce a rule that counts nothing and therefore passes
forever. The partial-presence case is called out explicitly (a `where` property
only SOME reachable types declare is legal; one that NONE declares is not),
since that is the difference between a legitimate heterogeneous gate and a typo.

A callout documents that incoming counts are entity-level and will report once
per face on a faced type, with the remedy (`faces:`).

## External Documentation

- [ ] ~~README.md~~ (N/A: not a project-level change)
- [ ] ~~Release notes~~ (N/A: handled by release tooling from commit messages)

## Verification

- [x] Examples in the docs actually work

Verified against a real project rather than written from memory: a
`procedure`/`taak`/`terugkerend` graph where the procedure whose only edge comes
from a recurring schedule violates, and the one with an open task passes.
Completing the task makes it violate; reopening clears it. Flipping `direction`
to `outgoing` produced a load error rather than a silently-empty count. Each
load error quoted in the docs is the string the loader actually emits —
triggered deliberately in that project and copied from the output.

**One line of the documented example is NOT covered by that run.** The example
carries `faces: [vastgesteld]`, and the project I verified against is faceless:
`rela create` cannot yet name a face (TKT-2RQMV4), so building a faced fixture
by hand was a bigger detour than the claim was worth. The `faces:` key itself is
pre-existing and separately tested (`internal/validator/faces_test.go`), and it
composes with `relations:` the same way it composes with `when:` — but the
specific combination in that code block was reasoned about, not executed. Stated
here rather than left for a reader to assume otherwise.
