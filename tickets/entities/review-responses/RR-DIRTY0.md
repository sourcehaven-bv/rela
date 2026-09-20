---
id: RR-DIRTY0
type: review-response
title: The duplicate dialog is dirty from the moment it opens, so its discard guard distinguishes nothing
finding: '"applyEmbeddedPrefill runs applyTemplate (which re-baselines originalData) and then routePrefilledCardRelations deletes keys from relations.value without re-baselining. Separately, pendingCardChanges.size > 0 already forces dirty = true. Net effect: the dialog is always dirty in the form phase, so closing a freshly-opened duplicate having touched nothing prompts ''Discard copy?''. The requestClose guard is correct in isolation; it is fed a dirty flag that is structurally always true, which defeats the purpose of having a guard."'
severity: significant
resolution: originalData is re-baselined after ALL prefill mutation, not mid-way. Pinned by 'is not dirty before the user edits anything'.
status: addressed
---

## Suggested resolution

Baseline originalData once, after all prefill mutation completes, and decide deliberately whether a prefilled-but-untouched card set should count as dirty.
