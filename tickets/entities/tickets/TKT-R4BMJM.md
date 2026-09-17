---
id: TKT-R4BMJM
type: ticket
title: Create related entities from the entity detail page (section + header buttons, modal or page flow)
kind: enhancement
priority: medium
effort: l
status: in-progress
---

## Description

The entity detail page (`/entity/:type/:id`, rendered by `EntityDetail.vue`)
shows related items in sections, but offers no way to create a new related
entity. A user who wants to add a task to an epic must navigate away, find the
create form, fill it in, and link the relation by hand.

This ticket adds operator-configurable create buttons to that page. The new
entity is pre-linked to the originating entity, so the user never links
manually. The operator chooses a modal or a page-navigation flow, and may preset
a template variant per target type.

## What already exists

Most primitives are in place. The gap is narrower than it first appears.

| Piece | Where | State |
| --- | --- | --- |
| Add-target derivation (form exists) | `internal/dataentry/sections.go:436` `resolveSectionButtonsWithTraverse` | Confined to `_sidepanel` **by deliberate decision** (TKT-651W), not by accident |
| Create-permission check | `affordances.go:155` `computeCollectionActions` | **Not called by the resolver at all** — the gate must be written, not rewired |
| Form resolution per type | `views_handler.go:773` `createFormForType` | Works |
| Pre-link query params | `DynamicForm.vue:673-678` reads `link_relation` / `link_peer` / `link_as`; post-create reverse link at `:1414-1434` | Works |
| Return-to-origin | `utils/returnPath.ts`, `useBackTarget.ts`, `DynamicForm.vue:1509` | Works, open-redirect guarded |
| Named template variants | `templating.EntityTemplates`, `GET /api/v1/_templates/{type}`, picker at `DynamicForm.vue:745-751` | Works; templates already carry relations |
| Modal-hosted create form | `useInlineCreate.ts`, `INLINE_CREATE_DEPTH`, embedded `DynamicForm` (TKT-OMUD56) | Works from relation fields |

So the UI mechanisms mostly exist. What does **not** exist is the authorization:
the resolver derives targets from "a create form is configured" alone, with no
principal involved. Adding that gate is the largest piece of new work, and it
also fixes `_sidepanel`, which shows Add buttons today for types the caller may
not create.

## Relationship to TKT-651W (read this first)

This feature **existed on the detail page and was deliberately removed.**
TKT-651W (`done`, implementing FEAT-K111 "Entity views are strictly read-only")
stripped `+ Add` / `Link Existing` from these exact sections because "editing
the graph from inside a read view blurs the line between viewing and editing".
The invariant is enforced by `TestV1Views_NoAddOrLinkInfoOnSections`
(`internal/dataentry/api_v1_test.go:4020`) and by doc comments at
`internal/apiwire/v1/responses.go:1088-1092` and
`internal/dataentry/sections.go:429-435`.

**This ticket does not reverse that decision; it narrows it** to "read-only
unless an operator explicitly asks otherwise". Consequences:

- The affordance is opt-in. No `create:` block means no button and nothing on the
wire, so every existing deployment is unchanged on upgrade.
- The guard test **narrows rather than inverts**: its five existing cases stay green,
and new cases assert presence only where a `create:` block is configured. It
remains a live guard against accidental re-bleed.
- The three enforcement comments are updated to state the narrowed rule and cite both
tickets. They are not deleted.
- `v1.ViewAddInfo` is **not** reused — its doc comment forbids it, and RR-R8X6
predicted that its misleading name would tempt exactly that. New types instead.

## Bug folded in: the side-panel Add button never pre-links

`SidePanel.vue:74-79` pushes query params `_relation` / `_linkAs` / `_peerId`.
`DynamicForm.vue:673` reads `link_relation` / `link_peer` / `link_as`. The
underscore-prefixed names are read by nothing — a repo-wide grep finds exactly
one write site and zero read sites. The existing Add button therefore opens a
create form with no relation context, and the user must link by hand anyway.

Folded in here rather than filed separately because this ticket defines the
pre-link contract, and both surfaces must speak it. One contract, one test.

## Config shape

Targets stay **derived** (the type is reachable by the relation, the principal
may create it, and a form resolves) — consistent with TKT-OMUD56's settled rule
that the offer is computed, not declared. The `create:` block declares only
*how*.

The affordance is **opt-in**. A section renders a button only when it carries an
explicit `create:` block; a section without one behaves exactly as today. See
"Relationship to TKT-651W" below — this is why.

```yaml
views:
  epic:
    sections:
      - heading: Tasks
        source: tasks
        display: table
        create:
          in: [header, section]   # default: [section]
          flow: modal             # or: page  (default: modal)
          types:                  # optional, per target type
            task: { template: bugfix }
            bug:  { template: regression }

      - heading: Notes
        source: notes
        create: {}                # opt in, all defaults
                                  # (modal, section-only, no template)

      - heading: Audit trail
        source: audit
        # no create: block -> read-only, as today
```

