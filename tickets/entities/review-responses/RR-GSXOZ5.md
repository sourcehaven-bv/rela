---
id: RR-GSXOZ5
type: review-response
title: Three ACL doc comments still asserted the pre-tombstone invariant; object values rendered as [object Object] in the prompt
finding: |-
    1. Three doc comments still stated 'hidden fields are omitted from properties AND from _fields', which the tombstone reverses. These are the contract docs the next reader relies on — notably FieldVerdicts.Visible, the type resolver authors implement:
       - internal/apiwire/v1/responses.go (Entity.FieldAffordances)
       - internal/dataentry/affordances.go (FieldVerdicts.Visible)
       - frontend/src/types/entity.ts (_fields)

    2. formatValueForPrompt used String(value), so an object-valued field prompts 'Clear: • Label: [object Object]' — the user cannot tell what they are approving the loss of.

    3. validate_test.go's new clear_when_hidden tests only exercised top-level form.Fields, not step-nested fields — which is the motivating case, since the AC5 fixture hides a whole wizard step.
severity: minor
resolution: |-
    1. All three doc comments corrected to describe the tombstone, the value/name distinction, and the read-only-row exception.

    2. formatValueForPrompt now JSON.stringifies non-null objects.

    3. Added TestValidateConfig_ClearWhenHiddenValidatedInWizardSteps, asserting an invalid value on a step-nested field is rejected AND that the error locates the step ('step[0]'). Verified the code path was already correct — validateForms calls the same validateFormField for form.Steps[].Fields with a ctx prefix — so this pins existing behavior rather than fixing a defect.
status: addressed
---
