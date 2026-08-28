---
id: RR-WRUTYN
type: review-response
title: '''Non-empty stored value'' is undefined for in-flight edits, making prompting nondeterministic; reuse isClearedForType'
finding: |-
    Plan step 6 prompts only when a hidden field holds a 'non-empty STORED value', but never defines 'stored'. lastSeenServer (useAutoSave.ts:167) is the only oracle and is mutated by every mergeServerResponse. So a value typed 200ms ago that has already committed counts as stored, while one still sitting in `pending` does not — whether the user is prompted depends on debounce timing. Needs an explicit definition (proposal: stored = lastSeenServer, evaluated at gate time, with pending treated as stored so a just-typed value is not silently discarded).

    Separately, the emptiness predicate must reuse isClearedForType (DynamicForm.vue:1009, from @/utils/formValue), which deliberately treats boolean false as a REAL value per TKT-E6094. Hand-rolling the undefined/''/null triple used by the current watcher (:222-224) would classify false as non-empty and prompt spuriously on every checkbox toggle.
severity: minor
resolution: |-
    Implemented. Emptiness uses the existing isClearedForType (utils/formValue), so boolean false is a real value and does not prompt spuriously on checkbox toggles; empty string, null, undefined and empty arrays all count as nothing-at-stake.

    'Stored' is defined as the CURRENT form value at gate time (formData), not autoSave's lastSeenServer. This sidesteps the nondeterminism the finding describes: whether a value has finished round-tripping through a debounce no longer changes whether the user is prompted. A value the user typed 200ms ago is treated the same as one loaded from the server — both are real values worth protecting.

    Unit tests cover each case: 'treats boolean false as a real value, not an empty one', 'ignores fields that hold nothing worth losing', 'treats an empty array as cleared'.
status: addressed
---
