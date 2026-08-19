---
id: TKT-860BNJ
type: ticket
title: 'Form relation direction: infer from schema, require it when self-referencing (drop the implicit outgoing default)'
kind: enhancement
priority: high
effort: m
status: done
---

## Description

An absent `direction:` on a form relation binding used to mean outgoing. That
default was wrong in two ways: on a to-side binding it silently bound the wrong
side of the edge, and on a self-referencing relation it silently picked one of
two opposite readings. Replace it with metamodel inference (on exactly one side
=> that direction) plus a hard error when the form's entity type is on BOTH
sides. `rela migrate` writes the explicit direction for every unambiguous
binding; the self-referencing ones are deliberately left for the author and
listed by `rela validate`.

## Background

Found while investigating a bug report claiming `rela validate` passed on a
config `rela-server` refused to start on. That claim did not reproduce — both
paths call the same `dataentryconfig.ValidateConfig` — but the investigation
surfaced two real defects.

### Defect 1: migrate stripped `direction: incoming`

`DataEntryCleanupMigration.isRedundantDirection` removed `direction` when the
form's entity type sat on exactly one side, reasoning it was inferable. Nothing
inferred it back: absent parsed as outgoing, and the SPA widgets test `direction
=== 'incoming'` literally. So migrate rewrote a valid config into one
`ValidateConfig` rejects, and re-running migrate reported "No migrations needed"
rather than repairing it — leaving a project permanently unable to start.

### Defect 2: the implicit outgoing default itself

The deeper problem is that absent-means-outgoing is magical for relations. It is
silently wrong on a to-side binding, and on a self-referencing relation
(`depends-on` from ticket to ticket) outgoing and incoming are both valid and
mean opposite things — so the default was guessing. On
`tickets/data-entry.yaml`, 8 of 34 bindings hit that ambiguous case.

## Approach

Direction is resolved by which side the form's entity type sits on:

| Entity type is | Result |
| --- | --- |
| the relation's `from` only | inferred `outgoing` |
| the relation's `to` only | inferred `incoming` |
| both (self-referencing) | error — author must choose |
| neither | wrong-side error (unchanged) |

`InferDirection` in `internal/dataentryconfig/direction.go` is the single shared
rule, used by validation, the server's config handler, and the migration.

Server-side resolution is load-bearing. `RelationCards.vue` and
`RelationPicker.vue` both test `direction === 'incoming'` literally with no
inference of their own, so `resolveFormRelations` (formerly
`resolveRelationWidgets`) fills the direction in before serving config. Without
it an inferred-incoming binding would validate clean and still render the wrong
side in the browser.

## Scope

IN: form relations (`FormRelation`) — where a wrong side binds the wrong entity
type on write.

OUT: `ListColumn`, `FilterControl`, `KanbanCardField`, `CalDAVCollection` still
default to outgoing. A wrong-side column renders an empty cell; a wrong-side
form corrupts a write. Removing the default there is a possible follow-up.

## Acceptance criteria

1. A from-side binding with no `direction:` validates and serves `outgoing`.
2. A to-side binding with no `direction:` validates and serves `incoming` (previously
a wrong-side error).
3. A self-referencing binding with no `direction:` is a validation error naming the
form, entity type and relation.
4. An explicit `direction:` is always preserved verbatim.
5. `rela migrate` writes explicit directions for unambiguous bindings (flat and wizard
steps) and leaves self-referencing ones untouched.
6. `rela migrate` never rewrites a valid project into an invalid one.
7. The cleanup migration no longer strips `direction`.

## Breaking change

Upgrading without running `rela migrate` fails startup for any config with a
self-referencing form binding. The error names the form and relation; migrate
handles everything else. Chosen deliberately over a staged rollout.
