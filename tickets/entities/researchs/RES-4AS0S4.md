---
id: RES-4AS0S4
type: research
title: Split create from update/delete in ACL grants so create-without-global-read is expressible
summary: 'Recommend Option A: add create/update/delete grant lists (Write kept as all-three sugar = zero migration), relax write⊆read to update⊆read+delete⊆read with create exempt, parameterize decideFromAttrs by WriteRequest.Op. Minimal, backward-compatible first slice of IDEA-HUWQ; unblocks create-without-global-read so submitters see only their own.'
status: done
---

## Problem

The ACL `write` grant conflates **create / update / delete** into one type-list
(DEC-RG878: "`write: [type,...]` — entity types this role can
create/update/delete"). The `write⊆read` invariant (RR-W2J6) then requires any
role with `write:[T]` to also hold `read:[T]` — and read is **type-level**, so a
global role that can write T necessarily gets `AllowAll` read of T
(`readquery.go`: a global role granting read short-circuits to AllowAll before
any per-entity scoping runs).

Consequence: **"submitter creates tickets but sees only their own"** is
**inexpressible** today. The per-entity read mechanism exists and works
(`created-by: confers: submitter` transfers a per-entity read via the role-
relation `HasInbound` query), but a submitter who can *create* must hold a
global ticket-writing role → global ticket read → AllowAll wins. Verified live:
sam (submitter) reads TKT-1 (an ACME ticket they did not author) → 200.

The dodge (an elevated "submit" action via rela.bypass_acl) **breaks the normal
create UX**: with no `write:[ticket]`, the SPA's Create button (gated on
`listResponse._actions.create`) disappears; submission becomes a separate action
invocation, not the "Create button → form → open for editing" flow. So the clean
fix is to make **create a first-class grant that does NOT imply read**.

This narrowly **revises DEC-RG878** (which deliberately bundled write=CUD) and
is the concrete first slice of **IDEA-HUWQ** (the full named-permission catalog,
effort=large, sequenced to "return after TKT-VMD8 lands" — which it now has).

## Context (codebase grounding)

