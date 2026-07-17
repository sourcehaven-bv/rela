---
id: RR-NODYR
type: review-response
title: 'when: predicate env hardcodes ''status'' property; machines on other-named properties mis-evaluate'
finding: 'compile.go:50-60 indexes ANY property whose type is a state machine (phase, lifecycle, etc.), but buildEnv (predicate.go:22-27) and entityRecord (predicate.go:82-88) hardcode the entity record as {id,type,status} and read e.GetString(''status'') literally. So for a machine on a non-status property: (1) a when: referencing its own value fails to compile; (2) a when: referencing entity.status evaluates against the unrelated status property. Fix: parameterize buildEnv/entityRecord by the machine''s own property name, OR restrict machines to status-named properties at compile time and document it. (Confirmed NOT a bug: stringArg(args,1) matches the {rec,str} FuncSig order.)'
severity: significant
resolution: 'Predicate env no longer hardcodes ''status''. buildEnv exposes entity.{id,type,value} where `value` is bound to the machine''s OWN property (entityRecord takes the prop name, threaded from applyEdge via evalWhen). A when: references entity.value for the machine''s value regardless of property name. Test TestEnforceUpdate_When_EntityValueBinding proves entity.value resolves to the written value (both positive and negative).'
status: addressed
---

## Finding

`compile.go:50-60` indexes **any** property whose type is a state machine, so a
machine can live on `phase`, `lifecycle`, `stage`, etc. But the predicate env
hardcodes `status`:
- `buildEnv` (predicate.go:22-27) declares the `entity` record as `{id,type,status}`.
- `entityRecord` (predicate.go:82-88) binds `{id,type,status}`, reading
`e.GetString("status")` literally.

Consequences for a machine on a non-`status` property:
1. A `when:` referencing the machine's own value (`entity.phase`) fails to compile.
2. A `when:` referencing `entity.status` compiles and evaluates against the
unrelated `status` property — a latent correctness landmine if the entity has
both a `status` and a `phase` machine.

## Resolution

Parameterize `buildEnv`/`entityRecord` by the machine's own property name, OR
restrict machines to `status`-named properties at compile time (and document the
restriction). Works today only because everything is named `status`.

(Cross-checked and fine: `stringArg(args, 1)` matches the `{rec, str}` FuncSig
param order — arg[0] record, arg[1] relation-type string.)
