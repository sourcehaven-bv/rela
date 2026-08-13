---
id: BUG-KIMZRK
type: bug
title: Cascade re-write skips post-automation validation/unique/transition re-checks (create path re-checks, cascade path does not)
description: 'SUBSTANTIALLY OVERSTATED AS ORIGINALLY FILED — see the correction section. cascadeHost.WriteEntity does NOT create an unaudited, unauthorized write path: it only ever re-writes an entity created moments earlier in the same cascade by cascadeHost.CreateEntity, which authorizes, validates, enforces transitions/uniques and audits. What genuinely remains is narrow: property values SET BY AUTOMATION after that create are persisted without re-running validation, unique or transition checks — unlike the top-level create path, which does re-check post-automation.'
priority: low
status: backlog
---

> **CORRECTION (2026-08-12).** As originally filed this bug was substantially
> overstated. I re-read the code on current `develop` and most of the claimed
> impact does not hold. The corrected finding is much narrower and the priority
> is dropped `high` → `low`. Original text is kept below the correction for
> provenance.

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

## Suspected impact — low

Requires a metamodel where a cascade creates an entity **and** an on-create
automation sets a property that is `unique:`, validated, or state-machine
governed. Plausible but not common. Prefer a failing test before any fix.

## Suggested fix

Re-check post-automation values before `WriteEntity`, mirroring
`manager.go:456-476`. That is a small, local change — much smaller than the
"route the whole thing through Manager.UpdateEntity" implied by the original
filing, which would have changed cascade failure semantics and needed its own
design (partial-cascade rollback is wont-fix per RR-KNXFF).

The linked measure `cascade-write-full-pipeline-test` should be narrowed to
match: assert post-automation unique/validation/transition re-checks, and **drop
the audit assertion** — audit is already correct and asserting otherwise would
pin a false expectation.

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
