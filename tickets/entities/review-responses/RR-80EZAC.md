---
id: RR-80EZAC
type: review-response
title: 401 not declared; middleware 403 is not problem+json
finding: A gated server can answer 401 on every operation and none declared it; the same-origin refusal is plain JSON.
severity: minor
resolution: 401 with problem+json is added to every operation when AuthHeader is set (TestSecurityScheme). The problemContent comment now says it covers handler errors only.
status: addressed
---
