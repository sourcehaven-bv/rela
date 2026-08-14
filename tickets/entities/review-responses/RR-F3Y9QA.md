---
id: RR-F3Y9QA
type: review-response
title: Metamodel-driven render gate makes ACL-hidden fields render as empty editable inputs (needs an explicit _fields tombstone)
finding: |-
    Plan step 1 removes the 'f.property in formData' render gate. An ACL-hidden field is absent from BOTH properties (stripHiddenProperties, affordances.go:915) and the sparse _fields map (computeFieldAffordances skips hidden fields entirely), so with the gate gone it renders as an empty, editable input that silently rejects every write.

    NOTE ON SEVERITY: one reviewer escalated this to critical on the theory that a user could type into the box and destroy a redacted value, because 'visible: gates reads, not writes'. That is FALSE and was verified against the code: validateFieldWrite (affordances.go:325) runs on the PATCH path before any other validation (write_handler.go:376) and checks setKeys AND unsetKeys against the same rules — 'hidden/read-only fields cannot be set OR unset', rejecting with RuleFieldHidden. Both reviewers independently confirmed the server fails closed. So there is no data-destruction path; the defect is UX (a phantom editable control that always 403s) plus a missed opportunity.

    FIX (highest leverage in this review): add an explicit tombstone to the _fields wire map — e.g. {visible: false} — so 'absent' stops being the signal for 'hidden'. The frontend then distinguishes redacted from unset directly and renders a read-only redacted affordance (precedent: InaccessibleField.vue). This permanently retires the ambiguous sentinel that why3 and why5 both identify as the systemic root cause; without it, this fix removes one consumer of the ambiguous sentinel and leaves the sentinel in place for the next subsystem to trip over — exactly what BUG-FB0LN8's own prevention field says to avoid.
severity: significant
resolution: |-
    ACCEPTED INTO THIS TICKET (not split out). The tombstone makes the current work simpler and more correct, so it is in scope.

    Change is small: add `Visible *bool` to FieldAffordance (internal/apiwire/v1/responses.go:73), following the same sparse convention as the existing `Writable *bool` — omitted when the permissive default holds, present only to deny. Then change the `continue` at affordances.go:740 ('hidden takes precedence; skip from _fields entirely') to emit {visible: false} instead of skipping.

    Why it SIMPLIFIES rather than enlarges the fix: without it, edit-mode renderability needs a three-way inference (metamodel-declared AND not-in-_fields AND not-in-formData -> guess). With it the rule is flat and reads directly off the wire:
        if (fieldAffordances[p]?.visible === false) -> render redacted, read-only
        if (declaredProperties.has(p))              -> render normally
    No inference from absence anywhere. This also disposes of the phantom-editable-input UX defect in one move: an ACL-hidden field renders as a proper read-only redacted affordance (precedent: InaccessibleField.vue) instead of an empty box that 403s on every keystroke.

    Disclosure check: the tombstone reveals WHICH fields are redacted, where today they are indistinguishable from unset. Explicitly sanctioned — root CLAUDE.md states field-level redaction 'makes no claim to conceal which properties exist', and /api/v1/_schema (handleV1Schema, api_v1.go:1149) was verified to serve every declared property name unfiltered to any authenticated user with no principal check. So this discloses nothing the SPA did not already have. Row-level ACL is untouched.

    This is what retires the ambiguous 'absence means hidden' sentinel that why3 and why5 both name as the systemic root cause, satisfying the bug's own prevention field.
status: addressed
---
