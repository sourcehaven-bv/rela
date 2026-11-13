---
id: RR-C4E5YHP
type: review-response
title: Provider refusal test duplicates config test
finding: TestNewProvider_RefusesRemoteCredentialEnv repeats the config test; keep it only as a constructor guard.
severity: nit
resolution: Comment now says it pins that the constructor keeps calling Validate.
status: addressed
---
