---
id: RES-Z5PYPDA
type: research
title: Conditional grant edges with read-time now
summary: Edge property on the grant relation, lowered to a new RelationPredicate.EdgeProps dimension; instant captured at the request edge
status: done
---

## Problem

An external auditor needs read access for a fixed period, for example six weeks.
Organisations with Entra end that access in the IdP. Others need rela itself to
stop counting a grant edge after a date. Today a grant edge counts for as long
as it exists, so ending access depends on someone deleting the edge on time.

## Context

A grant edge is evaluated in two forms, and both must give the same answer:

| Path | Form | Code |
| --- | --- | --- |
| Membership walk | Go, once per request | `acl.IdentityResolver.walkMembers` (`OutgoingRelations`) |
| Per-entity local roles | Go probe | `Request.computeForEntity` (`Graph.HasEdge`) |
| Page-primed local roles | Go batch | `PrimeLocalRoles` (`EdgesBetween`, `OutgoingFrom`) |
| World grant through a relation | Go probe | `Request.permitsWorldThroughRelations` |
| Row gate (list, bare id) | store query | `Request.ReadQuery` → `conferredEdge` |
| Search field scope, flow steps | store query | `Request.LocalRolePredicate` → `conferredEdge` |

- `store.RelationPredicate` has no constraint on the edge's own properties.
It has `EndpointMatch` (properties of the far entity) but nothing for the edge.
- Relations already carry `Properties`. pgstore and sqlitestore keep them in a
`properties` JSON column on `relations`, so a predicate on alias `r` is the same
SQL the entity predicates already emit.
- `store.PropGreaterEqual` / `PropLessEqual` compare byte-wise. The caller must
gate them on the declared type. Byte order equals time order for `date`
(`YYYY-MM-DD`). It does not for `datetime`: RFC3339 values keep their offset
(`...T17:00:00+01:00` vs `...T16:30:00Z`).
- `internal/acl` may import only `entity`, `principal` and `store`. Compiling a
`when:` string through `internal/predicate` against the schema has to happen at
the wiring site, as `visible: when:` already does in `affordances`.
- `requires_permission:` on a role relation gates all write verbs, so editing
`valid_until` on a grant edge is already gated like creating the edge.
- Read-time `now`/`today` for queries is being added by a sibling session
(gap item 3a). Not merged at the time of writing.

## Options

### A. Edge property on the grant relation (recommended)

`heeft_rol` (or `member-of`) carries `valid_until: date`. `acl.yaml` declares:

```yaml
role_relations:
  heeft_rol:
    confers: auditor
    requires_permission: delegate-roles
    when: "edge.valid_until >= today"
```

The condition lowers to `RelationPredicate.EdgeProps []store.PropPredicate`, a
new allowlist dimension evaluated on the edge row inside the same `EXISTS` the
conferred-edge predicate already emits. The Go probes apply the same predicates
to `Relation.Properties` with one shared matcher.

- Pro: no extra hop; one predicate per backend; the edge row is already read.
- Pro: the date sits next to the grant, under the same write gate.
- Con: needs a new store dimension in naive, pg and sqlite, plus the
differential test.

### B. Property of a role-assignment entity

`user --assigned--> assignment --grants--> project`, with the date on the
assignment. The conferral becomes two hops with an `EndpointMatch` in the
middle.

- Con: changes the conferral shape (`conferredEdge`, the primed batch, the
ancestor walk) for every policy; nested hops refuse `EntityInheritThrough`.
- Con: per-row endpoint lookups on the Go paths.

### C. Scheduled job that deletes expired edges

- Con: access survives until the job runs; a stopped scheduler means no expiry.
The brief rules it out.

## Recommendation

Option A.

1. `role_relations.<rel>.when:` is an AND of comparisons
`edge.<prop> (==|>=|<=) (<literal>|now|today)`. Anything else fails the policy
load. Strict `<`/`>` are refused because the store has no exact lowering for
them.
2. The same key on the membership relation's entry filters the member walk,
so one key covers both brief cases.
3. At load (wiring site, with the schema) every referenced property must be
declared on the relation type. Ordered comparisons require `date` with the
default format. `datetime` is accepted only if the read-time-now work lowers
`now` soundly against stored offsets; otherwise it is refused at load with a
message pointing to `date`.
4. The compiled form lives in `acl` as a store-shaped condition with a
symbolic `now`/`today` operand. `Request` binds it once, from the instant the
read-time-now work captures per request. No cache across requests.
5. Fail closed: an empty, list or missing property never matches an ordered
comparison (existing store contract). An ordered comparison also gets an upper
bound of `9999-12-31`, so a non-date string starting with a letter cannot pass a
`>=`. The Go matcher parses the date strictly.
6. `rela acl audit` reports each conditional grant (info finding) with its
condition, and warns when the conditional edge type has no
`requires_permission`.
7. Docs: access ends at the first request after expiry; no job runs.
Open question for design review: a long-lived request (SSE stream, MCP stdio
session) that holds one `Request` past the expiry.

Effort: L. Store dimension (3 backends + conformance + differential), acl
binding on six paths, policy parse and load-time compile, audit, docs.

## Design review resolutions

1. **Instant captured at the edge.** The member walk runs in
`IdentityResolver.ResolveDirect`, before a `Request` exists. The instant is
captured once at the request edge, carried on the `Identity`, and used by both
the walk and the `Request`. Every caller pinned by `TestIdentityEdgeCallers`
supplies it through the same binder.
2. **Unbound instant denies.** A zero instant makes every conditional edge
non-matching, in Go and in the store lowering. Tested per binder.
3. **Per relation type.** A conditional relation type gets its own
`RelationPredicate` branch (`GraphQuery.Any` in `readQuery`, a separate branch
in `LocalRolePredicate`). `EdgeProps` applies only to the conferring edge in its
`EXISTS`, never to `EntityInheritThrough` hops. The differential test pins
`Negate` and inherit compositions.
4. **One comparison.** The Go probes use the store's byte semantics through
one shared function in `internal/store` (also used by graphquerynaive), which
renders a `time.Time` as `YYYY-MM-DD` for `date`. Writes to a relation type
whose `when:` names a property validate that property as a hard error (422), so
malformed dates cannot be stored through the API.
5. **Load refusal.** A `when:` on a relation type without
`requires_permission` fails the policy load, so a grantee cannot extend their
own expiry. Docs note that sync and data migrations write under operator trust.
6. **Graph methods take the condition.** `HasEdge`, `OutgoingRelations` and
the batch methods receive the bound condition and filter on the edge row. A
guard test fails any `acl.Graph` implementation that ignores it.
7. **SSE.** `freshReadGate` also refreshes when the bound day changes, so an
open stream drops an expired grant on its next event.
8. **Zone.** `today` is UTC midnight, as `predicatefns.today()` already is.
Documented.
9. **Audit.** Info finding per conditional grant; the ungated warning is
unreachable after item 5 and is dropped.
