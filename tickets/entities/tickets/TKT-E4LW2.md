---
id: TKT-E4LW2
type: ticket
title: 'Declarative status/enum state machines: transitions on CustomType with ACL-permission guards + predicate preconditions'
kind: enhancement
priority: medium
effort: m
status: done
---

## Scope

Extract the **transition** piece from the approval-and-publication lifecycle
proposal (`.ignored/designs/status-transitions.md` in rela-workflow) as a
standalone enhancement. Makes the set of *legal* enum transitions first-class
and declarative. Benefits every status enum in the system (tickets, bugs,
decisions), not just the register use case.

**Explicitly out of scope** (separate tickets): snapshots / freezing / immutable
versions, approvals/sign-off entities, the "auto-obsolete sibling on establish"
invariant (an automation-action concern), notifications, publication.

> **Design review done (6 findings, RR-FPZJX / RR-UJPW4 / RR-EU8GJ / RR-1SMG4 /
> RR-NGBMT / RR-SGQO3).** The enforcement model below is the *corrected* one —
> the original "slots into existing ACL + validation calls" plan was wrong. See
> the review-responses and the ⚠️ notes inline.

## Tier: this is a served-path primitive

This primitive belongs to rela's evolution from *markdown-on-disk* to *network
service*. On a bare git-backed single-user workflow a gated state machine is
theater — anyone can `vim entities/ticket/foo.md` and set `status: done`; git is
the audit log and human review is the gate. The machine only *bites* where
writes pass through a real chokepoint carrying a `Principal`.

The line is **served vs. direct**, NOT filesystem vs. postgres. Store backend
and access path are orthogonal:

| Access path                      | Principal? | Chokepoint      | Guard bites? |
| -------------------------------- | ---------- | --------------- | ------------ |
| CLI direct write / `vim` on file | no         | none            | no (inert)   |
| **MCP server** (even on fsstore) | yes        | `entitymanager` | **yes**      |
| data-entry HTTP (fs *or* pg)     | yes        | `entitymanager` | **yes**      |

fsstore lands on *both* sides: served over MCP/HTTP → gated; reached directly
via CLI/editor → ungated. An fsstore deployment served over MCP is a
guard-enforcing deployment (the MCP transport already carries a principal and
intersects user caps with agent scope — see the `authorization` concept).

Enforcement splits by half, structurally (not a policy knob):

- **Guard half (403).** Enforced on every *served* path (MCP and HTTP, any
backend). Inert on direct CLI/editor writes — not by tier policy but because
there is no `Principal` to evaluate. This is *why* the feature is a state
machine and not a lint: the guard is meaningless without a principal, which is
meaningless without a served boundary.
- **Legality half (422).** A graph check — needs no principal. Hard-gated on
served paths; available as a `rela validate` lint on the CLI path so direct
edits still get advisory feedback.

Same enforcement boundary and degradation model as the ACL (`NopACL` when no
served context). This feature rides the data-entry/MCP trajectory, not storage.

## Problem

Today the set of *legal* status transitions is emergent, not declared. A
`status` enum knows its allowed *values*; it knows nothing about which
value→value moves are permitted. Legality is inferred from scattered `validate:`
rules. Nothing stops `backlog → done`. There is no inspectable answer to "from
`in-review`, where may I go, and who may take me there?"

## Relationship to TKT-XZEY (important — read before building)

TKT-XZEY ("ACL v0.5: parameterised verbs") was reframed twice on user feedback
(2026-05-21) and reached two conclusions this ticket must respect:

1. **"status is nothing special" — gating must be generic over enum fields, not
privileged to `status`.** ✅ This design agrees: the machine lives on
`CustomType`, so *any* enum property that references the type inherits it.
2. **Per-value fine-grained ACL gating belongs in Lua write-veto hooks, not the
declarative ACL; `acl.yaml` stays entity-type-grain.** This design **does not
contradict** that, because it does not introduce a per-value verb
(`set-prop:status:established`) at all. Instead:
   - **Transition *legality* (from→to) is metamodel structure, not ACL.** It is a
graph on the `CustomType`, checked in the write path (422). No ACL verb.
   - **The `guard` is a coarse *capability noun* (`establish`, `approve`), not a
