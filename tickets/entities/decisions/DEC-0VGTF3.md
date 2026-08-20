---
id: DEC-0VGTF3
type: decision
title: Pointer keys stay canonical tokens; schema-change re-keying rides a general data-migration system
context: 'Content-states design (FEAT-9CD2MX): the state key must exist as a DB key (entities PK component, relations from_pointer, fsstore filename) under any Go representation, so axis/pointer schema changes imply re-keying stored values regardless of whether the interior type is an opaque canonical token or a structured coordinate. Alternatives considered: surrogate state ids (permanent read/match complexity to avoid a rare bounded migration) and omit-default-axes canonicalization (defers rewrite on axis addition but makes canonical form metamodel-dependent and demands immutable axis defaults — revisit at multi-axis time). Jeroen: a proper data-migration system is needed anyway for entity changes (property/type/enum renames), not just pointers.'
consequences: 'entity.Pointer stays an opaque canonical token (== comparable, one text column, one frontmatter key, store equality-matches never inspects). Metamodel evolution that re-means stored data — property renames, type renames, enum value changes, pointer renames, future axis changes — is handled by a NEW user-data migration system (distinct from internal/migration, which migrates the schema-YAML format when rela evolves). v1 of content states ships detection only: undeclared stored pointers produce a load warning + analyze finding, remedied by the migration system. DOFYR1/Step 1 is NOT gated on the migration system.'
date: "2026-08-19"
status: accepted
---

Decided 2026-08-19 in architect discussion. The A-vs-B pointer representation
question resolved to A (canonical token) once the DB-level analysis showed the
re-keying problem is representation-independent: any state key is a
serialization, so the levers are the key encoding and the migration story, not
the Go type.

The migration system is scoped as its own feature (see the linked feature
entity): needed for ordinary metamodel evolution (rename a property, rename an
entity type, change enum values) independent of pointers; pointer/axis re-key
becomes one migration operation among several rather than ad-hoc tooling.
