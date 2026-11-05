---
id: BUG-O8QP7GX
type: bug
title: 'Relation fields: grants leave unnamed meta fields writable'
description: 'A relation fields: grant only denies the meta fields some role names; every other meta key stays writable on PATCH and conflict resolve.'
priority: high
effort: s
why1: validateRelationMetaWrite only refuses a meta key the verdict's Fields map marks false; a declared meta field no grant names never appears there.
why2: relationAccumulator.verdicts denied only the candidates some grant mentioned instead of every declared meta field the way the entity dimension does.
why3: Relation meta grants were built as a separate accumulator before the entity dimension gained its closed-world universe and role-union rules (BUG-Y5UF3K), and nobody carried those rules across.
why4: No test compiled a policy where a role names one meta field and then writes another; the existing tests stubbed verdicts with explicit false entries.
why5: ACL rules for relations were written separately from the entity rules they mirror, so a semantic change to one (the allowlist) was not carried to the other.
prevention: TestRelationMeta_FieldsGrantIsAnAllowlist pins the allowlist and role union against a compiled policy; TestV1RelationMeta_FieldsGrantIsAnAllowlist and the conflict-resolve cases pin PATCH, unified PATCH, POST and resolve; FieldWriteGate.CheckRelationMetaWrite applies the same verdict on the field-gated manager that MCP writes through.
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

## Description

In `acl.yaml`, a relation grant's `fields:` list only restricts the relation
meta fields that some role names. A meta field that no role mentions stays
writable for anyone who may write the relation. This holds on `PATCH` and on
conflict resolve, because both ask `validateRelationMetaWrite`, and that only
denies keys present in the verdict's `Fields` map.

The cause is in `affordances.relationAccumulator.verdicts`: its deny universe is
the set of fields some grant mentioned. Entity `fields:` grants deny every
declared property that no role grants (the `dimension` type). The relation
accumulator's own comment says meta grants are closed-world, so the gap is a
bug, not a design choice.

Found while fixing PR #2100.

## Fix direction

Make relation meta `fields:` follow the entity allowlist rule. Once a role
declares meta `fields:` for a relation type, every meta key that no role grants
is read-only, including undeclared keys. A role without a meta `fields:` list
for that relation type, holding update on the source row, still writes every key
(role union, BUG-Y5UF3K).