- `acl.RoleDef.Write []string` is the single mutation grant (`policy.go`).
`authorizeWrite` routes BOTH entity and relation writes through
`decideFromAttrs(attrs, target, ...)` against this one `Write` list
(`authz_write.go`). Entity create authz uses globals-only (no entity ID yet);
update/delete fold in per-entity local roles.
- `write⊆read` validator (`policy.Validate`) loops `role.Write` and requires
`roleGrantsRead(role, w)` — the exact coupling to relax.
- `acl.Op` constants already exist: `OpCreate / OpUpdate / OpDelete / OpRename`
(plus the affordance verbs). `WriteRequest.Op` already carries the verb — it's
just not consulted when deciding grant membership (decideFromAttrs ignores Op,
checks type ∈ Write).
- The **affordances wire already has verb granularity**: `_actions` exposes
`create` (per-collection) and `update/delete/rename` (per-item); the SPA gates
the Create button on `_actions.create` and per-item buttons on their verb
(TKT-Y72A, done). So the *wire* is verb-aware; only the *policy grant* is not.
Whatever schema we pick must let the affordances resolver answer "can this role
create T?" distinctly from "update T?".
- `translateVerb` in `internal/dataentry/affordances.go` is the single
constructor for `acl.WriteRequest{Op:...}` (grep-enforced) — the natural place
the verb→grant mapping lands.
- Prior art already surveyed in DEC-RG878 (Plone/Casbin/OpenFGA/Cerbos/Oso/
Postgres-RLS/IAM). Plone's permission model (Add X / Modify / Delete as distinct
permissions) is the direct precedent and the inspiration IDEA-HUWQ cites. We
build on, not re-run, that sweep.
- Related backlog: TKT-XZEY ("extend WriteRequest for parameterised verbs —
transitions, relations"). The create/update/delete split is the prerequisite
shape that parameterised verbs slot into.

## Options

### Option A — Add `create` / `update` / `delete` grant lists alongside `read`

`RoleDef` gains `Create []string`, `Update []string`, `Delete []string` (each
`"*"`-aware). `Write []string` is kept as **sugar** meaning "all three" for
backward compat (every existing acl.yaml keeps working). `decideFromAttrs` is
parameterized by Op → picks the matching list (Write expands to the union).

- **Invariant becomes:** `update⊆read` + `delete⊆read` (you must read to modify),
**`create` has NO read requirement.** A role can hold `create:[ticket]` with no
ticket read; reads then resolve per-entity via `created-by`.
- Pros: minimal schema delta; `Write` sugar = zero migration of existing
policies; maps 1:1 onto existing `acl.Op`; affordances resolver asks the right
list per verb; smallest resolver/validator change. Matches Plone (Add/Modify/
Delete as separate permissions).
- Cons: three new fields (vs a generic mechanism); doesn't yet address the
*other* verbs IDEA-HUWQ wants (script.run, analyze.run, audit.read) — but is
forward-compatible with a later catalog.
- Effort: **small-medium**. RoleDef + Validate + decideFromAttrs(Op) + affordances
  + docs. No acl.yaml migration (Write sugar). ~1 ticket.

### Option B — Generic per-verb permissions map: `permissions: {create: [ticket], ...}`

Replace/augment Write with a `map[verb][]type`. More general; the seed of
IDEA-HUWQ's catalog.
- Pros: one mechanism extends to script.run/analyze.run later; closer to the
end-state catalog.
- Cons: bigger schema change; needs a verb vocabulary decision NOW (the thing
IDEA-HUWQ explicitly deferred until the verb list settles); larger migration
story; over-reaches for the immediate need.
- Effort: medium-large. Risks pulling in the whole IDEA-HUWQ scope prematurely.

### Option C — Special-case: a `create_without_read` boolean on the role/grant

A flag that exempts create from `write⊆read`.
- Pros: tiniest change.
- Cons: a wart, not a model; doesn't generalize; leaves update/delete still
conflated with create under `write`; the affordances resolver still can't tell
create from update at the policy level. Rejected.

## Recommendation

**Option A** — add `create` / `update` / `delete` grant lists, keep `Write` as
"all-three" sugar, relax the invariant to `update⊆read` + `delete⊆read` (create
exempt), and parameterize `decideFromAttrs` by `WriteRequest.Op`.

Rationale: it's the **minimal, backward-compatible** change that makes
create-without-global-read expressible, preserves the normal SPA create UX
(`_actions.create` becomes truthfully answerable from a `create` grant), maps
directly onto the `acl.Op` constants and the affordances wire that already
distinguish these verbs, and requires **zero migration** of existing acl.yaml
(the `Write` sugar expands to all three). It is the concrete first slice of
IDEA-HUWQ without prematurely committing to the full catalog/verb-vocabulary
(which IDEA-HUWQ itself deferred until the verb list settles). Scripts/actions
ACL-gating (IDEA-HUWQ category 1, `script.<name>`) stays out of scope here —
it's a separate verb family with its own surface; this research deliberately
scopes to the CUD split that unblocks the submitter use case.

Tradeoff accepted: three concrete fields now rather than one generic map; a
future catalog migration (Option B/IDEA-HUWQ) would fold these into the general
mechanism, but the `Write`-sugar + per-verb-list shape is a natural subset that
compiles cleanly into a catalog later.

### Concrete schema + minimal changes

```yaml
roles:
  submitter:
    create: [ticket]     # may create tickets; NO read implied
    # read comes per-entity via created-by: confers: submitter
  editor:
    write: [ticket]      # sugar = create+update+delete (unchanged, still needs read:[ticket])
    read:  [ticket]
```

1. `RoleDef`: add `Create/Update/Delete []string`; add a helper
`grantsVerb(role, op, type)` that consults the matching list, treating `Write`
as the union for all three.
2. `policy.Validate`: loop Update+Delete (and Write-expansion) for `⊆read`;
**skip Create**. Update the RR-W2J6 error text.
3. `authorizeWrite`: pass `s.Op` into `decideFromAttrs`; check `grantsVerb`
instead of `roleGrantsWrite`.
4. Affordances resolver: `_actions.create` ← `grantsVerb(create)`, etc. (likely
already routes through translateVerb → Op; wire the grant lookup per Op).
5. Docs: DEC-RG878 amendment (or a new decision recording the revision);
docs/security.md acl.yaml schema; the in-tree `tickets/acl.yaml` and others keep
working via Write-sugar (verify with a load test).
6. Tests: submitter with `create:[ticket]` + no read can create + sees only own
(via created-by); cannot read unauthored; `_actions.create=true` / list empty
until they author; an `update:[ticket]`-without-read role fails load validation
(update⊆read still enforced).
