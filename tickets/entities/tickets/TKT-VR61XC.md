---
id: TKT-VR61XC
type: ticket
title: 'ACL increment 2: rela acl can — relation verification (may X create a <rel> edge from <id>?)'
kind: enhancement
status: done
priority: high
effort: s
---

## Increment 2 of 3

Depends on **TKT-K2VN9D** (the `relation_grants:` block). Split from
**TKT-XZEY** at design review (DR-10).

## Why this is not optional

TKT-XZEY names **two** properties that turned a permissions typo into an
outage:

1. no verb that expresses "may add an edge" — fixed by increment 1;
2. **"silent under every static check"** — `acl map`, `acl can` and `acl audit`
   all reported healthy while every relation write was denied at runtime.

Increment 1 fixes (1) and, on its own, makes (2) *worse*: it adds one more
allow source that no tool can see. An operator writing `relation_grants:` would
have **no way to verify it** short of attempting the write in production.

Confirmed scope of the gap: `internal/aclmap` contains **zero**
`acl.RelationSubject` references in non-test code. `Can` takes
`(principal, verb, entityID)` and resolves `ent.Type` (`aclmap/can.go:52-61`);
`CanResult` is entity-shaped throughout (`can.go:24-38`). Every one of
`can.go`, `whocan.go`, `mapall.go`, `mapprincipal.go`, `enumerate.go` is
entity-only.

## Approach — delegate to the real gate, do not re-derive it

```console
$ rela acl can --principal system:scheduler --relation spawnt --from TERUG-1 create
```

Resolve the principal, resolve `--from`'s entity type, build
`acl.RelationSubject{Type, FromType, FromID}`, call `AuthorizeWrite`, print the
`Decision`'s `RuleKind` / `RuleID` / `Reason`.

This **is** the runtime path, so it cannot drift from what the server does —
which is the whole point, given the incident was a divergence between what the
tooling reported and what the gate did. It needs no `AccessRoutes` extension.

Route-level explanation ("which role, via which edge, conferred it") is a
deliberate follow-up: `AccessRoutes` is entity-shaped and extending it is a
much larger change than this ticket needs.

## Acceptance criteria

1. `rela acl can --relation <type> --from <id> create|update|delete` prints an
   allow/deny with the deciding `RuleKind` and `RuleID`.
2. A denial caused by the client ceiling reports `RuleKind: "client-ceiling"`;
   by delegate-X, `"delegate-permission"`; by a relation permission,
   `"relation-grant"` (the increment-1 `RuleKind`, DR-12).
3. Exit code reflects the verdict (matching the existing `acl can` convention).
4. Reproduces the motivating incident **statically**: given the outage's
   `acl.yaml` and metamodel, the command reports DENIED for
   `create --relation spawnt --from <terugkerend id>` — i.e. the check that
   would have caught it "in seconds".
5. Unknown relation type / unknown entity id error clearly, distinguishably.
6. JSON output mode consistent with the existing `acl can` shape.

## Out of scope

- `acl map` / `acl who-can` relation support (both are route-shaped; bigger).
- Route provenance for relation verdicts.

## Files

`internal/aclmap/` (new relation-shaped entry point or a sibling to `Can`),
`internal/cli/acl_can.go`, `docs/cli-reference.md`, `docs/acl-overview.md`.

---

## Implementation notes (2026-08-22)

All 6 ACs met. `rela acl can-relation <principal> <verb> <relation> --from <id>`.

### What landed

- **`aclmap.CanRelation`** (`canrelation.go`) — resolves the source entity for
  its type, resolves the principal the same way `Can` does, and calls
  `req.AuthorizeWrite` with an `acl.RelationSubject`. It **is** the runtime
  gate, so it cannot drift; a re-derived verdict that disagreed with the gate
  would be worse than no check.
- **`ACLCanRelationCmd`** (`internal/cli/acl_can_relation.go`), registered as
  `acl can-relation`. Exit 0 allow / 1 deny. Undeclared relation type and
  missing source entity are distinct errors, never a plain deny.
- **`formatRelationRule`** renders an allow in the operator's vocabulary —
  `relation_grants` vs source-type role grant need different edits to revoke.
- **Audit `A6b-inert-relation-grant`** (Medium): a `relation_grants` entry
  naming a permission no role grants is inert; writes silently fall back to the
  source-type grant.
- **Fixed a false positive A6b exposed:** `A7-dead-permission` counted only
  `requires_permission` as a permission consumer, so it reported EVERY
  correctly-configured relation grant as dead. `relation_grants` is now a
  second consumer. Pinned in both directions
  (`TestA7_RelationGrantsCountAsAPermissionConsumer`,
  `TestA7_StillReportsGenuinelyDeadPermissions`).
- **Demoted increment 1's load-time `slog.Info` to `Debug`** and moved it from
  `LoadPolicy` to the appbuild wiring site. It fired twice per command and on
  every read-only CLI invocation — an INFO line on each `rela list` trains
  operators to filter the logger, defeating its purpose. `acl audit` and
  `acl can-relation` are the operator-facing surfaces; docs updated to match.

### AC4 verified against a real project

Built a project reproducing the outage (`terugkerend --spawnt--> taak`,
`create: [taak]`, `update: [taak, terugkerend]`). The config that passed
`acl map` and `acl audit` during the incident now reports:

```
DENY: system:scheduler cannot create a spawnt edge from TERUG-1 (terugkerend).
      no role grants create on relations from type "terugkerend"
      (rule_kind=role-grant rule_id=-)   [exit 1]
```

Adding the grant flips it to ALLOW via `relation_grants`, exit 0. Both outputs
are quoted verbatim in `docs/acl-overview.md` and were re-run against the built
binary to confirm they match.

### A test that was wrong

`TestCanRelation_VerbsAreIndependent` initially asserted `update` was denied.
It is legitimately ALLOWED — the fixture role holds `update: [terugkerend]`, so
the source-type grant covers it. Rewritten to assert the useful property
instead: update is allowed but attributed to `role-grant`, so the create-only
relation grant is not miscredited.
