---
id: RR-DS7TCYI
type: review-response
title: Conflicting credential flags resolve silently
finding: --token-secret, --token-file and --token-env overrode each other without a message.
severity: nit
resolution: The three flags are a kong xor group; naming two is an error. Tested.
status: addressed
---
