---
id: TKT-VR61XC
type: ticket
title: 'ACL increment 2: rela acl can — relation verification (may X create a <rel> edge from <id>?)'
kind: enhancement
status: ready
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
