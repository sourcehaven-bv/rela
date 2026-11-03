---
id: RR-8MAYUNC
type: review-response
title: Client repeats the unknown principal literal
finding: formDefaults.ts compared against a bare 'unknown', and MeResponse.user did not mention the placeholder.
severity: minor
resolution: UNKNOWN_USER constant in utils/userToken.ts names principal.Unknown; MeResponse.user documents it.
status: addressed
---
