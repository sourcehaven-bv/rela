---
id: BUG-KIMZRK
type: bug
title: Cascade re-write skips post-automation validation/unique/transition re-checks (create path re-checks, cascade path does not)
description: 'CONFIRMED with a reproduction (2026-08-12). An on-create automation that sets a `unique: true` property on a CASCADE-created entity persists a duplicate silently — no error, empty AutomationErrors. The identical automation on a TOP-LEVEL create is correctly rejected, because manager.go:456-476 re-runs checkUniqueProperties against post-automation values while the cascade path (autocascade/runner.go:181-187 -> cascadeHost.WriteEntity) does not. Same metamodel, same automation, opposite outcomes. NOTE: this bug was originally filed with a much broader claim (unaudited/unauthorized write path) that turned out to be wrong — see the correction section. Audit and ACL are fine; the post-automation re-check gap is the whole defect.'
priority: medium
effort: s
why1: An automation set a `unique:` property (and a state-machine value) on a cascade-created entity to an illegal value, and it persisted silently.
why2: cascadeHost.WriteEntity wrote straight to the store with no constraint checks — it re-persists an entity after automation mutates it.
why3: createCore ran its unique/transition/validation checks against the PRE-automation candidate; nothing re-examined the values automation introduced afterwards.
why4: The re-check was added to the top-level create path (manager.go, 'the create path must not be the weaker one') but not to the cascade path, because the two apply automation results in different places and the fix was made at only one of them.
why5: Nothing structurally couples 'automation mutated this entity' to 'therefore re-validate before persisting'. The obligation lived in a comment on one path rather than in a shared helper both paths must pass through, so the second site was easy to miss.
prevention: 'Regression tests pin BOTH paths, deliberately including the top-level control case — the defect was an ASYMMETRY, so a cascade-only test would miss a future change that weakened both. Both re-checks were mutation-tested (removing either makes its test fail). Longer term the real fix is structural: a single apply-automation-results helper that both paths call, which re-validates as part of applying — see the follow-up note on the ticket.'
status: done
---

> **CORRECTION (2026-08-12).** As originally filed this bug was substantially
> overstated — the audit/ACL claims were wrong. But the narrowed residual is
> now **CONFIRMED with a reproduction**: a cascade-created entity silently
> keeps a duplicate `unique:` value. Priority went `high` → `low` on the
> correction, then `low` → `medium` once reproduced. Original text is kept
> below for provenance.

## What is actually true

`cascadeHost.WriteEntity` (`internal/entitymanager/cascadehost.go:79`) does call
`h.deps.Store.UpdateEntity` directly. But the reachability argument in the
original filing was wrong.

`WriteEntity` has exactly **one** caller — `autocascade/runner.go:187`, inside
`runCreatedEntityAutomation` — and it is only ever reached to persist
automation-set properties onto an entity that `cascadeHost.CreateEntity` created
**earlier in the same cascade**. That create goes through `createCore`, which:

- enforces state-machine entry (`Transitions.EnforceCreate`, `core.go`)
- enforces `unique:` constraints (`checkUniqueProperties`)
- partitions validation errors (DEC-HWZHA)
- writes via `Store.CreateEntity` (never an upsert — BUG-ZWTDH9)

and the create **is audited** — `cascadehost.go:62` calls `h.recordCascade(ctx,
audit.OpCreateEntity, ...)`.

So the original claims collapse:

| Original claim | Reality |
|---|---|
| "skips ACL authorization" | The cascade is already running under an authorized trigger write; cascade writes are deliberately host-mediated, not re-authorized per step. Not a hole. |
| "produces NO audit record" | **False.** The create is audited at `cascadehost.go:62`. The absence of a second record on the property-set step is deliberate and documented — a second record would double-count one creation. |
| "an entity can change with no trace" | **False**, per the above. |
| "unvalidated write path" | Partly true, but only for the post-automation delta — see below. |

