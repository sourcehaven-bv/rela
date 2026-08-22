---
id: FEAT-9CD2MX
type: feature
title: Content states and worlds (pointers)
summary: Metamodel-declared content states per entity (draft/review/published) with worlds as the reified read coordinate, world-shaped read grants, and a copy kernel with declared definitions (promote/revise/CoW).
description: 'Entity types declare named pointers (draft/review/published); each names a content state with its own content and content-scoped relations. Worlds are metamodel-declared read coordinates resolving each entity to at most one face; reads are granted per world, writes per state; promote/revise ride an audited copy kernel with declared definitions. Design: .ignored/pointer-design.md (v2).'
priority: high
status: planned
---

Design: `.ignored/pointer-design.md` (v2, 2026-08-19). Supersedes the storage
halves of RES-NH3P12 and RES-GFWP85; retains RES-NH3P12's per-state read-verdict
conclusion in world-shaped form.

Core decisions: typed `(id, pointer)` references with the `@` form only at
boundaries; exactly N states per entity (no third "base" population); relation
scope (identity vs content) as a metamodel property; worlds as declared
resolution functions with an at-most-one-prime invariant; reads address worlds,
writes address states; copy kernel + named declared copy definitions with a
same-entity-elevated / cross-entity-visible-only security split; attachments as
state-linked references to shared blobs.

Sequencing: security gates (membership gating, MCP ACL) block shipping; Step 1
store contract → Step 2 worlds → Step 3 ACL → Step 4 copy → Step 5
analysis/search.