per-value rule.** It resolves against the existing declarative
`RoleDef.Permissions` machinery — entity-type-grain-ish, exactly the declarative
grain TKT-XZEY wanted to preserve. No verb-cardinality explosion.

So this ticket sidesteps the per-value-verb problem TKT-XZEY rejected by making
legality *structural* and the guard a *capability*. It effectively supersedes
TKT-XZEY's original `transition:<state>` verb sketch. TKT-XZEY's residual scope
(`relation:<type>:add/remove` wire verbs) is independent and unaffected.

Note: TKT-XZEY flagged that `WriteRequest` has no field for a parameterised
verb. The corrected enforcement below avoids needing one — the transition check
is a *separate step* in the manager, not a new `WriteRequest` shape.

## Design

State machine = **edges on the reusable `CustomType`** (not on the property; not
a `-->` string DSL — YAML, no parser to own). A `CustomType` with `transitions:`
*is* a state machine; any property of that type inherits values + machine.
Anonymous inline property enums stay value-only (forcing function: want a
lifecycle → name the type).

```yaml
types:
  snapshot-status:
    values: [in-review, approved, established, obsolete]
    initial: in-review            # only legal entry state (create path)
    transitions:
      - from: in-review
        to: approved
        guard: approve            # ACL permission -> 403 if not held
      - from: approved
        to: established
        guard: establish
        when: "no sibling established"   # precondition -> 422 if false
      - from: established
        to: obsolete
        guard: establish
```

### guard / when split — two engines, two failure kinds

| Field   | Question                                  | Engine                   | Failure |
| ------- | ----------------------------------------- | ------------------------ | ------- |
| `guard` | Do you hold the *right* to do this?       | ACL (subject-aware path) | 403     |
| `when`  | Is the transition *data-valid right now*? | `internal/predicate`     | 422     |

- **`guard` names an ACL permission** resolved against `RoleDef.Permissions`.
Metamodel names the permission; `acl.yaml` decides who holds it → machine stays
free of role names → reusable across deployments.
- **`when` reuses `internal/predicate`** — but the engine is graph-blind. Graph
predicates ("no sibling established") need host funcs
(`count_relations`/`has_relation`) that today live only in the affordances
`BindingContext` and must be extracted/re-provisioned at the write point
(RR-EU8GJ — the `when:` half is NOT free).
- Do NOT collapse authz into `when: "principal.role=='x'"` — hardcodes policy
into schema, loses reuse, opaque denials.

### Subject-scoping — via topology, not a scope field

Guard is a **global capability noun**; no owner/any distinction on the guard.
Scoping comes from *how the guarding role is conferred*: static assignment = any
subject; **role-relation on the ownership edge** (`ticket --assigned-to-->
person`) = own-subject-only. Same edge, same guard name — scope is an `acl.yaml`
decision.

**⚠️ Corrected (RR-UJPW4):** this only works if guard resolution uses the
**subject-aware** ACL path. `holdsPermission` (resolver.go:229) is
`Globals()`-only and does NOT see relation-conferred local roles — using it
silently breaks the own-subject case. Resolve via `computeForEntity` /
`authorizeEntityWrite` (authz_write.go:34-44), which requires the subject ID
(ties to enforcement ordering below).

**Preconditions (hard):**
1. Ownership/assignment must be a first-class graph relation the ACL can walk, or
own-subject scope degrades to global.
2. Any relation used to confer a governance-scoping role **must** carry
`requires_permission` (delegate-X gate), else a principal who can write the
ownership edge self-confers the guarding role (RR-7O6Q self-promotion).

## Enforcement (corrected — RR-FPZJX)

The original plan ("reuse the existing ACL + validation calls") does **not**
work: in `Manager.UpdateEntity` the ACL runs first (manager.go:489) and
`metamodel.ValidateEntity` next (499) — both on **new state only** — and
`oldEntity` is not loaded until line 505, *after* both. So neither existing gate
can see `from`, hence neither can know which edge (and which guard permission)
is in play. Also: `internal/validator` is the wrong component (offline batch
reader, never gates a live write); the write-time 422 is
`metamodel.ValidateEntity`.

