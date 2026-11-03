---
id: BUG-826XOX
type: bug
title: 'Relation write grants narrow when a role without a relations: block is added'
description: 'RelationVerdicts counts only roles that declare a relations: grant for the type, so a role conferring a relations: block narrows relation create/remove/meta-write for a principal whose other role may update the type.'
priority: medium
status: backlog
---

## Description

`RelationVerdicts` (internal/affordances/resolver.go, relation_accum.go) opts a
relation type in as soon as one role on the row declares a `relations:` grant
for it. A role without such a grant adds nothing, even when it may update the
source type. So gaining a conferred role with a `relations:` block can stop a
principal creating or removing an edge, or writing its meta, that another of its
roles allowed.

BUG-Y5UF3K fixed the same narrowing for `visible:`, `fields:`, `options:` and
relation meta visibility. This is the write-side remainder.

## Expected

The union rule from BUG-Y5UF3K: a role without a `relations:` grant for the
relation type contributes create, remove and every meta field when it holds
update on the source type and face.
