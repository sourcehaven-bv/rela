---
id: RR-2IJL4Y
type: review-response
title: 'Cascade semantics unspecified: retained hidden values feed conditionBindings, so dependent branches stop hiding'
finding: |-
    conditionBindings (DynamicForm.vue:173-177) binds form: formData.value, and useFormWizard's evalCond (useFormWizard.ts:109-113) evaluates every visible_when against it. Today the watcher DELETES a hidden key, so downstream conditions see undefined and cascade correctly.

    Under retention they see the retained value. Given a: trigger, b: visible_when 'form.a == x', c: visible_when 'form.b == yes' — with a=x, b=yes, c=2026-09-15 — setting a=other hides b. Today b is deleted so c also hides. Under retention b stays 'yes', so c REMAINS VISIBLE even though its parent branch is gone.

    The plan never picks a semantic: (a) conditions evaluate over VISIBLE values only (matches today, requires retention to live in a separate store, cascades correctly), or (b) over ALL retained values (breaks cascading hides). The plan implies (b) by accident.

    Consequence for the confirm dialog: under (a), the batched dialog must enumerate the TRANSITIVE CLOSURE of newly-hidden fields, not just direct ones. Step 5 says 'if the new value would hide a field' — singular, direct. Computing the closure requires evaluating the condition graph against a HYPOTHETICAL formData where a=other, without committing the change. No such machinery exists: getBindings (useFormWizard.ts:85) is a live getter closed over the real formData, deliberately (see comment :81-84). This needs a new pure API, e.g. wizard.activePropertiesFor(bindings) — a new public surface on useFormWizard, not mechanical work.
severity: minor
resolution: |-
    Downgraded critical -> minor and resolved by two decisions.

    1. MOOT IN PRACTICE. Retention now lives in a separate retainedHidden ref (RR-805VYU), not in formData. conditionBindings (DynamicForm.vue:173-177) keeps evaluating over formData, which no longer contains hidden values — so cascading behaves exactly as it does today. The anomaly the finding describes cannot arise.

    2. NO HYPOTHETICAL-EVALUATION API. The proposed wizard.activePropertiesFor(bindings) surface is DROPPED. It existed solely to compute a transitive closure of newly-hidden fields for the batched dialog, for a three-level chain (c depends on b depends on a) that is a contrived model and rare in practice. The gate handles DIRECT hides only.

    Accepted behavior for deeply-chained conditions: a dependent field may remain visible with a stale value after its parent branch hides. This is strictly better than the current behavior (silent permanent data loss) and clear_when_hidden: yes remains available as the escape hatch for anyone who wants the old cascading clear. Document the limitation; do not engineer around it.

    Rationale (user): 'if people make dumb models they get dumb behavior; let's not overcomplicate things. The system allows for clearing which would resolve the cascade issue. It is also not very likely to set up this cascading stuff in the first place.'
status: addressed
---
