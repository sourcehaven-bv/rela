---
id: RR-CDGL1
type: review-response
title: Plan does not address automations triggered by property deletion
finding: 'The metamodel automation engine fires on property changes (e.g., ''becomes'' triggers, see CLAUDE.md automation section). What happens when a property is deleted that has a ''becomes'' automation? Does removing status= from a ticket trigger the ''becomes:done'' rule because the diff shows a change away from the previous value? The plan handwaves this with ''same automation triggers fire on update''. Verify: read internal/automation/ to confirm the engine handles oldValue->nil transitions correctly, and add a test that deletes a property which has a ''becomes'' automation attached, asserting the automation either fires correctly or is correctly skipped (whichever is the documented contract).'
severity: minor
resolution: 'Verified `internal/automation/engine.go:190-196`: oldValue is read via `entity.GetString(trigger.Property)` which returns "" when the property is missing. So deletion produces (oldValue=<previous>, newValue=""). Automations with `becomes:<specific value>` won''t fire on deletion (newValue is empty string, not the trigger value); automations with `becomes:""` would fire but no such automation exists in our metamodel. Behavior: deletion looks like ''set to empty string'' to the automation engine. Plan updated to document this and add ONE test confirming a `becomes:done` rule does NOT fire when status is deleted.'
status: addressed
---
