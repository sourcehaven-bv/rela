---
id: RR-UTD3R
type: review-response
title: Trivial bypass via api['rawRequest'] bracket access
finding: '`api[''rawRequest''](''GET'', ''/x'')` passes lint clean. Bracket access is something a dev fishing for ''how do I silence this lint'' will find quickly. Reference extraction via `const fn = api.rawRequest; fn(...)` is a similar bypass but harder to catch via AST.'
severity: minor
resolution: 'Added a second selector entry that matches `api[''rawRequest''](...)` via `callee.computed=true` and `callee.property.value=''rawRequest''`. Verified empirically: a canary spec containing `api[''rawRequest''](''GET'', ''features'')` in a test body now fires the rule. Reference-extraction (`const fn = api.rawRequest; fn(...)`) remains unguarded but is documented in the planning checklist as a known limitation — catching it requires inter-procedural analysis that''s out of proportion for an eslint rule, and code review handles deliberate bypasses.'
status: addressed
---
