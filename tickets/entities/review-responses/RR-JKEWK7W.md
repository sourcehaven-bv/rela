---
id: RR-JKEWK7W
type: review-response
title: Validation rules are warnings so the suite cannot prove the gates
finding: 'tickets rules have no enforce: write and no validation: strict; partitionValidationErrors (internal/entitymanager/validation.go:84) turns them into warnings on the CLI and in rela test. The risk in the plan was false and the threat model credited the suite with gate integrity.'
severity: significant
resolution: Risk rewritten. Suite scope stated as trigger effects only. Gate integrity rests on the existing rela validate step of the rela-tickets CI job and on the analyze gates; threat model row updated; plan states that review-response and checklist gates stay analyze-only.
status: addressed
---
