---
id: FEAT-YEC5NQ
type: feature
title: Create related entities from the entity detail page
description: 'From an entity''s detail page, create a new related entity directly — via a button on a related-items section or a header menu — with the relation pre-linked to the originating entity. The operator declares only how: a modal or page-navigation flow (page returns to the originator) and an optional template variant per target type. Which types are offered stays derived from create permission and form availability.'
status: proposed
---

From an entity's detail page, create a **new** related entity without navigating
away to hunt for a form and linking by hand. The relation is set on creation.

## Derived offer, declared flow

Which target types are offered is **derived**, following the rule settled in
TKT-OMUD56: a type appears when the relation can reach it, the principal may
`create` it, and a create form resolves for it. There is no per-type allowlist
to maintain, so the offer cannot drift from ACL or from the registered forms.

What the operator declares is *how* the create happens:

- **`flow: modal | page`** — modal keeps the user on the detail page and refreshes
the section in place; page navigates to the full create form and returns to the
originating entity on submit. Relation-wide, because a flow is a UI choice and
is type-independent.
- **`types.<type>.template`** — an optional preset template variant, keyed by entity
type. A heterogeneous relation reaching several types has no single meaningful
template, so the key must be per type.
- **`in: [header, section]`** — where the button appears. The header menu is the
union of sections that opted in, because a section knows its relation only
through its `traverse:` rule.

## Relationship to inline create

[[FEAT-0YL031]] covers creating a target from a **relation field inside a
form**, where an in-progress draft must survive and modal is therefore
mandatory. This feature covers the **read-only detail page**, which holds no
draft, so both flows are available and the page flow is a legitimate choice for
a long create form.

Both share one pre-link contract and one target-derivation rule.
