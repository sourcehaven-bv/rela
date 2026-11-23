---
id: RR-BZASHNZ
type: review-response
title: Data-entry-only Configure edit refused when flows.yaml exists without acl.yaml
finding: 'internal/configedit/triggers.go:47-50 now sends every data-entry.yaml edit to the trigger checker when flows.yaml exists; CheckTriggerACL errors with ''acl.yaml is absent or empty'' and the save is refused. Configure runs without acl.yaml and many projects have a triggers-only flows.yaml, so renaming app.name is now refused. Fix: judge a data-entry-only edit only when the resolved step-form bounds differ before and after.'
severity: significant
resolution: A data-entry.yaml-only edit that leaves every step form's fields unchanged (and any edit when no flow names a form) returns before the acl.yaml check. Pinned by TestCheckTriggerACL_StepFormEdits.
status: addressed
---
