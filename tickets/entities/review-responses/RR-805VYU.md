---
id: RR-805VYU
type: review-response
title: Retention storage location unspecified — leaving retained values in formData silently changes every existing consumer
finding: |-
    Plan step 2 says 'retain hidden values client-side' without saying WHERE. If retention means 'leave the key in formData', several existing consumers silently change meaning:

    1. mergeServerResponse (useAutoSave.ts:521-526) sweeps disappeared keys: for each k in lastSeenServer not in entity.properties and not pending/timers, it calls applyServerProperty(k, undefined), which does delete formData.value[k] (DynamicForm.vue:1220-1224). So the NEXT unrelated PATCH response silently deletes the retained value mid-session and the reveal is lossy again.
    2. conditionBindings (:173-177) — see RR-2IJL4Y (cascade semantics).
    3. pruneWizardHidden (:722), visibleWritablePropertiesForCommit (:300), checkDirty (:1131) all read formData and would change behavior. In particular the plan promises 'create path unchanged' (step 7) while mutating the shared state that path reads from; if an implementer then 'tidies' pruneWizardHidden as now-redundant, RR-O4SRG regresses.

    Resolution: retention must live in a SEPARATE ref (e.g. retainedHidden: Ref<Record<string, unknown>>), not in formData, or mergeServerResponse must be taught about it explicitly. This must be decided in the plan, not at implementation time.
severity: critical
resolution: |-
    Resolved by design decision: retention lives in a SEPARATE ref, retainedHidden: Ref<Record<string, unknown>>, never in formData.

    This keeps formData meaning exactly what it means today, so every existing consumer is untouched: mergeServerResponse's disappeared-key sweep (useAutoSave.ts:521-526) cannot delete retained values, and pruneWizardHidden (:722), visibleWritablePropertiesForCommit (:300), checkDirty (:1131) and the create path all keep their current semantics. The 'create path unchanged' promise (plan step 7) now actually holds.

    Side benefit: this also resolves the cascade concern (RR-2IJL4Y) for free — conditionBindings keeps evaluating over formData, which no longer contains hidden values, so cascading behaves exactly as it does today.
status: addressed
---
