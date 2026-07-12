---
id: TKT-4MKKKA
type: ticket
title: Name the reserved-key predicates for entity/relation frontmatter
kind: refactor
priority: low
effort: s
status: done
description: Consolidate copy-pasted reserved-key literal sets into named predicates in internal/entity. Driven by gocleaner diverging-literal-allowlists finding 2efd69b41dfe.
---

## Problem

gocleaner's `02-diverging-literal-allowlists` detector flagged two implicit
concepts guarded by copy-pasted literal sets (finding `2efd69b41dfe`, plus its
group-mate `394d3bcd3f97`):

- The entity identity keys `id` / `type` — checked at 8 sites across `conflict`,
`dataentryconfig`, `importer`, `fsstore`, `storetest`, and `templating`, with
one divergent variant (`+ _template_relations`) in `templating`.
- The relation identity keys `from` / `relation` / `to` — duplicated identically
at 3 sites (`conflict`, `fsstore`, `templating`).

These are the *reserved identity keys* of an entity/relation document: they map
to `entity.Entity` / `entity.Relation` struct fields, must never land in
`Properties`, and validators must accept them as filter targets even though they
are not schema-declared properties. Nothing kept the copies aligned.

## Approach

Reify each concept as one named predicate in the package that owns it
(`internal/entity`): `IsReservedEntityKey` / `IsReservedRelationKey` and their
positive complements `IsEntityPropertyKey` / `IsRelationPropertyKey`. Route all
call sites through them. The templating-only `_template_relations` key stays
local as a named constant (`templateRelationsKey`) composed with the shared
predicate — a template-format concept owned by `internal/templating`, not an
entity concept.

Behavior-preserving by construction: no set changed anywhere. gocleaner re-run
confirms both findings gone.