The `WriteEntity` godoc (`cascadehost.go:69-78`) already explains both the
update-not-upsert choice and the no-second-audit choice. I did not read it
carefully enough before filing.

## The genuine residual

Narrow and real: **property values set by automation after the create are
persisted without re-running validation, unique or transition checks.**

`runner.go:181-187` applies `newAutoResult.PropertiesSet` onto `created` and
immediately calls `WriteEntity`. `createCore` ran its checks against the
*pre-automation* candidate, so a value automation introduces afterwards is never
re-examined.

The asymmetry with the top-level create path is the actual defect. At
`manager.go:456-476` the create path explicitly re-runs `checkUniqueProperties`
against post-automation values, with the reasoning that the create path *"must
not be the weaker one."* The cascade path has no equivalent.

**Consequences** (all require an automation with `set` actions firing on a
cascade-created entity):

- a `unique:` constraint can be violated by an automation-set value
- an automation can set a property value that validation would reject
- a state-machine entry value set by automation is not re-enforced

No audit gap. No ACL gap.

## Reproduction — CONFIRMED

Built as a throwaway probe against current `develop` (memstore, real engine +
runner, no stubs). Metamodel: `persoon` with `email: {unique: true}`.

Setup: seed `PERS-EXISTING` holding `taken@example.com`. Two automations — (1)
creating a `ticket` cascades into creating a `persoon`; (2) on-create for
`persoon` sets `email` to the value already taken.

**Cascade path — constraint bypassed:**

```
CreateEntity err=<nil>
automation errors: []
entities created: 1
holder: PERS-3DLE       email=taken@example.com
holder: PERS-EXISTING   email=taken@example.com
=== rows holding the unique value: 2 ===
```

Two rows share a `unique: true` value, and the write reported **no error at
all** — `AutomationErrors` empty, `CreateEntity` returned nil. Silent.

**Control — identical automation on a TOP-LEVEL create:**

```
top-level CreateEntity err=validation errors: ...
=== top-level path: rows holding the unique value: 1 ===
```

Correctly rejected. Same metamodel, same automation, opposite outcomes — which
is precisely the `manager.go:456-476` asymmetry, demonstrated rather than
inferred.

**Impact — medium.** Silent data corruption of a natural key, no error surfaced
to the caller, and the resulting duplicate persists. Reachability needs a
cascade-created entity plus an on-create `set` on a constrained property, which
is not exotic: it is exactly the shape of the checklist automations in this
project's own `metamodel.yaml`.

Not raised above medium because it requires an operator-authored automation to
set a *constrained* property, and a duplicate is recoverable once noticed
(`analyze_unique` surfaces it).

## Suggested fix

Re-check post-automation values before `WriteEntity`, mirroring
`manager.go:456-476`. That is a small, local change — much smaller than the
"route the whole thing through Manager.UpdateEntity" implied by the original
filing, which would have changed cascade failure semantics and needed its own
design (partial-cascade rollback is wont-fix per RR-KNXFF).

The linked measure `cascade-write-full-pipeline-test` is already narrowed to
match: it asserts post-automation unique/validation/transition re-checks and
deliberately does **not** assert audit (audit is correct; asserting otherwise
would pin a false expectation).

Start from the reproduction above — it is the regression test, minus the
`t.Logf` scaffolding. Keep the top-level control case alongside it: the bug is
an *asymmetry*, so a test that only exercises the cascade path would not catch a
future regression that weakened both.

---

## Original filing (superseded — retained for provenance)

The text below overstates the problem; read the correction above first.

`cascadeHost.WriteEntity` writes straight to the raw store, so this write skips
ACL authorization, metamodel validation, `unique:` enforcement, state-machine
transition enforcement, and the audit log. Contradicts root `CLAUDE.md`: *"All
writes go through `entitymanager.Manager`; do not write to `store.Store`
directly from a write path."* Severity provisionally **high** because it is an
unaudited, unvalidated write path — *needs confirmation during analysis before
the priority is trusted.*
