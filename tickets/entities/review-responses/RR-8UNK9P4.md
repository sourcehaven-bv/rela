---
id: RR-8UNK9P4
type: review-response
title: Configure Validate does not check step forms
finding: 'cmd/rela-server/configure.go Validate: a draft naming an undeclared or unsuitable form only surfaces through the trigger checker, whose message wrongly says it could not be checked against acl.yaml. Report it as a normal invalid problem from Validate.'
severity: minor
resolution: CheckFlowsData now takes the data-entry.yaml draft and checks every step form against it. Configure Validate reports a bad step form as a normal flows.yaml problem. Covered by TestCheckFlowsData_StepForms.
status: addressed
---
