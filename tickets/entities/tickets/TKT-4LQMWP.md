---
id: TKT-4LQMWP
type: ticket
title: 'ACL: split write into create/update/delete grants; create implies no read'
kind: refactor
priority: medium
effort: m
status: ready
---

Implements RES-4AS0S4 (Option A, clean variant — no production acl.yaml exists,
so `write` is removed outright rather than kept as sugar).

## Why

`write: [T]` conflates create/update/delete, and the `write⊆read` invariant
(RR-W2J6) forces any writer of T to hold global `read: [T]` → AllowAll. So
"submitter creates tickets but sees only their own (via the created-by
role-relation)" is inexpressible. Splitting create out — with **create implying
no read** — fixes it and preserves the normal SPA Create-button UX (the
affordances wire already distinguishes create/update/delete/rename verbs).

## Change

- **`acl.RoleDef`**: remove `Write []string`; add `Create / Update / Delete
[]string` (each `"*"`-aware). A helper `grantsVerb(role, op, type)` consults the
list matching the op.
- **`policy.Validate` (the invariant)**: was `write⊆read`. Becomes
**`update⊆read` AND `delete⊆read`** — a role that can update/delete a type must
be able to read it. **`create` is exempt** (no read requirement). Update the
RR-W2J6 error text.
- **`authorizeWrite` / `decideFromAttrs`**: thread `WriteRequest.Op` (already
present) into the grant check — pick the create/update/delete list by op instead
of checking the single `Write` list. `OpRename` maps to the update list (rename
is a modification; requires read).
- **Relation writes**: `authorizeRelationWrite` gates on the source type's
grant; a relation create checks the `create` list of `FromType` (consistent with
entity create). Keep the delegate-permission gate unchanged.
- **Affordances resolver**: `_actions.create` ← create grant; update/delete/
rename ← their lists. translateVerb already emits the right Op; only the grant
lookup changes.

## Migration (small — no committed acl.yaml uses write:)

- Go test fixtures that build `RoleDef{Write: [...]}` (~5 files:
resolver_test, declarative_test, affordances_test, affordances_policy_test,
rooted_test) → set `Create/Update/Delete` (or just the verbs the test needs).
- Docs: `docs-project/.../CON-authorization.md`, `docs/security.md` schema
section, regenerate `docs/acl-security.md`. Update the `write:` examples to the
new verbs and document "create implies no read".
- The `.ignored/` sandbox acl.yaml files are gitignored; update the pen-test one
so the submitter→triager→team demo works (submitter: `create: [ticket]`, no
read; reads own via `created-by: confers: submitter`).

## Acceptance

- A role with `create: [ticket]` and NO ticket read CAN create tickets, sees
ONLY tickets it authored (via created-by role-relation), and gets 404 on
unauthored tickets; `_actions.create` is true and the list is empty until it
authors one.
- A role with `update: [ticket]` but no `read: [ticket]` FAILS load validation
(update⊆read still enforced); same for delete.
- Existing full-editor behavior reproduced with `create+update+delete: [T]`.
- `just arch-lint`, lint, and the storetest/ACL suites pass.
