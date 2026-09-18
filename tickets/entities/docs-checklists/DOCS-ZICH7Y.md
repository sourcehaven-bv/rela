---
id: DOCS-ZICH7Y
type: docs-checklist
title: 'Documentation: Create related entities from the entity detail page (TKT-R4BMJM)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported types and functions
- [x] Non-obvious decisions explained with the reason, not just the mechanism

New godoc: `SectionCreate` (and why it declares only *how*, never *which*
types), `SectionCreateTarget`, `SectionOriginRelation` (and why an ambiguous
section gets nothing), `SectionCreate.UnmarshalYAML` (and why every scalar
spelling is refused), `v1.ViewSectionCreate` / `ViewSectionCreateTarget`
(including that the triple is a convenience, not a trust anchor),
`creatableTargets`, `resolveSectionCreate`, `headerCreateMenu`,
`gateCreateRelationAffordances`, `writeCreateRelations`, `buildCreateLinkQuery`,
`SectionCreateButton.vue`.

Updated godoc where this ticket changed a stated rule: `v1.ViewAddInfo` (the
invariant narrowed, and this type is still side-panel-only), `SectionLinkInfo`,
`resolveSectionButtonsWithTraverse` (now ACL-gated), `DynamicForm`'s `embedded`
prop block (the new prop channel and why the empty-query rule still holds), and
the `link_as` comment that was documenting the flag backwards.

## Project Documentation

- [x] `docs/data-entry.md` — new "Creating related entities from a section"
section under Views, plus the `create` row in the section field table
- [x] `internal/dataentry/CLAUDE.md` — new section recording that the
entity-detail view is read-only BY DEFAULT rather than absolutely
- [x] ~~`docs/metamodel.md`~~ (N/A: data-entry config, not the metamodel)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI surface)
- [x] ~~`README.md`~~ (N/A: not a project-level change)

The `docs/data-entry.md` section documents the keys and their defaults, the
derivation rule (and so why there is no per-type allowlist to maintain), both
flows, the per-type `types:` map with the heterogeneous-relation reason, the
header menu, and the refusal for a section with no single originating relation.

It also states plainly that the button follows the grant but is **not** what
enforces it — the server re-authorizes the entity and the relation on submit.
Added on review: without it, "removing someone's `create` grant removes their
button" reads as an enforcement claim.

## External Documentation

- [x] ~~API reference~~ (N/A: `docs/data-entry/api-reference.md` documents the
`_actions` verb set, which is unchanged. The new affordance rides
`ViewSection.create` / `ViewResponse.create`, both documented in the wire type's
godoc, and no verb was added or renamed)
- [x] Upgrade note captured: the side-panel Add button now disappears for a
principal without create permission (AC13). A declared behaviour change on a
shipped surface, recorded in the ticket and in the commit message so it reaches
a release note.

## Verification

- [x] Examples in the docs match the implemented config shape

The `docs/data-entry.md` example was checked against the real prototype config
during manual verification: the same block shape loaded, produced the
affordance, and the deliberately-invalid variant (a `create:` on a section fed
from another collection) was refused at startup with the message the docs
describe.
