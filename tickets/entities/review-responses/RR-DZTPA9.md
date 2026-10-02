---
id: RR-DZTPA9
type: review-response
title: Function results bypassed secret redaction
finding: summarizeResults applied no name filter, so (*mail.Config).resolvePassword wrote the SMTP password to the trace; stripBearer recorded a JWT prefix.
severity: significant
resolution: 'Arguments and results of functions whose names suggest a secret (secretFunc: pass, token, bearer, ...) are now redacted. Tests: TestSummarizeResults, TestSummarizeArgs/secret function.'
status: addressed
---
