---
id: RR-IVS1FQ
type: review-response
title: 'Code: on.updated with an inline script recurses without bound'
finding: An inline script writing its own entity re-fired on.updated in a nested write that no depth limit covers; DeepEqual also saw int vs float64 as a change.
severity: critical
resolution: Load refuses a non-background script under on.updated; entityChanged compares canonical.HashEntity. Tests in automationjob_test.go and TestUpdatedTrigger.
status: addressed
---
