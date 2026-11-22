---
id: RR-SEC2JOB
type: review-response
title: 'A manager who cannot read the subject got the job error'
finding: 'The job error was shown to every manager, and its text can hold restricted subject values.'
severity: significant
resolution: 'The error is withheld unless the caller manages the run and reads its subject, in the main place and in parallel branches; TestWithholdJobError pins it.'
status: addressed
---
