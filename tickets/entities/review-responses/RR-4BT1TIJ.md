---
id: RR-4BT1TIJ
type: review-response
title: 'CodeQL: refusal errors log credential-named fields'
finding: 'CodeQL go/clear-text-logging flagged 9 sinks: the new errors quoted APIKeyEnv and PasswordVar, which it treats as credentials.'
severity: minor
resolution: The ai.yaml and mail.yaml refusal errors name the prefix instead of the configured variable name.
status: addressed
---
