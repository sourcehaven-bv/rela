---
id: RR-NB135
type: review-response
title: ApplyEntity (sync path) bypasses the transition enforcer entirely
finding: 'Manager.ApplyEntity (apply.go:78-149) is a full write entry point (authorize+validate+unique+persist+audit) that NEVER calls EnforceUpdate/EnforceCreate. It writes directly via persistApplyEntity -> Store.Create/UpdateEntity (apply.go:157+). ApplyEntity is the live sync ingestion path (dataentry/sync_handlers.go, cli/sync/pull.go), carries a real ToolSync principal, and can mutate status across an illegal edge or bypass a guard. This directly falsifies the design''s unforgettability claim (''no write path can silently skip legality/guard/precondition'', manager.go + statemachine.go docs). Guard bypass is the sharp edge: the guard is subject/principal-specific, so ''origin already validated'' does not authorize the sync principal. Fix: run legality+precondition in ApplyEntity (guard likely inert for replay), OR document sync as a deliberate exception and drop the unforgettability claim, OR reject sync-applies that land illegal machine states.'
severity: critical
resolution: 'ApplyEntity now enforces transitions (apply.go): EnforceUpdate against the probed `stored` state on update-intent, EnforceCreate on create-intent, before persist. Guard denials routed through mapTransitionError (403). The guard adapter stays inert for CLI/no-policy sync, so only policy-backed served sync is gated. Regression tests TestTransition_ApplyEntity_EnforcesLegality/EnforcesGuard. The unforgettability claim is now true across all Manager write paths (Create/Update/Apply).'
status: addressed
---

## Finding

`Manager.ApplyEntity` (`internal/entitymanager/apply.go:78-149`) — the
sync/upsert write path — authorizes, validates, unique-checks, persists
(`persistApplyEntity` → `Store.Create/UpdateEntity`, apply.go:157+), and audits,
but **never** invokes the transition enforcer. There is no `Transitions`
reference in `apply.go`.

It is a live, served path: `dataentry/sync_handlers.go` (HTTP sync) and
`cli/sync/pull.go` (CLI pull). It carries a real `ToolSync` principal.

## Concrete failure

A divergent/older peer, hand-editor, or peer with a transitions-less metamodel
sets `status` directly from `in-review` to `established`. On pull, `ApplyEntity`
authorizes the coarse `update` verb, validates `established` as a legal *value*
(not a legal *transition*), and writes it — audited as a clean `update-entity`.
The `approve→establish` machine is bypassed by routing through sync. The guard
(subject/principal-specific) is skipped: "the origin validated it" validated it
against the *origin's* principal, not the sync principal.

## Why critical

The feature's entire value proposition is "you cannot skip this check." A served
write path skips it, and the `var _ TransitionEnforcer` assertion + "fixed
pipeline" prose gave false confidence that this couldn't happen.

## Resolution options

1. Run `EnforceUpdate`/`EnforceCreate` in `ApplyEntity` (guard likely inert for
replay, but legality + preconditions must hold). **Recommended.**
2. Document sync as a deliberate exception AND drop the unforgettability claim.
3. Reject a sync-apply that would land an illegal machine state (force reconcile).
