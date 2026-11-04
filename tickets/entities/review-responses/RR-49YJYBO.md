---
id: RR-49YJYBO
type: review-response
title: activeStepIndex does not wait for a step change
finding: After clickNext the old active pill is visible so the wait passes and the non-retrying expect reads the index too early.
severity: minor
resolution: Replaced by FormPage.expectActiveStep which asserts toHaveClass on the step and retries; wizard.spec uses it.
status: addressed
---
