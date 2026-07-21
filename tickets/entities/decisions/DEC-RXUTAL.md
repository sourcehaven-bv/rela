---
id: DEC-RXUTAL
type: decision
title: 'Data-entry help/schema endpoints are the in-product manual — ungated by principal (declines #1176)'
context: 'IB-review of #1173 (issue #1176) found that GET /api/help/{entityType} returns a type''s schema (properties, per-value descriptions, lifecycle prose) on existence alone, without consulting the readGate that attachACLRequest already puts in context. A DenyAll-on-persoon principal can thus learn which categories of personal data the deployment holds, without reading a single instance. Severity: Low (schema-metadata, not instance data).'
consequences: 'Decline the gate. The help/schema endpoints are the in-product manual: like any product/admin manual, they document the system''s shape (types, fields, statuses, transitions) to every authenticated user, independent of who may act on instances. The schema structure is in fact already served ungated by GET /api/v1/_schema — the endpoint the SPA loads at startup to build its forms; the help endpoint merely renders what _schema already exposes. The protected asset is instance data (whose DOB, which person), which stays gated by readGate on the instance reads. Gating help per-principal would gate the manual per-reader, which no manual does, and would be a half-measure while _schema / affordance / form-render surfaces still expose the same type structure the SPA needs. If a reviewer insists on a token mitigation, the correct minimal primitive is ReadQuery(type).DenyAll (type-level ''no read at all''), NOT the finding''s suggested PermitsRead (which needs an instance id help has none of) — but this adds an affordance/help inconsistency for a Low-severity metadata leak and is not adopted. A coherent ''schema visibility'' model, if ever wanted, is separate larger work, not a three-line bolt-on.'
date: "2026-07-21"
status: accepted
---

## Decision

The data-entry documentation endpoints — chiefly `GET /api/help/{entityType}`
and the schema-describing surfaces — intentionally expose
entity/property/lifecycle **structure** to any authenticated principal, ungated
by per-principal ACL. Per-principal ACL governs acting on **instances**;
instance reads remain gated by `readGate` (`PermitsRead` / `ReadQuery`).

## Why (grounds CONTROL-5-15)

Documentation ≠ data. A product/admin **manual** lists every entity type, field,
status meaning, and workflow transition, and ships to all users regardless of
what any individual may do — a support agent reads the manual for an admin-only
workflow they can never execute. `/api/help/{type}` is exactly that: the
in-product manual (#1173 made this explicit — value descriptions and mermaid
lifecycle diagrams are manual paragraphs). The reporter's own `persoon` example
— "which categories of personal data we hold" — is a page-one manual fact (the
*shape*), not a disclosure of any betrokkene (the *data*, which stays gated).

The schema structure is in fact already served ungated by **`GET
/api/v1/_schema`** — the endpoint the SPA loads at startup to build its forms.
The help endpoint isn't even the first place a principal sees the type
structure; it renders what `_schema` already exposes.

## Not adopted

- **Per-principal gate on help.** Gates the manual per-reader; half-measure while `_schema` / affordance surfaces still expose the same type structure the SPA legitimately needs.
- **The finding's `PermitsRead` fix.** Needs an instance id the help endpoint doesn't have; the shape mismatch is itself a tell that help isn't instance-authorization-shaped.

## If a reviewer insists on movement

Minimal defensible mitigation is `ReadQuery(ctx, type).DenyAll` (hide help only
from a principal with **zero** read on the type) — one line, existing primitive,
satisfies the literal finding. Accepts an affordance/help inconsistency for a
Low-severity metadata leak; deferred unless required.
