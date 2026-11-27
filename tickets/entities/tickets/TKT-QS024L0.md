---
id: TKT-QS024L0
type: ticket
title: Computed group keys for data-entry lists
kind: enhancement
priority: medium
effort: m
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

`group_by` on a data-entry list takes one property or one relation. Add computed
groups: an ordered `groups:` list whose entries carry a predicate `condition:`.
The first matching group takes the row; an entry without a condition is the
fallback and must be last. Rows matching no group land in an "Other" section.

The server evaluates the conditions through `internal/predicate`, on the
redacted row, once per group per page (`MatchPage`), and stamps each row with
`_group`. The SPA splits the rows by that key. `today()` is UTC, as in every
other condition surface.

## Acceptance

- Config validation: ids, one fallback, last; conditions compile at load.
- List rows carry `_group` when the request names the list.
- Budget test: calls flat at 10 and 50 rows.
- ACL test: a condition over a hidden field cannot place a row by its value.
- SPA renders the declared sections in order.
- Docs in docs-project; todo-app example uses it.

## Threat model

- Assets: entity field values and entity existence.
- Input: `data-entry.yaml` (operator config, not secret) and `list_id` on the
request (an id, never an expression).
- Threat: a group condition over a hidden field places rows by its value. Mitigation:
conditions judge the redacted row, so the field binds nil.
- Threat: an existence oracle. Mitigation: `_group` is set only on served rows;
no counts or truncation before the gate.
- Threat: cost. Mitigation: one batched pass per group, at most 20 groups, one
compile per request; budget test.

## Depends on

The todo-app example uses the trigger `unset` output from TKT-61Y46A0 (#2575).
