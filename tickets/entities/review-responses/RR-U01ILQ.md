---
id: RR-U01ILQ
type: review-response
title: 'updateCore extraction boundary unspecified: attribution and authorize must stay in the entry points'
finding: 'The plan says ''extract updateCore'' without specifying the split point, which is the one detail that decides whether the extraction is safe. withStoreAttribution(ctx) (manager.go:556) and authorizeAndAudit (manager.go:559) must stay in the ENTRY POINTS, not move into updateCore. If they move: PatchEntity would double-authorize (it must authorize early to get the type per RR-0V0TVB), which is harmless functionally but emits TWO denied-write audit rows on a deny, corrupting the forensic count — and updateCore would silently become a second public write surface with its own ACL check, which is exactly the kind of parallel write path the root CLAUDE.md forbids. The correct boundary mirrors createCore, which is a free function over Deps (core.go:88-90) containing no ACL and no attribution — both live in Manager.CreateEntity. Extractable body is manager.go:566 (preErrs := ...) through :653.'
severity: minor
resolution: |-
    ACCEPTED. Extraction boundary now specified explicitly in the plan, mirroring createCore (core.go:88-90), which contains no ACL and no attribution.

    STAYS in the entry points (UpdateEntity / PatchEntity): withStoreAttribution (manager.go:556), nil/id guards, authorizeAndAudit (:559), the raw GetEntity, the IsLocked guard (RR-0QWLRC), and the FieldGate call (RR-32XA5V).

    MOVES into updateCore: validate(pre) + partitionValidationErrors (:566-571), automation + re-validate (:580-604), transitions (:606-616), unique (:618-623), store write (:625-630), audit (:632-635), cascade (:641-655).

    Rationale for keeping authorize out: PatchEntity must authorize early (it needs the type from the read, RR-0V0TVB), so an authorize inside updateCore would double-authorize — harmless functionally but emitting TWO denied-write audit rows on a deny, corrupting the forensic count — and would make updateCore a de-facto second public write surface with its own ACL check.

    Sequencing: the extraction lands as a standalone behaviour-preserving commit with the full suite green BEFORE PatchEntity is added. Load-bearing ordering comments (audit-before-cascade at :632, re-validate-after-automation, exclude-self on unique) are carried verbatim, not paraphrased.
status: addressed
---

## Precedent

`internal/entitymanager/core.go:84-90` — `createCore`'s godoc explains the
shape:

> Free function over [Deps] (not a method on Manager) so cascadeHost can call it
> directly without constructing a half-initialized Manager view.

It contains no ACL check and no attribution; `Manager.CreateEntity` owns both.
`updateCore` must match.

## Boundary

| Stays in entry point (`UpdateEntity` / `PatchEntity`) | Moves to `updateCore` |
|---|---|
| `withStoreAttribution(ctx)` (`manager.go:556`) | validate(pre) + `partitionValidationErrors` (`:566-571`) |
| nil/id guards | automation + re-validate (`:580-604`) |
| `authorizeAndAudit` (`:559`) | transitions (`:606-616`) |
| the raw `GetEntity` (now passed in, RR-GM92KZ) | unique (`:618-623`) |
| `IsLocked` guard (RR-0QWLRC) | store write (`:625-630`) |
| `FieldGate` (RR-32XA5V) | audit (`:632-635`) |
| | cascade (`:641-655`) |

## Sequencing

Do the extraction as a **standalone, behaviour-preserving commit** with the full
suite green before `PatchEntity` is added. The ordering comments in `manager.go`
(audit-before-cascade at `:632`, re-validate-after-automation, exclude-self on
unique) are load-bearing — carry them verbatim rather than paraphrasing.
