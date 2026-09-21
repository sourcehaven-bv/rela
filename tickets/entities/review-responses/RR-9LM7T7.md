---
id: RR-9LM7T7
type: review-response
title: entitymanager.CreateRelation never validates FromFace against the relation type's scope, so Lua becomes the first caller to pass it unchecked
finding: 'The entity create path has requireCreateFaceFor (internal/entitymanager/core.go:321-353), a symmetric allowlist. The RELATION path has no equivalent: opts.FromFace flows from caller into the ACL subject (manager.go:1853), the duplicate pre-check (:1886), rel.FromFace (:1892) and store.RelationData.FromFace (:1926) with no metamodel consultation — grepping Scope across manager.go and core.go finds only an unrelated doc comment. This is invisible today because every existing caller COMPUTES the tail from the metamodel rather than accepting it: internal/dataentry/relations_modern.go:300-303 zeroes it for identity scope. This ticket makes Lua the first caller to accept the face as raw caller input and pass it through unexamined. The plan flagged this as an edge case rated Low with ''confirm during implementation''; the answer is that the manager does not refuse, it stores the face. An identity-scoped edge written at @draft is then invisible to readers querying the zero tail (relations_direction.go:218), so an existence check reads false and a script re-creates it — unbounded duplicate edges at phantom tails.'
severity: critical
resolution: 'Accepted. requireRelationFaceFor added to the ticket scope as item 3: a relation-side twin of requireCreateFaceFor in internal/entitymanager/core.go, called from CreateRelation and UpdateRelation BEFORE the ACL subject is built, so an unvalidated coordinate never becomes an authorization coordinate and the check still runs on the bypassACL path. Rules: identity scope requires the zero face; content scope on a faced source requires a declared face; content scope on a faceless source requires zero. AC 7 pins it from both the ordinary and elevated bindings. Effort revised s -> m to cover the entitymanager work.'
status: addressed
---

## Finding

The entity create path validates the face symmetrically — `requireCreateFaceFor`
(`internal/entitymanager/core.go:321-353`): a faced type must name a declared
face, a faceless type must name none.

**The relation path has no equivalent.** `opts.FromFace` travels from the caller
to storage untouched by the metamodel:

- `internal/entitymanager/manager.go:1853` — into the `acl.RelationSubject`
- `:1886` — into the duplicate pre-check
- `:1892` — onto `rel.FromFace`
- `:1926` — into `store.RelationData.FromFace`

Grepping `Scope` across `manager.go` and `core.go` returns only an unrelated doc
comment at `manager.go:301`. There is no `relDef.Scope` check on the write path
at all.

## Why this is latent today and becomes live with this ticket

Every current caller **computes** the tail from the metamodel rather than
accepting it. `internal/dataentry/relations_modern.go:300-303`:

```go
newTail := entity.Face("")
if !incoming && relDef.Scope.IsContent() {
    newTail = addr.Face
}
```

The tail is derived by a trusted handler, never taken from the wire. This ticket
makes **Lua the first caller to accept the face as raw caller input** and hand
it to the manager unexamined.

The plan anticipated the question but deferred the answer, rating it `Low`:

> Face on `create_relation` for an **identity**-scoped type — should be refused
> rather than silently ignored... **To confirm against the manager's behaviour
> during implementation**

Confirmed during design review: **the manager does not refuse. It stores the
face.**

## Consequences

**Functional.** An identity-scoped edge written at `@draft` is invisible to
every reader querying at the zero tail — `relations_direction.go:218` filters
`edge.FromFace != tail`. The edge exists and no client returns it. A script's
"does this link already exist" check therefore reads false and re-creates it,
producing unbounded duplicate edges at distinct phantom tails. Each one is
carried into the audit record and the version capture (`version_hook.go:166`).

**Authorization.** The ACL subject carries a coordinate nobody validated. On an
identity-scoped type the principal is authorized against `type@draft` rather
than the bare type. That direction fails *closed* (a spurious deny), but it
means the grant actually consulted is not the one the relation type implies.

## Attack path

`rela.create_relation("POL-1", "created-by", "USR-2", {face = "draft"})` where
`created-by` is `identity`-scoped. `ParseFace` accepts `draft` — the grammar is
fine. The manager stores `FromFace: "draft"`. Reachable by any principal holding
an ordinary `create` grant on the source type at that face; no elevation needed.

## Fix

Add `requireRelationFaceFor(relType, fromType, face)` beside
`requireCreateFaceFor` in `core.go`, and call it from `CreateRelation`,
`UpdateRelation` and `DeleteRelationState` **before the ACL subject is built**,
so an unvalidated coordinate never becomes an authorization coordinate. Rules
mirroring the entity version:

- identity scope (`relDef.Scope.IsIdentity()`) → require the zero face
- content scope on a faced source → require a face the source type declares
- content scope on a faceless source → require zero

**It belongs in the manager, not the Lua binding.** That is what makes it hold
for the elevated path too (see the companion finding on
`admin.create_relation`), and it is the same reason `requireCreateFaceFor` lives
there rather than in each client.

## Scope note

This is arguably a pre-existing manager defect rather than one this ticket
introduces. But it is unreachable until a caller passes an unvalidated face, and
this ticket is that caller. Shipping the Lua face without the manager-side check
would be knowingly opening the path.
