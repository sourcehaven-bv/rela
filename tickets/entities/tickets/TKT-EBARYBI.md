---
id: TKT-EBARYBI
type: ticket
title: 'Relation checks in enforce: write validation rules'
kind: enhancement
priority: medium
effort: m
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

A validation rule with `enforce: write` may read only the entity being written.
`related(...)` is refused at load, so a rule such as "a risk with status
`in_behandeling` has at least one treatment plan (`heeft_behandelplan`)" stays
advisory. GRC deployments need such rules to refuse the write.

Allow a bounded `related(...)` in an enforced rule and check it at write time
against the raw relations in the store.

## Design

- Surface: `related(entity, 'rel')` and `related(entity, 'rel', {type = 'x'})`.
One hop, no property or `id` constraint. `count_relations` and `has_relation`
are not in the validation environment, so `rela validate` could not agree with
the write path; `related(...)` is answered by the same `relresolve` code in
both. Roll-ups and Lua stay refused. A type with `faces:` is refused for a
relation rule (a traversal is answered from the default face).
- Semantics: a relation rule refuses a write only when the write turns a
satisfied rule into a broken one, for an entity that existed before the write.
Creates are not checked (a new entity has no edges until the request links it),
so create-then-link and edge-by-edge sequences never stop halfway. Data that
already breaks the rule stays editable.
- Checked on: entity update and patch (old and new state, the same edges);
relation create, delete and `ReplaceRelations` (both endpoints, before and after
the edge write, in one `store.Store.Tx`); data-entry PATCH `relations`.
- Combined PATCH (decision, from the security review): a data-entry PATCH that
changes properties and edges a rule reads is written with
`entitymanager.PatchWithRelations`. The replace is authorized and validated
before the patch. Inside one `store.Store.Tx` every check of both writes runs
first (the update's rules on the new state and the edges the replace leaves; the
replace's existing-edge, bound and far-end rule checks), then the entity, then
the edges. So an ACL denial, a bound or a far-end rule refusal of a counted edge
stores neither part. The earlier design judged the update on pending edges and
wrote them after the update commit, so a failed edge write left an entity that
broke the rule. Rejected alternatives: compensating the update after a failed
edge write (it would rerun automations and cannot restore values hidden from the
caller), and counting only edges that passed a dry run (races and far-end
refusals still fail after the commit).
- Not checked: creates, entity deletes (a neighbor may be hidden from the
caller, so refusing would reveal it), automation, copy and conversion edge
writes, batch and linked-create edges, imports and migrations. `rela analyze
validations` still reports those.
- ACL: the check counts raw edges through an ungated `relresolve` binder over
the transaction view. The 422 names the rule and its description, never an
entity. The remaining channel is one bit about an end the caller can read:
whether it has another edge the rule counts. On a relation write that end may be
the far end, so the bit can show an edge to a hidden entity. Recorded in
`docs/acl-security.md`.
- Performance: one `store.MatchingIDs` query per distinct traversal per
endpoint per phase. Only rules that read the written relation type are
evaluated.

## Acceptance

- Table-driven tests in `internal/writerule` and `internal/entitymanager`:
update transition, legacy row, relation delete, create-then-link, ACL (hidden
neighbor counted, message names no entity), inverse direction.
- A combined PATCH whose counted edge is refused (ACL, far-end rule) stores
neither the properties nor the edges.
- Docs: metamodel guide validations section.
