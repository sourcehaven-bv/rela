---
id: RR-VLQ5WM
type: review-response
title: 'Three more PR-C criticals: phantom edge tail, missing authority-minting mitigation, unevaluated guard.when'
finding: |-
    Three further criticals from the PR-C review, all verified:

    1. PHANTOM EDGE TAIL. copy_apply.go used `plan.to.Face` — the DECLARED name — while every other site routed through StoredFace. A face marked `default: true` IS the zero coordinate, so a revise-into-draft wrote the entity at "" but its edges at "draft": orphaned from any face. Two failures from one line, because `replace` then queried the correct tail, found nothing, and left the stale edge:

      EDGE ptr=""          cites SPEC-12   <- stale, replace missed it
      EDGE ptr="draft"     cites SPEC-9    <- phantom tail

    My own StoredFace godoc names this exact failure ('two rows claiming to be the same face') and then the adjacent file committed it.

    2. §9.2's FIRST authority-minting mitigation was MISSING ENTIRELY. The design names two: exclude identity-scoped/role-conferring relation types, AND authorize cross-entity edges per-edge. Only the second existed. validateCopyRelations never read relDef.Scope despite RelationScope.IsIdentity() being available, and nothing in aclaudit mentioned copies — so it was not relocated per Q6, it was dropped. Probe: a definition naming an identity-scoped `owned-by` loaded clean and DUPLICATED the edge onto a state tail, breaking §2.2's invariant that identity edges attach to the bare id.

    3. `guard.when` was declared, godoc'd as 'False refuses the copy (422)', and NEVER EVALUATED. grep for Guard.When returns nothing outside the type definition. An operator writes `when: "source.status == 'approved'"`, the schema loads clean, and unapproved drafts publish. A silently-ignored security control is worse than an absent one, because the operator stops looking.

    Also minor: the target-existence probe folded every store error into 'does not exist', so a transient read failure became a silent full overwrite whose audit record said created=true.
severity: critical
resolution: |-
    1. The tail is resolved ONCE on copyPlan (sourceTail/targetTail), beside the other three resolutions, removing the chance of a fourth divergence. Pinned by TestCopy_EdgesLandOnTheStoredTail, which also gives relation copying and `replace` their first coverage; verified to bite — reintroducing the declared name fails it on both the phantom tail AND the stale edge.

    2. validateCopyRelations now refuses any identity-scoped relation at load, with an error explaining that such an edge is shared by every state so copying it duplicates an edge that may confer roles. Role-conferring types live in acl.yaml which the metamodel cannot see, but they are overwhelmingly identity-scoped, so this catches the class.

    3. A non-empty `guard.when` is REFUSED at load as unimplemented. Chosen over a silent accept (the one option that must not ship) and over implementing it inside a PR already carrying four security fixes — refusing costs an operator a startup message and cannot mislead.

    4. The probe now distinguishes ErrNotFound from a real error and fails loud.
status: addressed
---