**Fix:** a dedicated **transition-check step** in `Manager.UpdateEntity`,
positioned *after* `oldEntity` is loaded and *before* the store write (option C;
option A = re-order the whole path so ACL/validation get old-state is the
heavier alternative). Steps:

1. Resolve the changed property's `CustomType`; if no `transitions`, skip.
2. `from = oldEntity[prop]`, `to = newEntity[prop]`; if unchanged, skip.
3. **Legality:** `(from,to)` must be a declared edge → else 422.
4. **Guard:** resolve `edge.Guard` via the subject-aware path
(`computeForEntity`, NOT `holdsPermission`) for `(principal, subjectID)` → else
403 with `RuleKind: "transition-guard"`, `RuleID: <permission>`.
5. **Precondition:** evaluate `edge.When` against a graph-backed predicate binding
→ else 422.

The top-of-`UpdateEntity` ACL call still runs (gates the update *verb* on type);
the transition guard is an additional gate after old-load. Guard denials audit
as `denied-write` like any ACL deny.

### Create path (RR-1SMG4)

`CreateEntity` has no old state (no `GetEntity` pre-read, manager.go:334-411),
so no `from`. Entry semantics, explicit:
- created value defaults to `initial` (or the enum `default`);
- any **non-`initial`** value on create → 422 ("illegal entry state");
- (deferred alternative: model explicit `[*] -> value` entry edges with guards).

## Priced against the code

- **`RuleKind: "transition-guard"`** — additive, no exhaustive switch exists
(acl.go:101; producers set string literals). Trivial.
- **Guard resolver** — revised UP from "~15 lines / near-copy of delegate gate":
the delegate gate uses `Globals()`; a transition guard needs the subject-aware
`computeForEntity` local-role probes and the subject ID. Still small, but the
entity-write authz path, not the globals-only delegate gate.
- **`when:`** — engine is reusable (`internal/predicate`, standalone,
doc.go:80-83) but graph host-funcs are affordances-specific plumbing; extract to
a shared package or build a write-path binding context. Not free.
- **CustomType is the right home** — inline `type:enum` and named types are
separate, never-merged paths, so `transitions:` on CustomType cleanly means
named-type-only. **Caveat (RR-NGBMT):** inline enums silently get no machine and
no warning; add a migration note + optional lint.
- **Old-state availability** — the automation engine already computes `from`/`to`
from `OldEntity`/`Entity` (automation/engine.go:210-214), proving the pattern;
the transition check needs the same pairing, hosted in the manager after
old-load.

## Load-time validation

- Every `to:`/`from:` references a declared value (no dangling states).
- `initial` (if set) is a declared value.
- (Nice-to-have) reachability / unreachable-state warning.

## Acceptance criteria

1. A `CustomType` can declare `transitions` (from/to/guard?/when?) and optional
`initial`; a property of that type inherits the machine.
2. On update, changing the value is rejected 422 unless `(from,to)` is a declared
edge (or the type has no transitions → unconstrained, as today).
3. On a served path, an edge `guard` is enforced as an ACL permission via the
**subject-aware** resolver; missing → 403 with `RuleKind: "transition-guard"`,
`RuleID: <permission>`. Subject-scoped (own-subject) guards work via
relation-conferred roles. On the direct CLI path the guard is inert (no
principal) and legality surfaces as a `rela validate` lint.
4. An edge `when` is enforced as an `internal/predicate` precondition (with
graph-backed host funcs available); false → 422.
5. **Create path:** value defaults to `initial`; a non-`initial` value on create
is rejected 422.
6. Load-time validation rejects dangling `from`/`to`/`initial`.
7. Inline `type:enum` properties (no named type) are documented as unguarded; a
lint/migration path is provided or explicitly deferred.

## References

- Working design doc: `.ignored/designs/status-transitions.md` (rela-workflow)
- Source proposal: `openvwr-screenshots/tickets/approval-and-publication-lifecycle.md`
- Supersedes the `transition:<state>` sketch in TKT-XZEY
- Evaluator convergence: RES-6PK0S3
- Design-review findings: RR-FPZJX, RR-UJPW4, RR-EU8GJ, RR-1SMG4, RR-NGBMT, RR-SGQO3
