---
id: TKT-AWM6L
type: ticket
title: Hide / disable write affordances when the server is read-only (or when an ACL denies the type)
kind: enhancement
priority: medium
effort: m
status: wont-fix
---

## Superseded

**Closed `wont-fix` on 2026-05-20 — superseded by TKT-Y72A (response-level
action affordances).**

The planning exercise for this ticket (see PLAN-B0CI) uncovered an architectural
smell during crit review: gating write affordances on the frontend with a
`useACL()` composable + per-component `v-if` commits the SPA to maintaining its
own authorization evaluator that must track every evolution of the backend ACL
(binary read-only now → per-type in v1 → per-role in v1.5 → per-entity-state in
v2 → …). That's the **duplication trap**.

The right architectural seam is **backend-driven response-level action
affordances** (HATEOAS-flavored): each list / entity / relation / form response
carries the verbs the principal can apply to that specific resource. The SPA
renders affordances iff the corresponding action is present in the response.
Authorization computed once, on the server, per resource.

Read-only mode then **falls out for free**: when ACL is `ReadOnlyACL`, no
`_actions` are emitted on any resource → SPA hides everything → effectively the
same UX this ticket was trying to engineer, but data-driven instead of
flag-driven.

## What survives

The 31-affordance survey in PLAN-B0CI is still valuable — not as a list of
gating sites, but as the inventory of **response shapes that need `_actions` in
their wire contract** (list responses, entity detail responses, relation
responses, form spec responses, settings responses). TKT-Y72A will consume this
inventory.

## See also

- Replacement: **TKT-Y72A** — "Response-level action affordances: backend declares per-resource verbs to drive UI"
- Backend gate this depends on: TKT-GN5LN (ACL v0 PR 1)
- Original design: DEC-RG878 (four-layer ACL model)

## Original content

The original ticket body (before this closure note) lives in `git log` and in
PLAN-B0CI's revision history. Both remain useful reference material for
TKT-Y72A's planning.