- **`flow`** is relation-wide. A flow is a UI choice and is type-independent; letting
one type in a section open a modal while its sibling navigates would read as a
bug.
- **`types`** is keyed by entity type, because a template variant only exists per type
— a heterogeneous relation (`to: [task, bug]`) has no single meaningful
`template:`. This mirrors `ViewSection.ParentColumns` / `ChildColumns`, which
already solved the same "one level, several types" problem in this struct. A
type absent from `types:` still gets a button, just with no preset template.
- **`in`** places the button. The header menu is the **union of sections that opted
in**, so a relation cannot appear in the header without a section that produced
it. This matters structurally: a section knows its relation only via its
`traverse:` rule, so aggregating from sections is the only place the relation is
actually known.

### Button rendering

- One reachable type: a direct button, `[+ Task]`.
- Several: a menu, `[+ New]` -> Task / Bug / Note. The picker appears only when the
derived target count exceeds one, so a homogeneous relation never costs a click.

## Flows

**Modal** — the create form opens embedded, exactly as inline-create does from a
relation field today. On success the section refreshes in place. The detail page
is read-only with no draft state, so unlike TKT-OMUD56 there is no "must not
navigate" constraint; modal is the default because it preserves scroll position
and context.

**Page** — router push to `/form/:formId` carrying `link_relation`, `link_peer`,
`link_as`, `return_to` (the originating entity's path) and the template
selection. `DynamicForm` already honours all but the last. On submit the user
lands back on the originating entity with the new item present.

## Scope

**In scope**

- `create:` block on `ViewSection` in `internal/dataentryconfig`, with load-time validation.
- Extending add-target resolution to the `_views` detail path (currently `_sidepanel` only).
- Carrying the relation type from the `traverse:` rule onto the resolved section affordance.
- Affordance fields on the `_views` wire response.
- Section buttons and header menu in `EntityDetail.vue`.
- Modal and page flows; return-to-origin on page flow.
- Template preselection, per target type.
- Fixing the `_relation` / `_linkAs` / `_peerId` mismatch in `SidePanel.vue`.

**Out of scope**

- Linking *existing* entities from the detail page (`LinkInfo` equivalent). Separate concern.
- Reworking `EntityDetail.vue`'s 3072-line six-branch section render (FEAT-KQ45P).
- Relation edge properties on the created link — `RelationCards` territory.
- Extending the `_actions` closed verb set. It has no vocabulary for "create type X via
relation Y" and is guarded by a lint invariant; the affordance rides the
section, not `_actions`.

## Acceptance criteria

1. A section carrying a `create:` block renders a button when at least one target
type is derivable. A section with no `create:` block emits nothing on the wire
and renders nothing — the existing guard test's five cases stay green. The
button is also absent when the principal may create none of the targets.
2. A relation reaching one type renders a direct button; several types render a picker.
3. `flow: modal` opens an embedded create form and, on success, refetches the view
without a route change. (`loadView()` is a whole-view refetch, so "in place"
means "no navigation", not a partial update.) When the principal may create but
not read the new type, the refetch cannot show the row: surface a toast naming
the created id rather than failing silently or claiming an error.
4. `flow: page` navigates to the create form and, on submit, returns to the originating
entity with the new item visible in its section.
5. The created entity is linked to the originator on both `link_as: to` and
`link_as: from`, with no manual linking step.
6. A `types.<type>.template` preselects that template variant in the opened form; a type
with no entry opens with the form's normal template default.
7. `in: [header, ...]` contributes the section's relation to a header menu; the header
menu is absent when no section opted in.
8. The side-panel Add button pre-links its relation (the param-mismatch fix), verified by
a test that would have failed before.
9. Config load rejects an invalid `create:` block: unknown `flow`, unknown `in` value,
a `types:` key naming a type the relation cannot reach or for which no form
resolves, a template variant that does not exist for that type, a block on a
section whose relation cannot be resolved, or any scalar (`create:` is a
mapping; absence is how a section opts out).
10. A principal who may not create type X never receives a button for X, and a forged
request to create it is refused by the write path. Edge refusal is asserted on
**both** `link_as: to` and `link_as: from` (they take different server code
paths) against a schema declaring a non-creatable relation verdict — undeclared
relation types are default-permissive, so a verdict-free schema would pass the
test vacuously.
11. An unresolvable `link_peer` produces a visible failure, never a silently created
but unlinked entity.
12. A create issued from a non-default world lands the new entity at that world's face,
on both flows.
13. The side-panel Add button disappears for a principal who may not create the target
type — the new ACL gate, a declared behaviour change on a shipped surface.
14. The `_views` query count is unchanged at 10 and 50 rows with `create:` enabled
(`querybudget_test.go` budget test).

## Risks

- **Affordance/write divergence.** A button shown for something the write path refuses.
Mitigated by deriving from `computeCollectionActions` — the same call the list
handler uses — and never inferring permission from section visibility.
- **`EntityDetail.vue` size.** Buttons touch six display branches. Mitigated by one
section-header button component used by all branches, rather than six copies.
- **Traverse-to-section relation threading.** A section may be fed by a `recursive:` or
multi-hop rule with no single originating relation. Such sections must degrade
to no button rather than guess a relation.
