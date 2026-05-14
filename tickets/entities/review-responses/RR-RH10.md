---
id: RR-RH10
type: review-response
title: Multi-step state transition inside openTimer relies on as-cast to silence TS
finding: 'useBacktickAutocomplete.ts lines 366-389 mutate `state.phase` up to three times inside a single setTimeout callback: `pending → prefix` (transitionToPrefix), then optionally `prefix → id` (transitionToIdPhase). The control-flow uses `(state.phase as Phase) === ''prefix''` casts on lines 381 and 383 to read the freshly-mutated value — TypeScript flow analysis can''t follow that `transitionToPrefix()` writes `state.phase` because it doesn''t introspect the called function. The cast is a code smell: every cast like this is a place where a future refactor (e.g. a watcher that asynchronously changes phase) becomes invisible to the type checker. Vue 3 reactive mutations are synchronous to dep-tracking but watchers default to post-render flush, so reactive observers see only the final phase value — that''s fine. But: there are no tests covering the fast-typing case where `pending → prefix → id` collapses inside one tick. Fix: extract the post-delay logic into its own pure function that returns the final state, then write it once; or guard via an explicit local-variable phase tracking rather than re-reading `state.phase` after each transition.'
severity: significant
resolution: Extracted `applyTypedToPhase(typed)` as the single source of truth for 'typed text -> phase transition'. Both the open-delay timer and the change handler call it; the as-Phase casts are gone.
status: addressed
---
